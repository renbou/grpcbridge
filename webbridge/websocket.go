package webbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/lxzan/gws"
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

var (
	errExpectedText   = status.Errorf(codes.InvalidArgument, "received binary message instead of text")
	errExpectedBinary = status.Errorf(codes.InvalidArgument, "received text message instead of binary")
)

// gwsStreamKey is used to store the complete stream state in gws socket sessions,
// which allows reusing a single upgrader for all requests.
const gwsStreamKey = "grpcbridge\x00request"

// TranscodedWebSocketBridgeOpts define all the optional settings which can be set for [TranscodedWebSocketBridge].
type TranscodedWebSocketBridgeOpts struct {
	// Logs are discarded by default.
	Logger bridgelog.Logger

	// If not set, the default [transcoding.StandardTranscoder] is created with default options.
	Transcoder transcoding.HTTPTranscoder

	// If not set, the default [grpcadapter.ProxyForwarder] is created with default options.
	Forwarder grpcadapter.Forwarder

	// MetadataParam specifies the name of the query parameter to be parsed as a map containing the metadata to be forwarded.
	// This is needed for WebSockets since there's no way to set headers on the WebSocket handshake request through the WebSocket web API.
	//
	// If not set, _metadata is used. For more info about the format, see [TranscodedWebSocketBridge.ServeHTTP].
	MetadataParam string
}

func (o TranscodedWebSocketBridgeOpts) withDefaults() TranscodedWebSocketBridgeOpts {
	if o.Logger == nil {
		o.Logger = bridgelog.Discard()
	}

	if o.Transcoder == nil {
		o.Transcoder = transcoding.NewStandardTranscoder(transcoding.StandardTranscoderOpts{})
	}

	if o.Forwarder == nil {
		o.Forwarder = grpcadapter.NewProxyForwarder(grpcadapter.ProxyForwarderOpts{})
	}

	if o.MetadataParam == "" {
		o.MetadataParam = defaultMetadataParam
	}

	return o
}

type TranscodedWebSocketBridge struct {
	logger        bridgelog.Logger
	upgrader      *gws.Upgrader
	router        routing.HTTPRouter
	transcoder    transcoding.HTTPTranscoder
	forwarder     grpcadapter.Forwarder
	metadataParam string
}

// NewTranscodedWebSocketBridge initializes a new [TranscodedWebSocketBridge] using the specified router and options.
// The router isn't optional, because no routers in grpcbridge can be constructed without some form of required args.
func NewTranscodedWebSocketBridge(router routing.HTTPRouter, opts TranscodedWebSocketBridgeOpts) *TranscodedWebSocketBridge {
	opts = opts.withDefaults()
	logger := opts.Logger.WithComponent("grpcbridge.web")

	upgrader := gws.NewUpgrader(new(gwsHandler), &gws.ServerOption{
		ParallelEnabled:  false, // No point in parallel message processing, since we're using a single stream per request.
		CheckUtf8Enabled: true,
		Logger:           gwsLogger{logger},
	})

	return &TranscodedWebSocketBridge{
		logger:        logger,
		upgrader:      upgrader,
		router:        router,
		transcoder:    opts.Transcoder,
		forwarder:     opts.Forwarder,
		metadataParam: opts.MetadataParam,
	}
}

func (b *TranscodedWebSocketBridge) ServeHTTP(unwrappedRW http.ResponseWriter, r *http.Request) {
	md := parseMetadataQuery(r, b.metadataParam)

	req := routeTranscodedRequest(unwrappedRW, r, b.router, b.transcoder)
	if req == nil {
		return
	}

	// TODO(renbou): check that only streaming requests are handled here.

	socket, err := b.upgrader.Upgrade(unwrappedRW, r)
	if err != nil {
		// Upgrade() will write an error if Hijack() was successful, but if it wasn't,
		// then no error will be written, so we need to write one ourselves just in case.
		// This is a no-op if Upgrade() successfully Hijack()ed the request.
		writeError(req.w, req.r, req.resptc, status.Error(codes.Internal, err.Error()))
		return
	}

	// Always clean up the incoming socket to avoid any potential leaks.
	defer socket.NetConn().Close()

	stream := &gwsStream{
		socket: socket,
		req:    req,
		events: make(chan gwsReadEvent),
		done:   make(chan struct{}),
	}

	// Store the whole stream state in the session so that it can be retrieved by OnMessage.
	// ReadLoop() will exit when the stream exits due to client/server closure, or when the client closes the WebSocket.
	socket.Session().Store(gwsStreamKey, stream)

	// End of forwarding will notify ReadLoop() to exit via close(done) and WriteClose().
	// However, ReadLoop() must also have a way to notify the forwarding proccess to handle cases where the client closes the connection.
	// This context allows us to immediately cancel Forward() once we know no client is listening for any more responses.
	// NB: r.Context is valid here even after Hijack() in Upgrade()
	ctx, cancel := context.WithCancel(r.Context())

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer cancel()
		socket.ReadLoop()
	}()

	// Even though web clients aren't able to set metadata in headers, it's still useful to support it for other potential clients.
	ctx = metadata.NewIncomingContext(ctx, metadata.Join(md, headersToMD(r.Header)))

	logger := b.logger.With(
		"target", req.route.Target.Name,
		"grpc.method", req.route.Method.RPCName,
		"http.method", r.Method,
		"http.path", r.URL.Path,
		"http.params", req.route.PathParams,
	)
	logger.Debug("began handling WebSocket stream")
	defer logger.Debug("ended handling WebSocket stream")

	err = b.forwarder.Forward(ctx, grpcadapter.ForwardParams{
		Target:   req.route.Target,
		Service:  req.route.Service,
		Method:   req.route.Method,
		Incoming: stream,
		Outgoing: req.conn,
	})

	logger.Debug("WebSocket stream forwarding done", "error", err)

	// Close the WebSocket and notify ReadLoop() to stop processing OnMessage, if it hasn't already.
	code, reason := websocketError(err)
	socket.WriteClose(code, []byte(reason))

	close(stream.done) // this allows OnMessage to instantly exit
	wg.Wait()          // just a safety measure to avoid leaks
}

