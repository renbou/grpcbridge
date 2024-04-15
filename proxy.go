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

type GPRCProxyOpts struct {
	Logger bridgelog.Logger
}

func (o GPRCProxyOpts) withDefaults() GPRCProxyOpts {
	if o.Logger == nil {
		o.Logger = defaultGRPCProxyOpts.Logger
	}

	return o
}

var defaultGRPCProxyOpts = &GPRCProxyOpts{
	Logger: bridgelog.Discard(),
}

type GRPCProxy struct {
	logger bridgelog.Logger
	router routing.GRPCRouter
}

func NewGRPCProxy(router routing.GRPCRouter, opts GPRCProxyOpts) *GRPCProxy {
	opts = opts.withDefaults()

	return &GRPCProxy{
		logger: opts.Logger.WithComponent("grpcbridge.proxy.grpc"),
		router: router,
	}
}

func (s *GRPCProxy) AsServerOption() grpc.ServerOption {
	return grpc.UnknownServiceHandler(s.StreamHandler)
}

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
