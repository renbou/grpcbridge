package grpcbridge

import (
	"context"

	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/routing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

var _ grpc.StreamHandler = (*GRPCProxy)(nil).StreamHandler

// ProxyOption configures gRPC proxying options.
type ProxyOption interface {
	applyProxy(*proxyOptions)
}

// GRPCProxy is a basic gRPC proxy implementation which uses a [routing.GRPCRouter] to route incoming gRPC requests
// to the proper target services. It should be registered with a [grpc.Server] as a [grpc.UnknownServiceHandler],
// which is precisely what the [GRPCProxy.AsServerOption] helper method returns.
//
// This way of routing is used instead of relying on the [grpc.ServiceRegistrar] interface because
// GRPCProxy supports dynamic routing, which is not possible with a classic gRPC service, since it expects
// all gRPC services to be registered before launch.
//
// Actual forwarding of routed calls is performed using a [Forwarder], which is a separate entity so that it can be
// easily shared between the both GRPCProxy and [WebBridge].
type GRPCProxy struct {
	logger    bridgelog.Logger
	router    routing.GRPCRouter
	forwarder grpcadapter.Forwarder
}

// NewGRPCProxy constructs a new [*GRPCProxy] with the given router and options. When no options are provided, sane defaults are used.
// The router is the only required parameter because without it there is no way to figure out over which gRPC connection requests must be proxied.
// By default, a new [Forwarder] is created with default options, but [WithForwarder] can be used to specify a custom call forwarder.
func NewGRPCProxy(router routing.GRPCRouter, opts ...ProxyOption) *GRPCProxy {
	options := defaultProxyOptions()

	for _, opt := range opts {
		opt.applyProxy(&options)
	}

	return &GRPCProxy{
		logger:    options.common.logger.WithComponent("grpcbridge.proxy"),
		router:    router,
		forwarder: options.common.forwarder,
	}
}

// AsServerOption returns a [grpc.ServerOption] which can be used to register this proxy with
// a gRPC server as the handler to be used for all unknown services.
func (s *GRPCProxy) AsServerOption() grpc.ServerOption {
	return grpc.UnknownServiceHandler(s.StreamHandler)
}

// StreamHandler allows proxying any incoming gRPC streams.
// It uses the router's RouteGRPC method with the incoming context to get a connection to the target
// and the description of the method. The actual forwarding is performed using the generic [grpcadapter.ForwardServerToClient] function.
func (s *GRPCProxy) StreamHandler(_ any, incoming grpc.ServerStream) error {
	conn, route, err := s.router.RouteGRPC(incoming.Context())
	if err != nil {
		return err
	}

	logger := s.logger.With("target", route.Target.Name, "grpc.method", route.Method.RPCName)
	logger.Debug("began proxying gRPC stream")
	defer logger.Debug("ended proxying gRPC stream")

	return s.forwarder.Forward(incoming.Context(), grpcadapter.ForwardParams{
		Target:   route.Target,
		Service:  route.Service,
		Method:   route.Method,
		Incoming: grpcServerStream{incoming},
		Outgoing: conn,
	})
}

type grpcServerStream struct {
	grpc.ServerStream
}

func (s grpcServerStream) Recv(_ context.Context, msg proto.Message) error {
	return s.ServerStream.RecvMsg(msg)
}

func (s grpcServerStream) Send(_ context.Context, msg proto.Message) error {
	return s.ServerStream.SendMsg(msg)
}

func (s grpcServerStream) SetHeader(md metadata.MD) {
	_ = s.ServerStream.SetHeader(md)
}

func (s grpcServerStream) SetTrailer(md metadata.MD) {
	s.ServerStream.SetTrailer(md)
}

type proxyOptions struct {
	common options
}

func defaultProxyOptions() proxyOptions {
	return proxyOptions{
		common: defaultOptions(),
	}
}
