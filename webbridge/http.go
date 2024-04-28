package webbridge

import (
	"context"
	"io"
	"net/http"

	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/routing"
	"github.com/renbou/grpcbridge/transcoding"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// TranscodedHTTPBridgeOpts define all the optional settings which can be set for [TranscodedHTTPBridge].
type TranscodedHTTPBridgeOpts struct {
	// Logs are discarded by default.
	Logger bridgelog.Logger
}

func (o TranscodedHTTPBridgeOpts) withDefaults() TranscodedHTTPBridgeOpts {
	if o.Logger == nil {
		o.Logger = bridgelog.Discard()
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
}

// NewTranscodedHTTPBridge initializes a new [TranscodedHTTPBridge] using the specified router and transcoder.
func NewTranscodedHTTPBridge(router routing.HTTPRouter, transcoder transcoding.HTTPTranscoder, opts TranscodedHTTPBridgeOpts) *TranscodedHTTPBridge {
	opts = opts.withDefaults()

	return &TranscodedHTTPBridge{
		logger:     opts.Logger.WithComponent("grpcbridge.web"),
		router:     router,
		transcoder: transcoder,
	}
}

// ServeHTTP implements [net/http.Handler] so that the bridge is used as a normal HTTP handler.
func (b *TranscodedHTTPBridge) ServeHTTP(unwrappedRW http.ResponseWriter, r *http.Request) {
	req := routeTranscodedRequest(unwrappedRW, r, b.router, b.transcoder)
	if req == nil {
		return
	}

	// Currently, no extra checks are performed here, but transcoder streaming validity should be checked.
	if ok := req.connect(); !ok {
		return
	}

	// Always clean up the outgoing stream by explicitly closing it.
	defer req.outgoing.Close()

	logger := b.logger.With(
		"target", req.route.Target.Name,
		"grpc.method", req.route.Method.RPCName,
		"http.method", r.Method,
		"http.path", r.URL.Path,
		"http.params", req.route.PathParams,
	)
	logger.Debug("began handling HTTP request")
	defer logger.Debug("ended handling HTTP request")

	err := grpcadapter.ForwardServerToClient(r.Context(), grpcadapter.ForwardS2C{
		Incoming: &unaryHTTPStream{w: req.w, r: r, reqtc: req.reqtc, resptc: req.resptc, readCh: make(chan struct{})},
		Outgoing: req.outgoing,
		Method:   req.route.Method,
	})
	if err != nil {
		writeError(req.w, r, req.resptc, err)
	}
}

type transcodedRequest struct {
	w        *responseWrapper
	r        *http.Request
	conn     grpcadapter.ClientConn
	route    routing.HTTPRoute
	outgoing grpcadapter.ClientStream
	reqtc    transcoding.HTTPRequestTranscoder
	resptc   transcoding.HTTPResponseTranscoder
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

func (req *transcodedRequest) connect() bool {
	var err error

	// At this point all responses including errors should be transcoded to get properly parsed by a client.
	req.outgoing, err = req.conn.Stream(req.r.Context(), req.route.Method.RPCName)
	if err != nil {
		writeError(req.w, req.r, req.resptc, err)
		return false
	}

	return true
}

type unaryHTTPStream struct {
	w      *responseWrapper
	r      *http.Request
	reqtc  transcoding.HTTPRequestTranscoder
	resptc transcoding.HTTPResponseTranscoder
	read   bool // not synchronized because Recv() cannot be called concurrently
	readCh chan struct{}
	sent   bool // not synchronized because Send() cannot be called concurrently
}

// TODO(renbou): support context for cancelling send when the server fails.
func (s *unaryHTTPStream) Send(_ context.Context, msg proto.Message) error {
	if s.sent {
		return status.Error(codes.Internal, "grpcbridge: tried sending second response on unary stream")
	}

	// Wait for request to be received, because after this we aren't guaranteed to be able to read the request body,
	// for example when using HTTP/1.1. See http.ResponseWriter.Write comment for more info.
	<-s.readCh

	b, err := s.resptc.Transcode(msg)
	if err != nil {
		return responseTranscodingError(err)
	}

	// Set here, because Write can perform a partial write and still return an error
	s.sent = true

	ct, _ := s.resptc.ContentType(msg) // don't care about whether the response is in binary/utf8
	s.w.Header()[contentTypeHeader] = []string{ct}

	if _, err = s.w.Write(b); err != nil {
		return status.Errorf(codes.Internal, "failed to write response body: %s", err)
	}

	return nil
}

func (s *unaryHTTPStream) Recv(_ context.Context, msg proto.Message) error {
	if s.read {
		return io.EOF
	}

	b, err := io.ReadAll(s.r.Body)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to read request body: %s", err)
	}

	s.read = true
	close(s.readCh)

	// Treat completely empty bodies as valid ones.
	if len(b) == 0 {
		return nil
	}

	return requestTranscodingError(s.reqtc.Transcode(b, msg))
}
