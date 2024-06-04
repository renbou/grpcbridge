package webbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"slices"
	"strconv"
	"sync"

	"github.com/lxzan/gws"
	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/rpcutil"
	"github.com/renbou/grpcbridge/routing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// GRPCWebBridgeOpts define all the optional settings which can be set for [GRPCWebBridge] and [GRPCWebSocketBridge].
type GRPCWebBridgeOpts struct {
	// Logs are discarded by default.
	Logger bridgelog.Logger

	// If not set, the default [grpcadapter.ProxyForwarder] is created with default options.
	Forwarder grpcadapter.Forwarder
}

func (o GRPCWebBridgeOpts) withDefaults() GRPCWebBridgeOpts {
	if o.Logger == nil {
		o.Logger = bridgelog.Discard()
	}

	if o.Forwarder == nil {
		o.Forwarder = grpcadapter.NewProxyForwarder(grpcadapter.ProxyForwarderOpts{})
	}

	return o
}

// GRPCWebBridge is a gRPC bridge implementing the HTTP-based gRPC-Web protocol, as it is described in the [PROTOCOL-WEB] specification.
// It always returns a 200 OK HTTP response, writing the actual gRPC response in a Length-Prefixed message as part of the response body.
//
// [PROTOCOL-WEB]: https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-WEB.md
type GRPCWebBridge struct {
	logger    bridgelog.Logger
	router    routing.GRPCRouter
	forwarder grpcadapter.Forwarder
}

// NewGRPCWebBridge initializes a new [GRPCWebBridge] using the specified router and options.
// The router isn't optional, because no routers in grpcbridge can be constructed without some form of required args.
func NewGRPCWebBridge(router routing.GRPCRouter, opts GRPCWebBridgeOpts) *GRPCWebBridge {
	opts = opts.withDefaults()

	return &GRPCWebBridge{
		logger:    opts.Logger.WithComponent("grpcbridge.web"),
		router:    router,
		forwarder: opts.Forwarder,
	}
}

// ServeHTTP implements [net/http.Handler] so that the bridge is used as a normal HTTP handler.
func (b *GRPCWebBridge) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/grpc-web+proto")

	streamCtx := grpc.NewContextWithServerTransportStream(r.Context(), &gRPCWebServerStream{r: r})
	conn, route, err := b.router.RouteGRPC(streamCtx)
	if err != nil {
		writeTrailerWithStatus(rw, metadata.MD{}, status.Convert(err))
		return
	}

	logger := b.logger.With(
		"target", route.Target.Name,
		"grpc.method", route.Method.RPCName,
	)
	logger.Debug("began handling gRPC-Web HTTP request")
	defer logger.Debug("ended handling gRPC-Web HTTP request")

	incoming := &gRPCWebStream{rw: rw, r: r, trailer: metadata.MD{}}
	ctx := metadata.NewIncomingContext(r.Context(), headersToMD(r.Header))

	err = b.forwarder.Forward(ctx, grpcadapter.ForwardParams{
		Target:   route.Target,
		Service:  route.Service,
		Method:   route.Method,
		Incoming: incoming,
		Outgoing: conn,
	})

	writeTrailerWithStatus(rw, incoming.trailer, status.Convert(err))
}

type GRPCWebSocketBridge struct {
	logger    bridgelog.Logger
	upgrader  *gws.Upgrader
	router    routing.GRPCRouter
	forwarder grpcadapter.Forwarder
}

func NewGRPCWebSocketBridge(router routing.GRPCRouter, opts GRPCWebBridgeOpts) *GRPCWebSocketBridge {
	logger := opts.Logger.WithComponent("grpcbridge.web")
	opts = opts.withDefaults()

	upgrader := gws.NewUpgrader(new(gwsGRPCWebHandler), &gws.ServerOption{
		ParallelEnabled:  false, // No point in parallel message processing, since we're using a single stream per request.
		CheckUtf8Enabled: false, // In contrast to TranscodedWebSocketBridge, all messages are binary, so utf-8 checking isn't needed
		Logger:           gwsLogger{logger},
		SubProtocols:     []string{"grpc-websockets"},
	})

	return &GRPCWebSocketBridge{
		logger:    logger,
		upgrader:  upgrader,
		router:    router,
		forwarder: opts.Forwarder,
	}
}

