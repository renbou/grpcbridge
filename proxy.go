package grpcbridge

import (
	"context"
	"io"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"google.golang.org/grpc"
)

var _ grpc.StreamHandler = (*GRPCProxy)(nil).StreamHandler

type GRPCRouter interface {
	RouteGRPC(context.Context) (grpcadapter.Connection, *bridgedesc.Method, error)
}

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
	router GRPCRouter
}

func NewGRPCProxy(router GRPCRouter, opts GPRCProxyOpts) *GRPCProxy {
	opts = opts.withDefaults()

	return &GRPCProxy{
		logger: opts.Logger.WithComponent("grpcbridge.proxy.grpc"),
		router: router,
	}
}

func (s *GRPCProxy) AsOption() grpc.ServerOption {
	return grpc.UnknownServiceHandler(s.StreamHandler)
}

func (s *GRPCProxy) StreamHandler(_ any, incoming grpc.ServerStream) error {
	conn, desc, err := s.router.RouteGRPC(incoming.Context())
	if err != nil {
		return err
	}

	logger := s.logger.With("method", desc.RPCName)

	// TODO(renbou): timeouts for stream initiation and Recv/Sends
	outgoing, err := conn.BiDiStream(incoming.Context(), desc.RPCName)
	if err != nil {
		return err
	}

	// Always clean up the outgoing stream by explicitly closing it.
	defer outgoing.Close()

	i2oErrCh := make(chan proxyingError, 1)
	o2iErrCh := make(chan proxyingError, 1)

	go func() {
		i2oErrCh <- s.forwardIncomingToOutgoing(logger, incoming, outgoing, desc.Input)
	}()

	go func() {
		o2iErrCh <- s.forwardOutgoingToIncoming(logger, outgoing, incoming, desc.Output)
	}()

	// Handle proxying errors and return them when needed,
	// closing the outgoing stream without waiting for the other goroutine to complete.
	// It will not deadlock thanks to the buffer of the channels,
	// and is guaranteed to receive an error from operations on the outgoing channel.
	for {
		select {
		case perr := <-i2oErrCh:
			if perr.incomingErr == io.EOF || perr.outgoingErr == io.EOF {
				// io.EOF can arrive either from incoming.RecvMsg or outgoing.Send,
				// and in both cases we need to receive further responses via outgoing.Recv until it returns a proper error.
				// For this to work properly, forward a CloseSend to outgoing, even though io.EOF from outgoing.Send might've already done so.
				// Calling it here is safe since outgoing.Send isn't going to get called anymore anyway.
				outgoing.CloseSend()
			} else if perr.incomingErr != nil {
				return perr.incomingErr
			} else {
				// perr.outgoingErr is guaranteed to be non-nil by forwardIncomingToOutgoing.
				return perr.outgoingErr
			}
		case perr := <-o2iErrCh:
			if perr.outgoingErr == io.EOF {
				// io.EOF from outgoing.Recv means that the stream has been properly closed, forward this closure to the incoming stream.
				return nil
			} else if perr.incomingErr != nil {
				return perr.incomingErr
			} else {
				// perr.outgoingErr is guaranteed to be non-nil by forwardOutgoingToIncoming.
				return perr.outgoingErr
			}
		}
	}
}

// proxyingError is a utility struct to distinguish errors received from incoming.RecvMsg/SendMsg and outgoing.Recv/Send calls.
// This is mostly needed for properly handling io.EOF errors which might come from all sorts of calls.
type proxyingError struct {
	incomingErr error
	outgoingErr error
}

func (s *GRPCProxy) forwardIncomingToOutgoing(logger bridgelog.Logger, incoming grpc.ServerStream, outgoing grpcadapter.BiDiStream, message bridgedesc.Message) proxyingError {
	defer logger.Debug("ending forwarding incoming to outgoing")

	for {
		msg := message.New()

		if err := incoming.RecvMsg(msg); err != nil {
			return proxyingError{incomingErr: err}
		}

		if err := outgoing.Send(incoming.Context(), msg); err != nil {
			return proxyingError{outgoingErr: err}
		}
	}
}

func (s *GRPCProxy) forwardOutgoingToIncoming(logger bridgelog.Logger, outgoing grpcadapter.BiDiStream, incoming grpc.ServerStream, message bridgedesc.Message) proxyingError {
	defer logger.Debug("ending forwarding outgoing to incoming")

	for {
		msg := message.New()

		if err := outgoing.Recv(incoming.Context(), msg); err != nil {
			return proxyingError{outgoingErr: err}
		}

		if err := incoming.SendMsg(msg); err != nil {
			return proxyingError{incomingErr: err}
		}
	}
}