func websocketError(err error) (code uint16, reason string) {
	if err == nil {
		return 1000, ""
	}

	code = 1001
	if errors.Is(err, errExpectedBinary) || errors.Is(err, errExpectedText) {
		code = 1003
	}

	if st, ok := status.FromError(err); ok {
		// more compact form because ws has ~123 bytes limit on the reason
		reason = fmt.Sprintf("code %s: %s", st.Code(), st.Message())
	} else {
		reason = err.Error()
	}

	return code, reason
}

type gwsReadEvent struct {
	data []byte
	err  error
}

// TODO(renbou): document the various stream flow states to properly showcase how the various errors and closures are handled.
type gwsStream struct {
	socket *gws.Conn
	req    *transcodedRequest

	// done is needed separately to the events channel so that OnMessage can be safely notified
	// to stop trying to send any more events, so that the ReadLoop can exit successfully.
	done   chan struct{}
	events chan gwsReadEvent

	sendActive atomic.Bool
	recvActive atomic.Bool
	// used to ignore messages for unary requests
	alreadyRead atomic.Bool
}

func (s *gwsStream) Send(ctx context.Context, msg proto.Message) error {
	if !s.sendActive.CompareAndSwap(false, true) {
		panic("grpcbridge: Send() called concurrently on gwsStream")
	}
	defer s.sendActive.Store(false)

	errChan := make(chan error, 1)
	go func() {
		errChan <- s.send(msg)
	}()

	select {
	case <-ctx.Done():
		return rpcutil.ContextError(ctx.Err())
	case err := <-errChan:
		return err
	}
}

func (s *gwsStream) send(msg proto.Message) error {
	b, err := s.req.resptc.Transcode(msg)
	if err != nil {
		return responseTranscodingError(err)
	}

	code := gws.OpcodeText
	if _, binary := s.req.resptc.ContentType(msg); binary {
		code = gws.OpcodeBinary
	}

	// WriteMessage will be unblocked by socket.NetConn().Close() in ServeHTTP, if not before.
	if err := s.socket.WriteMessage(code, b); err != nil {
		return status.Errorf(codes.Internal, "failed to write response message: %s", err)
	}

	return nil
}

func (s *gwsStream) Recv(ctx context.Context, msg proto.Message) error {
	if !s.recvActive.CompareAndSwap(false, true) {
		panic("grpcbridge: Recv() called concurrently on unaryHTTPStream")
	}
	defer s.recvActive.Store(false)

	var event gwsReadEvent

	// Only wait for a message when an event/body is actually required.
	if s.req.route.Method.ClientStreaming || s.req.route.Binding.RequestBodyPath != "" {
		select {
		case ev, ok := <-s.events:
			if !ok {
				return io.EOF // events channel closed by OnMessage for unary requests
			}
			event = ev
		case <-ctx.Done():
			return rpcutil.ContextError(ctx.Err())
		}
	}

	// err written when the incoming side of the socket is closed by us due to other errors,
	// which can't really be counted as a proper client-side closure.
	if event.err != nil {
		return event.err
	}

	return requestTranscodingError(s.req.reqtc.Transcode(event.data, msg))
}

// WebSockets don't support headers, and they can only be returned during the upgrade, which would be just way too tedious to implement.
func (s *gwsStream) SetHeader(md metadata.MD) {}

// WebSockets don't support trailers AT ALL - there's no way to send them after the upgrade.
func (s *gwsStream) SetTrailer(md metadata.MD) {}

type gwsHandler struct {
	gws.BuiltinEventHandler
}

func (b *gwsHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	streamAny, _ := socket.Session().Load(gwsStreamKey)
	stream := streamAny.(*gwsStream)

	if !(stream.req.route.Method.ClientStreaming || (stream.req.route.Binding.RequestBodyPath != "" && stream.alreadyRead.CompareAndSwap(false, true))) {
		return
	}

	_, expectBinary := stream.req.reqtc.ContentType()
	messageBinary := message.Opcode != gws.OpcodeText

	var event gwsReadEvent

	// send error as event, the stream/socket will be closed when the error is handled.
	if messageBinary != expectBinary {
		if expectBinary {
			event = gwsReadEvent{err: errExpectedBinary}
		} else {
			event = gwsReadEvent{err: errExpectedText}
		}
	} else {
		event = gwsReadEvent{data: message.Data.Bytes()}
	}

	select {
	case stream.events <- event: // events closed only by OnMessage, so no panic will occur here
	case <-stream.done:
	}

	if !stream.req.route.Method.ClientStreaming {
		// only one instance of OnMessage can get to this statement due to the alreadyRead check above.
		close(stream.events)
	}
}

type gwsLogger struct {
	bridgelog.Logger
}

func (l gwsLogger) Error(args ...any) {
	l.Logger.Error("gws WebSocket error", "error", fmt.Sprint(args...))
}