func (b *GRPCWebSocketBridge) ServeHTTP(unwrappedRW http.ResponseWriter, r *http.Request) {
	// Mostly the same comments as in TranscodedWebSocketBridge.ServeHTTP apply here.
	// However, the flows differ quite a lot, which is why not much code is shared.
	socket, err := b.upgrader.Upgrade(unwrappedRW, r)
	if err != nil {
		writeError(&responseWrapper{ResponseWriter: unwrappedRW}, r, nil, status.Error(codes.Internal, err.Error()))
		return
	}

	defer socket.NetConn().Close()

	stream := &gRPCWebSocketStream{
		socket:     socket,
		metadataCh: make(chan metadata.MD),
		events:     make(chan gwsReadEvent),
		done:       make(chan struct{}),
		trailer:    metadata.MD{},
	}
	socket.Session().Store(gwsStreamKey, stream)

	ctx, cancel := context.WithCancel(r.Context())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		socket.ReadLoop()
	}()

	defer func() {
		close(stream.done)
		wg.Wait()
	}()

	var md metadata.MD
	select {
	// ReadLoop ended due to internal error or due to invalid metadata
	case <-ctx.Done():
		return
	case md = <-stream.metadataCh:
	}

	// Finally perform routing after request has been initialized properly.
	conn, route, err := b.router.RouteGRPC(grpc.NewContextWithServerTransportStream(r.Context(), &gRPCWebServerStream{r: r}))
	if err != nil {
		stream.sendTrailer(status.Convert(err))
		return
	}

	logger := b.logger.With(
		"target", route.Target.Name,
		"grpc.method", route.Method.RPCName,
	)
	logger.Debug("began handling gRPC-Web HTTP request")
	defer logger.Debug("ended handling gRPC-Web HTTP request")

	err = b.forwarder.Forward(metadata.NewIncomingContext(ctx, md), grpcadapter.ForwardParams{
		Target:   route.Target,
		Service:  route.Service,
		Method:   route.Method,
		Incoming: stream,
		Outgoing: conn,
	})

	stream.sendTrailer(status.Convert(err))
}

type gRPCWebStream struct {
	rw      http.ResponseWriter
	r       *http.Request
	trailer metadata.MD
}

func (s *gRPCWebStream) Send(ctx context.Context, msg proto.Message) error {
	return withCtx(ctx, func() error { return s.send(msg) })
}

func (s *gRPCWebStream) send(msg proto.Message) error {
	data, err := lpmMessage(msg)
	if err != nil {
		return err
	}

	if _, err := s.rw.Write(data); err != nil {
		return status.Errorf(codes.Unavailable, "failed to write length-prefixed message: %s", err)
	}

	return nil
}

func (s *gRPCWebStream) Recv(ctx context.Context, msg proto.Message) error {
	return withCtx(ctx, func() error { return s.recv(msg) })
}

func (s *gRPCWebStream) recv(msg proto.Message) error {
	header := make([]byte, 5)

	if _, err := io.ReadFull(s.r.Body, header); errors.Is(err, io.EOF) {
		return io.EOF
	} else if err != nil {
		return status.Errorf(codes.Unavailable, "failed to read length-prefixed message header: %s", err)
	}

	length := binary.BigEndian.Uint32(header[1:5])
	if length < 1 {
		// empty message doesn't need unmarshaling
		return nil
	}

	data := make([]byte, min(length, 1<<22)) // 4 MB is the max message size used by gRPC

	if _, err := io.ReadFull(s.r.Body, data); err != nil {
		return status.Errorf(codes.Unavailable, "failed to read length-prefixed message body: %s", err)
	}

	if err := proto.Unmarshal(data, msg); err != nil {
		// Avoid wrapping to replicate gRPC-Go behaviour
		return err
	}

	return nil
}

func (s *gRPCWebStream) SetHeader(md metadata.MD) {
	appendHeaders(s.rw, md)
}

func (s *gRPCWebStream) SetTrailer(md metadata.MD) {
	s.trailer = md
}

type gRPCWebSocketStream struct {
	socket *gws.Conn

	receivedMD bool // not synced because written only in OnMessage
	metadataCh chan metadata.MD

	sentMD  bool
	header  metadata.MD
	trailer metadata.MD

	closed bool
	done   chan struct{}
	events chan gwsReadEvent
}

func (s *gRPCWebSocketStream) Recv(ctx context.Context, msg proto.Message) error {
	var event gwsReadEvent
	select {
	case ev, ok := <-s.events:
		if !ok {
			return io.EOF
		}
		event = ev
	case <-ctx.Done():
		return rpcutil.ContextError(ctx.Err())
	}

	if event.err != nil {
		return event.err
	}

	return proto.Unmarshal(event.data, msg)
}

