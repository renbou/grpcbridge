package webbridge

import (
	"context"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/rpcutil"
	"github.com/renbou/grpcbridge/routing"
	"github.com/renbou/grpcbridge/transcoding"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// TranscodedHTTPBridgeOpts define all the optional settings which can be set for [TranscodedHTTPBridge].
type TranscodedHTTPBridgeOpts struct {
	// Logs are discarded by default.
	Logger bridgelog.Logger

	// If not set, the default [transcoding.StandardTranscoder] is created with default options.
	Transcoder transcoding.HTTPTranscoder

	// If not set, the default [grpcadapter.ProxyForwarder] is created with default options.
	Forwarder grpcadapter.Forwarder
}

func (o TranscodedHTTPBridgeOpts) withDefaults() TranscodedHTTPBridgeOpts {
	if o.Logger == nil {
		o.Logger = bridgelog.Discard()
	}

	if o.Transcoder == nil {
		o.Transcoder = transcoding.NewStandardTranscoder(transcoding.StandardTranscoderOpts{})
	}

	if o.Forwarder == nil {
		o.Forwarder = grpcadapter.NewProxyForwarder(grpcadapter.ProxyForwarderOpts{})
	}

	return o
}

// TranscodedHTTPBridge is a gRPC bridge which performs transcoding between HTTP and gRPC requests/responses
// using the specified transcoder, which isn't an optional argument by default since a single transcoder should be used
// for the various available bridges for compatibility between them.
//
// Currently, only unary RPCs are supported, and the streaming functionality of the transcoder is not used.
// TranscodedHTTPBridge performs transcoding not only for the request and response messages,
// but also for the errors and statuses returned by the router, transcoder, and gRPC connection to which the RPC is bridged.
// More specifically, gRPC status codes will be used to set an HTTP code according to the [Closest HTTP Mapping],
// with the possibility to override the code by returning an error implementing interface{ HTTPStatus() int }.
//
// Unary RPCs follow the Proto3 "all fields are optional" convention, treating completely-empty request messages as valid.
// This matches with gRPC-Gateway's behaviour.
//
// [Closest HTTP Mapping]: https://chromium.googlesource.com/external/github.com/grpc/grpc/+/refs/tags/v1.21.4-pre1/doc/statuscodes.md
type TranscodedHTTPBridge struct {
	logger     bridgelog.Logger
	router     routing.HTTPRouter
	transcoder transcoding.HTTPTranscoder
	forwarder  grpcadapter.Forwarder
}

// NewTranscodedHTTPBridge initializes a new [TranscodedHTTPBridge] using the specified router and options.
// The router isn't optional, because no routers in grpcbridge can be constructed without some form of required args.
func NewTranscodedHTTPBridge(router routing.HTTPRouter, opts TranscodedHTTPBridgeOpts) *TranscodedHTTPBridge {
	opts = opts.withDefaults()

	return &TranscodedHTTPBridge{
		logger:     opts.Logger.WithComponent("grpcbridge.web"),
		router:     router,
		transcoder: opts.Transcoder,
		forwarder:  opts.Forwarder,
	}
}

// ServeHTTP implements [net/http.Handler] so that the bridge is used as a normal HTTP handler.
func (b *TranscodedHTTPBridge) ServeHTTP(unwrappedRW http.ResponseWriter, r *http.Request) {
	req := routeTranscodedRequest(unwrappedRW, r, b.router, b.transcoder)
	if req == nil {
		return
	}

	if req.route.Method.ClientStreaming {
		writeError(req.w, req.r, req.resptc, status.Errorf(codes.Unimplemented, "client streaming through HTTP not supported"))
		return
	}

	incoming := &httpStream{w: req.w, r: r, reqtc: req.reqtc, resptc: req.resptc}

	if req.route.Method.ServerStreaming {
		streamtc, ok := req.resptc.(transcoding.ResponseStreamTranscoder)
		if !ok {
			writeError(req.w, req.r, req.resptc, status.Error(codes.InvalidArgument, "encoding does not support streaming"))
			return
		}

		flusher, ok := req.w.ResponseWriter.(http.Flusher)
		if !ok {
			writeError(req.w, req.r, req.resptc, status.Error(codes.Internal, "server does not support streaming"))
			return
		}

		incoming.respstream = streamtc.Stream(incoming.w)
		incoming.flusher = flusher
	}

	logger := b.logger.With(
		"target", req.route.Target.Name,
		"grpc.method", req.route.Method.RPCName,
		"http.method", r.Method,
		"http.path", r.URL.Path,
		"http.params", req.route.PathParams,
	)
	logger.Debug("began handling HTTP request")
	defer logger.Debug("ended handling HTTP request")

	incoming.logger = logger
	incoming.readCh = make(chan struct{})

	ctx := metadata.NewIncomingContext(r.Context(), headersToMD(r.Header))

	err := b.forwarder.Forward(ctx, grpcadapter.ForwardParams{
		Target:   req.route.Target,
		Service:  req.route.Service,
		Method:   req.route.Method,
		Incoming: incoming,
		Outgoing: req.conn,
	})
	if err != nil {
		writeError(req.w, req.r, req.resptc, err)
	}
}

type transcodedRequest struct {
	w      *responseWrapper
	r      *http.Request
	conn   grpcadapter.ClientConn
	route  routing.HTTPRoute
	reqtc  transcoding.HTTPRequestTranscoder
	resptc transcoding.HTTPResponseTranscoder
}

func routeTranscodedRequest(unwrappedRW http.ResponseWriter, r *http.Request, router routing.HTTPRouter, transcoder transcoding.HTTPTranscoder) *transcodedRequest {
	var err error

	req := &transcodedRequest{w: &responseWrapper{ResponseWriter: unwrappedRW}, r: r}

	req.conn, req.route, err = router.RouteHTTP(req.r)
	if err != nil {
		writeError(req.w, req.r, nil, err)
		return nil
	}

	req.reqtc, req.resptc, err = transcoder.Bind(transcoding.HTTPRequest{
		Target:     req.route.Target,
		Service:    req.route.Service,
		Method:     req.route.Method,
		Binding:    req.route.Binding,
		RawRequest: req.r,
		PathParams: req.route.PathParams,
	})
	if err != nil {
		writeError(req.w, req.r, nil, err)
		return nil
	}

	return req
}

type httpStream struct {
	logger bridgelog.Logger

	w     *responseWrapper
	r     *http.Request
	reqtc transcoding.HTTPRequestTranscoder

	resptc     transcoding.HTTPResponseTranscoder
	respstream transcoding.TranscodedStream
	flusher    http.Flusher

	sendActive atomic.Bool
	recvActive atomic.Bool

	read   bool // not synchronized because recv() cannot be called concurrently
	readCh chan struct{}
	sent   bool // not synchronized because send() cannot be called concurrently
}

func (s *httpStream) Send(ctx context.Context, msg proto.Message) error {
	s.setSendActive()
	defer s.sendActive.Store(false)

	return s.withCtx(ctx, func() error { return s.send(msg) })
}

func (s *httpStream) setSendActive() {
	if !s.sendActive.CompareAndSwap(false, true) {
		panic("grpcbridge: Send()/SetHeader()/SetTrailer() called concurrently on unaryHTTPStream")
	}
}

func (s *httpStream) send(msg proto.Message) error {
	if s.sent && s.respstream == nil {
		return status.Error(codes.Internal, "grpcbridge: tried sending second response on unary stream")
	}

	// Wait for request to be received, because after this we aren't guaranteed to be able to read the request body,
	// for example when using HTTP/1.1. See http.ResponseWriter.Write comment for more info.
	<-s.readCh

	if !s.sent {
		ct, _ := s.resptc.ContentType(msg) // don't care about whether the response is in binary/utf8
		s.w.Header()[contentTypeHeader] = []string{ct}
	}

	// Set here, because Write can perform a partial write and still return an error
	s.sent = true

	if s.respstream != nil {
		// Use stream response instead.
		if err := s.respstream.Transcode(msg); err != nil {
			return responseTranscodingError(err)
		}

		s.flusher.Flush()
		return nil
	}

	b, err := s.resptc.Transcode(msg)
	if err != nil {
		return responseTranscodingError(err)
	}

	if _, err = s.w.Write(b); err != nil {
		return status.Errorf(codes.Unavailable, "failed to write response body: %s", err)
	}

	return nil
}

func (s *httpStream) Recv(ctx context.Context, msg proto.Message) error {
	if !s.recvActive.CompareAndSwap(false, true) {
		panic("grpcbridge: Recv() called concurrently on unaryHTTPStream")
	}
	defer s.recvActive.Store(false)

	return s.withCtx(ctx, func() error { return s.recv(msg) })
}

func (s *httpStream) recv(msg proto.Message) error {
	if s.read {
		return io.EOF
	}

	b, err := io.ReadAll(s.r.Body)
	if err != nil {
		return status.Errorf(codes.Unavailable, "failed to read request body: %s", err)
	}

	s.read = true
	close(s.readCh)

	// Treat completely empty bodies as valid ones.
	if len(b) == 0 {
		return nil
	}

	return requestTranscodingError(s.reqtc.Transcode(b, msg))
}

func (s *httpStream) SetHeader(md metadata.MD) {
	s.setSendActive()
	defer s.sendActive.Store(false)

	if s.sent {
		s.logger.Warn("SetHeader() called on already-sent unary stream")
		return
	}

	s.appendHeaders(md)
}

func (s *httpStream) SetTrailer(md metadata.MD) {
	s.setSendActive()
	defer s.sendActive.Store(false)

	if s.sent {
		// gRPC services usually don't return a "Trailer" header containing a list of all the trailers,
		// so we set these headers using the TrailerPrefix functionality, and they will be sent after ServeHTTP returns.
		for k, v := range md {
			k = http.CanonicalHeaderKey(http.TrailerPrefix + k)
			s.w.Header()[k] = append(s.w.Header()[k], v...)
		}
		return
	}

	// This is the path taken by unary calls if the forwarder supports it,
	// for which we can actually send the trailers as headers.
	s.appendHeaders(md)
}

func (s *httpStream) appendHeaders(md metadata.MD) {
	for k, v := range md {
		k = http.CanonicalHeaderKey(k)
		s.w.Header()[k] = append(s.w.Header()[k], v...)
	}
}

func (s *httpStream) withCtx(ctx context.Context, f func() error) error {
	errChan := make(chan error, 1)
	go func() {
		errChan <- f()
	}()

	select {
	case <-ctx.Done():
		return rpcutil.ContextError(ctx.Err())
	case err := <-errChan:
		return err
	}
}
