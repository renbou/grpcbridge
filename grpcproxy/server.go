package grpcproxy

import (
	"io"

	"github.com/renbou/grpcbridge"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/route"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Assertion to make sure that Handler can be registered as valid gRPC stream handler.
var _ grpc.StreamHandler = (*Server)(nil).Handler

type Server struct {
	logger grpcbridge.Logger
	router *route.ServiceRouter
}

func NewServer(logger grpcbridge.Logger, router *route.ServiceRouter) *Server {
	return &Server{
		logger: logger.WithComponent("grpcbridge.proxy.grpc"),
		router: router,
	}
}

func (s *Server) Handler(_ any, incoming grpc.ServerStream) error {
	methodName, ok := grpc.Method(incoming.Context())
	if !ok {
		s.logger.Error("no method name in stream context, unable to route request")
		return status.Errorf(codes.Internal, "no method name in stream context, unable to route request")
	}

	logger := s.logger.With("method", methodName)

	conn, err := s.router.Route(methodName)
	if err != nil {
		return err
	}

	// TODO(renbou): timeouts for stream initiation and Recv/Sends
	outgoing, err := conn.BiDiStream(incoming.Context(), methodName)
	if err != nil {
		return err
	}

	// Always clean up the outgoing stream by explicitly closing it.
	defer outgoing.Close()

	i2oErrCh := make(chan proxyingError, 1)
	o2iErrCh := make(chan proxyingError, 1)

	go func() {
		i2oErrCh <- s.forwardIncomingToOutgoing(logger, incoming, outgoing)
	}()

	go func() {
		o2iErrCh <- s.forwardOutgoingToIncoming(logger, outgoing, incoming)
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

func (s *Server) forwardIncomingToOutgoing(logger grpcbridge.Logger, incoming grpc.ServerStream, outgoing *grpcadapter.BiDiStream) proxyingError {
	defer logger.Debug("ending forwarding incoming to outgoing")

	for {
		// Proto properly unmarshals any message into an empty one, keeping all the fields as protoimpl.UnknownFields.
		msg := new(emptypb.Empty)

		if err := incoming.RecvMsg(msg); err != nil {
			return proxyingError{incomingErr: err}
		}

		if err := outgoing.Send(incoming.Context(), msg); err != nil {
			return proxyingError{outgoingErr: err}
		}
	}
}

func (s *Server) forwardOutgoingToIncoming(logger grpcbridge.Logger, outgoing *grpcadapter.BiDiStream, incoming grpc.ServerStream) proxyingError {
	defer logger.Debug("ending forwarding outgoing to incoming")

	for {
		msg := new(emptypb.Empty)

		if err := outgoing.Recv(incoming.Context(), msg); err != nil {
			return proxyingError{outgoingErr: err}
		}

		if err := incoming.SendMsg(msg); err != nil {
			return proxyingError{incomingErr: err}
		}
	}
}