func (s *gRPCWebSocketStream) Send(ctx context.Context, msg proto.Message) error {
	return withCtx(ctx, func() error { return s.send(msg) })
}

func (s *gRPCWebSocketStream) send(msg proto.Message) error {
	if !s.sentMD {
		s.sentMD = true
		if err := s.socket.WriteMessage(gws.OpcodeBinary, lpmTrailer(s.header)); err != nil {
			return status.Errorf(codes.Unavailable, "failed to send metadata: %s", err)
		}
	}

	data, err := lpmMessage(msg)
	if err != nil {
		return err
	}

	if err := s.socket.WriteMessage(gws.OpcodeBinary, data); err != nil {
		return status.Errorf(codes.Unavailable, "failed to send message: %s", err)
	}

	return nil
}

func (s *gRPCWebSocketStream) SetHeader(md metadata.MD) {
	s.header = md
}

func (s *gRPCWebSocketStream) SetTrailer(md metadata.MD) {
	s.trailer = md
}

func (s *gRPCWebSocketStream) sendTrailer(st *status.Status) {
	_ = s.socket.WriteMessage(gws.OpcodeBinary, lpmTrailer(trailerWithStatus(s.trailer, st)))
	s.socket.WriteClose(1000, []byte{})
}

type gwsGRPCWebHandler struct {
	gws.BuiltinEventHandler
}

func (b *gwsGRPCWebHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	streamAny, _ := socket.Session().Load(gwsStreamKey)
	stream := streamAny.(*gRPCWebSocketStream)

	// just ignore any messages after the client-side stream has been closed
	if stream.closed {
		return
	}

	data := message.Bytes()

	if !stream.receivedMD {
		b.readMD(stream, data)
		return
	}

	var event gwsReadEvent

	// handle flow control byte
	if len(data) > 0 {
		stream.closed = data[0] == 1
	} else {
		event.err = status.Error(codes.InvalidArgument, "expected flow control byte")
	}

	// handle length-prefixed message, but length doesn't actually matter for websockets
	if len(data) > 6 {
		event.data = data[6:]
		select {
		case stream.events <- event: // events closed only by OnMessage, so no panic will occur here
		case <-stream.done:
		}
	} else if event.err == nil && len(data) != 1 {
		event.err = status.Error(codes.InvalidArgument, "expected length-prefixed message header")
	}

	if stream.closed {
		close(stream.events)
	}
}

func (b *gwsGRPCWebHandler) readMD(stream *gRPCWebSocketStream, data []byte) {
	// extra \r\n needed to properly end textproto header
	tp := textproto.NewReader(bufio.NewReader(bytes.NewReader(slices.Concat(data, []byte("\r\n")))))

	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		stream.sendTrailer(status.New(codes.InvalidArgument, "expected metadata as valid HTTP/1.1 header"))
		return
	}

	stream.receivedMD = true
	stream.metadataCh <- metadata.MD(mimeHeader)
}

type gRPCWebServerStream struct {
	r *http.Request
}

func (s *gRPCWebServerStream) Method() string {
	return s.r.URL.Path
}

func (s *gRPCWebServerStream) SendHeader(md metadata.MD) error {
	return nil
}

func (s *gRPCWebServerStream) SetHeader(md metadata.MD) error {
	return nil
}

func (s *gRPCWebServerStream) SetTrailer(md metadata.MD) error {
	return nil
}

func trailerWithStatus(md metadata.MD, st *status.Status) metadata.MD {
	md.Set("grpc-status", strconv.Itoa(int(st.Code())))
	md.Set("grpc-message", url.PathEscape(st.Message()))
	return md
}

func writeTrailerWithStatus(rw http.ResponseWriter, md metadata.MD, st *status.Status) {
	_, _ = rw.Write(lpmTrailer(trailerWithStatus(md, st)))
}

func lpmMessage(msg proto.Message) ([]byte, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	header := []byte{0x0, 0x0, 0x0, 0x0, 0x0}
	binary.BigEndian.PutUint32(header[1:5], uint32(len(data)))

	return append(header, data...), nil
}

// format gRPC-Web trailer as gRPC length-prefixed message.
func lpmTrailer(md metadata.MD) []byte {
	buf := bytes.NewBuffer(nil)

	for k, vs := range md {
		for _, v := range vs {
			buf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
	}

	// 0x80 is a header frame according to gRPC PROTOCOL-WEB spec
	header := []byte{0x80, 0x0, 0x0, 0x0, 0x0}
	binary.BigEndian.PutUint32(header[1:5], uint32(buf.Len()))

	return append(header, buf.Bytes()...)
}
