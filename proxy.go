package grpcbridge

import (
	"context"

	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/routing"
	"google.golang.org/grpc"
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
// This way of routing is used instead of relying on the [grpc.ServiceRegistrar] interface because
// GRPCProxy supports dynamic routing, which is not possible with a classic gRPC service, since it expects
// all gRPC services to be registered before launch.
type GRPCProxy struct {
	logger bridgelog.Logger
	router routing.GRPCRouter
}

// NewGRPCProxy constructs a new [*GRPCProxy] with the given router and options. When no options are provided, sane defaults are used.
func NewGRPCProxy(router routing.GRPCRouter, opts ...ProxyOption) *GRPCProxy {
	options := defaultProxyOptions()

	for _, opt := range opts {
		opt.applyProxy(&options)
	}

	return &GRPCProxy{
		logger: options.common.logger.WithComponent("grpcbridge.proxy"),
		router: router,
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

	logger := s.logger.With("target", route.Target.Name, "method", route.Method.RPCName)

	// TODO(renbou): timeouts for stream initiation and Recv/Sends
	outgoing, err := conn.Stream(incoming.Context(), route.Method.RPCName)
	if err != nil {
		return err
	}

	// Always clean up the outgoing stream by explicitly closing it.
	defer outgoing.Close()

	logger.Debug("began proxying gRPC stream")
	defer logger.Debug("ended proxying gRPC stream")

	return grpcadapter.ForwardServerToClient(incoming.Context(), grpcadapter.ForwardS2C{
		Method:   route.Method,
		Incoming: grpcServerStream{incoming},
		Outgoing: outgoing,
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

type proxyOptions struct {
	common options
}

func defaultProxyOptions() proxyOptions {
	return proxyOptions{
		common: defaultOptions(),
	}
}
