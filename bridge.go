package grpcbridge

import (
	"log/slog"
	"net/http"

	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/ascii"
	"github.com/renbou/grpcbridge/routing"
	"github.com/renbou/grpcbridge/transcoding"
	"github.com/renbou/grpcbridge/webbridge"
)

var _ = slog.Logger{}

// Option configures common grpcbridge options, such as the logger.
type Option interface {
	RouterOption
	ForwarderOption
	ProxyOption
	BridgeOption
}

// Router unifies the [routing.GRPCRouter] and [routing.HTTPRouter] interfaces,
// providing routing support for both [GRPCProxy] and [WebBridge] for all kinds of incoming calls.
// It is implemented by [ReflectionRouter], which is the default routing component used by grpcbridge itself.
type Router interface {
	routing.GRPCRouter
	routing.HTTPRouter
}

// BridgeOption configures the various bridging handlers used by [WebBridge].
type BridgeOption interface {
	applyBridge(o *bridgeOptions)
}

// WebBridge provides a single entrypoint for all web-originating requests which are bridged to target gRPC services with various applied transformations.
// It uses a [Router] which combines gRPC and HTTP routing to support both typical REST-like API (HTTP, WebSockets, Server-Sent Events), and modern gRPC-Web APIs.
// Meanwhile, forwarding of all incoming requests is performed using a [Forwarder], which can be shared with [GRPCProxy] in order to unify how all outgoing gRPC streams are run.
//
// WebBridge itself is no more than a thin wrapper around various handlers from the [webbridge] package, multiplexing incoming requests to the appropriate
// router based on headers such as Content-Type, Connection, and Upgrade. For more info, see the [WebBridge.ServeHTTP] method.
type WebBridge struct {
	transcodedHTTPBridge      *webbridge.TranscodedHTTPBridge
	transcodedWebSocketBridge *webbridge.TranscodedWebSocketBridge
}

// NewWebBridge constructs a new [*WebBridge] with the given router and options.
// When no options are provided, the [transcoding.StandardTranscoder] and the
// underlying bridge handlers from [webbridge] will be initialized with their default options.
func NewWebBridge(router Router, opts ...BridgeOption) *WebBridge {
	options := defaultBridgeOptions()

	for _, opt := range opts {
		opt.applyBridge(&options)
	}

	transcoder := transcoding.NewStandardTranscoder(options.transcoderOpts)

	transcodedHTTPBridge := webbridge.NewTranscodedHTTPBridge(router, webbridge.TranscodedHTTPBridgeOpts{
		Logger:     options.common.logger,
		Transcoder: transcoder,
		Forwarder:  options.common.forwarder,
	})

	transcodedWebSocketBridge := webbridge.NewTranscodedWebSocketBridge(router, webbridge.TranscodedWebSocketBridgeOpts{
		Logger:     options.common.logger,
		Transcoder: transcoder,
		Forwarder:  options.common.forwarder,
	})

	return &WebBridge{transcodedHTTPBridge: transcodedHTTPBridge, transcodedWebSocketBridge: transcodedWebSocketBridge}
}

// ServeHTTP implements [net/http.Handler] and routes the request to the appropriate bridging handler according to these rules:
//  1. WebSocket upgrades (Connection: Upgrade and Upgrade: WebSocket) are handled by [webbridge.TranscodedWebSocketBridge].
//  2. All other requests are handled by [webbridge.TranscodedHTTPBridge].
func (b *WebBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Case-insensitive comparison as specified in the RFC https://datatracker.ietf.org/doc/html/rfc6455#section-4.2.1.
	if ascii.EqualFold(r.Header.Get("Connection"), "upgrade") && ascii.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		b.transcodedWebSocketBridge.ServeHTTP(w, r)
		return
	}

	b.transcodedHTTPBridge.ServeHTTP(w, r)
}

// WithMarshalers allows using custom marshalers for transcoding-based handlers,
// which will be picked according to the content types they support.
// By default, [transcoding.DefaultJSONMarshaler] is used.
func WithMarshalers(marshalers []transcoding.Marshaler) BridgeOption {
	return newFuncBridgeOption(func(o *bridgeOptions) {
		o.transcoderOpts.Marshalers = marshalers
	})
}

// WithDefaultMarshaler allows setting a custom default marshaler for transcoding-based handlers,
// which will be used for requests which do not specify the Content-Type header.
// By default, [transcoding.DefaultJSONMarshaler] is used.
func WithDefaultMarshaler(m transcoding.Marshaler) BridgeOption {
	return newFuncBridgeOption(func(o *bridgeOptions) {
		o.transcoderOpts.DefaultMarshaler = m
	})
}

// WithLogger configures the logger to be used by grpcbridge components. By default all logs are discarded.
//
// Taking the full Logger interface allows you to configure all functionality however you want,
// however you can also use [bridgelog.WrapPlainLogger] to wrap a basic logger such as [slog.Logger].
func WithLogger(logger bridgelog.Logger) Option {
	return newFuncOption(func(o *options) {
		o.logger = logger
	})
}

// WithForwarder configures the gRPC call forwarder to be used by [GRPCProxy] or [WebBridge].
// By default each of these components would create their own [Forwarder] with default options.
//
// The default grpcbridge [Forwarder] can be customized using [ForwarderOption]s,
// and passed using this option to [NewGRPCProxy] or [NewWebBridge].
func WithForwarder(forwarder grpcadapter.Forwarder) Option {
	return newFuncOption(func(o *options) {
		o.forwarder = forwarder
	})
}

type options struct {
	logger    bridgelog.Logger
	forwarder grpcadapter.Forwarder
}

func defaultOptions() options {
	return options{
		logger:    bridgelog.Discard(),
		forwarder: NewForwarder(),
	}
}

type funcOption struct {
	f func(*options)
}

func (f *funcOption) applyRouter(o *routerOptions) {
	f.f(&o.common)
}

func (f *funcOption) applyProxy(o *proxyOptions) {
	f.f(&o.common)
}

func (f *funcOption) applyBridge(o *bridgeOptions) {
	f.f(&o.common)
}

func (f *funcOption) applyForwarder(o *forwarderOptions) {
	f.f(&o.common)
}

func newFuncOption(f func(*options)) Option {
	return &funcOption{f: f}
}

type bridgeOptions struct {
	common         options
	transcoderOpts transcoding.StandardTranscoderOpts
}

func defaultBridgeOptions() bridgeOptions {
	return bridgeOptions{common: defaultOptions()}
}

type funcBridgeOption struct {
	f func(*bridgeOptions)
}

func (f *funcBridgeOption) applyBridge(o *bridgeOptions) {
	f.f(o)
}

func newFuncBridgeOption(f func(*bridgeOptions)) BridgeOption {
	return &funcBridgeOption{f: f}
}
