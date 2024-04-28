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
	"github.com/renbou/grpcbridge/routing"
	"github.com/renbou/grpcbridge/transcoding"
	"google.golang.org/grpc/codes"
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
}

func (o TranscodedWebSocketBridgeOpts) withDefaults() TranscodedWebSocketBridgeOpts {
	if o.Logger == nil {
		o.Logger = bridgelog.Discard()
	}

	return o
}

type TranscodedWebSocketBridge struct {
	logger     bridgelog.Logger
	upgrader   *gws.Upgrader
	router     routing.HTTPRouter
	transcoder transcoding.HTTPTranscoder
}

func NewTranscodedWebSocketBridge(router routing.HTTPRouter, transcoder transcoding.HTTPTranscoder, opts TranscodedWebSocketBridgeOpts) *TranscodedWebSocketBridge {
	opts = opts.withDefaults()
	logger := opts.Logger.WithComponent("grpcbridge.web")

	upgrader := gws.NewUpgrader(new(gwsHandler), &gws.ServerOption{
		ParallelEnabled:  false, // No point in parallel message processing, since we're using a single stream per request.
		CheckUtf8Enabled: true,
		Logger:           gwsLogger{logger},
	})

	return &TranscodedWebSocketBridge{
		logger:     logger,
		upgrader:   upgrader,
		router:     router,
		transcoder: transcoder,
	}
}

func (b *TranscodedWebSocketBridge) ServeHTTP(unwrappedRW http.ResponseWriter, r *http.Request) {
	req := routeTranscodedRequest(unwrappedRW, r, b.router, b.transcoder)
	if req == nil {
		return
	}

	// We know that the request is properly routed and supported by the transcoder,
	// lets perform a WebSocket upgrade now, and connect to the target only if everything goes well.
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

	if ok := req.connect(); !ok {
		return
	}

	// Always clean up the outgoing streams.
	defer req.outgoing.Close()

	logger := b.logger.With(
		"target", req.route.Target.Name,
		"grpc.method", req.route.Method.RPCName,
		"http.method", r.Method,
		"http.path", r.URL.Path,
		"http.params", req.route.PathParams,
	)
	logger.Debug("began handling WebSocket stream")
	defer logger.Debug("ended handling WebSocket stream")

	stream := &gwsStream{
		socket: socket,
		req:    req,
		events: make(chan gwsReadEvent),
		done:   make(chan struct{}),
	}

	// Store the whole stream state in the session so that it can be retrieved by OnMessage.
	// ReadLoop() will exit when the stream exits due to client/server closure, or when the client closes the WebSocket.
	socket.Session().Store(gwsStreamKey, stream)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		socket.ReadLoop()
	}()

	// r.Context is valid here even after Hijack() in Upgrade()
	err = grpcadapter.ForwardServerToClient(r.Context(), grpcadapter.ForwardS2C{
		Incoming: stream,
		Outgoing: req.outgoing,
		Method:   req.route.Method,
	})

	logger.Debug("WebSocket stream forwarding done", "error", err)

	// Close the WebSocket and notify the WebSocket handler to stop processing OnMessage,
	// if ReadLoop hasn't already exited (server error or EOF).
	if err == nil {
		socket.WriteClose(1000, []byte(""))
	} else if errors.Is(err, errExpectedBinary) || errors.Is(err, errExpectedText) {
		socket.WriteClose(1003, []byte(err.Error()))
	} else {
		socket.WriteClose(1001, []byte(err.Error()))
	}

	close(stream.done) // this allows OnMessage to instantly exit
	wg.Wait()          // just a safety measure to avoid leaks
}

type gwsReadEvent struct {
	data []byte
	err  error
}

// TODO(renbou): document the various stream flow states to properly showcase how the various errors and closures are handled.
type gwsStream struct {
	socket *gws.Conn
	req    *transcodedRequest

	// done is needed separately to the events channel so that a stream error
	// can properly notify OnMessage to stop trying to send any more events,
	// so that the ReadLoop can exit successfully.
	done   chan struct{}
	events chan gwsReadEvent

	// read is needed so that Recv() can be called multiple times for Unary methods, just as an extra safeguard.
	// using an atomic without any mutex is fine here because Recv() and OnMessage() cannot be called concurrently.
	read atomic.Bool
}

// TODO(renbou): support context for cancelling send when the server fails.
func (s *gwsStream) Send(_ context.Context, msg proto.Message) error {
	b, err := s.req.resptc.Transcode(msg)
	if err != nil {
		return responseTranscodingError(err)
	}

	code := gws.OpcodeText
	if _, binary := s.req.resptc.ContentType(msg); binary {
		code = gws.OpcodeBinary
	}

	if err := s.socket.WriteMessage(code, b); err != nil {
		return status.Errorf(codes.Internal, "failed to write response message: %s", err)
	}

	return nil
}

func (s *gwsStream) wantMessage() bool {
	return s.req.route.Method.ClientStreaming || (s.req.route.Binding.RequestBodyPath != "" && !s.read.Load())
}

func (s *gwsStream) Recv(ctx context.Context, msg proto.Message) error {
	var event gwsReadEvent

	// Only wait for a message when a body is actually required.
	if s.wantMessage() {
		select {
		case event = <-s.events:
		case <-s.done:
			return io.EOF
		case <-ctx.Done():
			return ctx.Err()
		}
	} else if s.read.Load() {
		return io.EOF
	}

	// set to true in both Recv() and OnMessage() to avoid relying on the atomic being instantly synced between the two goroutines.
	s.read.Store(true)

	// err written when the incoming side of the socket is closed with an error,
	// or closed by us due to other errors, which can't really be counted as a proper client-side closure.
	if event.err != nil {
		return event.err
	}

	return requestTranscodingError(s.req.reqtc.Transcode(event.data, msg))
}

type gwsHandler struct {
	gws.BuiltinEventHandler
}

func (b *gwsHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	streamAny, _ := socket.Session().Load(gwsStreamKey)
	stream := streamAny.(*gwsStream)

	if stream.wantMessage() {
		// set to true in both Recv() and OnMessage() to avoid relying on the atomic being instantly synced between the two goroutines.
		stream.read.Store(true)
	} else {
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
	case stream.events <- event: // events never closed, so no panic will occur here
	case <-stream.done:
	}
}

// OnClose needed to handle client-side closure or some unexpected error.
func (b *gwsHandler) OnClose(socket *gws.Conn, err error) {
	streamAny, _ := socket.Session().Load(gwsStreamKey)
	stream := streamAny.(*gwsStream)

	var eventErr error

	var closeErr *gws.CloseError
	if errors.As(err, &closeErr) {
		if closeErr.Code == 1000 || closeErr.Code == 0 {
			// Normal closure by client. Code == 0 set when one wasn't specified by the client.
			eventErr = io.EOF
		} else {
			// Closed by client due to error or some other reason.
			eventErr = status.Errorf(codes.Unavailable, "WebSocket closed with non-OK status code %d and reason %q", closeErr.Code, closeErr.Reason)
		}
	} else {
		// Closed by server (us), doesn't really matter what we write here.
		eventErr = err
	}

	select {
	case stream.events <- gwsReadEvent{err: eventErr}:
	case <-stream.done:
	}
}

type gwsLogger struct {
	bridgelog.Logger
}

func (l gwsLogger) Error(args ...any) {
	l.Logger.Error("gws WebSocket error", "error", fmt.Sprint(args...))
}
