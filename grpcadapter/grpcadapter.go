package grpcadapter

import (
	"context"
	"errors"
	"io"

	"github.com/renbou/grpcbridge/bridgedesc"
	"google.golang.org/protobuf/proto"
)

var ErrAlreadyDialed = errors.New("connection already dialed")

var (
	_ ClientConn   = (*AdaptedClientConn)(nil)
	_ ClientStream = (*AdaptedClientStream)(nil)
)

type ClientPool interface {
	Get(target string) (ClientConn, bool)
}

type ClientConn interface {
	Stream(ctx context.Context, method string) (ClientStream, error)
	Close()
}

type ClientStream interface {
	Send(context.Context, proto.Message) error
	Recv(context.Context, proto.Message) error
	CloseSend()
	Close()
}

type ServerStream interface {
	Send(context.Context, proto.Message) error
	Recv(context.Context, proto.Message) error
}

type ForwardS2C struct {
	Incoming ServerStream
	Outgoing ClientStream
	Method   *bridgedesc.Method
}

// ForwardServerToClient forwards an incoming ServerStream to an outgoing ClientStream via 2 concurrent goroutines.
// It either returns an error when the forwarding process fails, or nil when the outgoing stream returns an io.EOF on Recv.
func ForwardServerToClient(ctx context.Context, params ForwardS2C) error {
	i2oErrCh := make(chan forwardError, 1)
	o2iErrCh := make(chan forwardError, 1)

	// Close both goroutines when one fails.
	// However, we don't wait for them to actually exit,
	// because ServerStream operations can still block until the server handler returns.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		i2oErrCh <- forwardIncomingToOutgoing(ctx, params.Incoming, params.Outgoing, params.Method.Input)
	}()

	go func() {
		o2iErrCh <- forwardOutgoingToIncoming(ctx, params.Outgoing, params.Incoming, params.Method.Output)
	}()

	// Handle proxying errors and return them when needed.
	// Nothing here will deadlock thanks to the buffer of the channels,
	// and the outgoing channel is guaranteed to receive an error due to the context cancelation.
	for {
		select {
		case perr := <-i2oErrCh:
			if errors.Is(perr.incomingErr, io.EOF) || errors.Is(perr.outgoingErr, io.EOF) {
				// io.EOF can arrive either from incoming.Recv or outgoing.Send,
				// and in both cases we need to receive further responses via outgoing.Recv until it returns a proper error.
				// For this to work properly, forward a CloseSend to outgoing, even though io.EOF from outgoing.Send might've already done so.
				// Calling it here is safe since outgoing.Send isn't going to get called anymore anyway.
				params.Outgoing.CloseSend()
			} else if perr.incomingErr != nil {
				return perr.incomingErr
			} else {
				// perr.outgoingErr is guaranteed to be non-nil by forwardIncomingToOutgoing.
				return perr.outgoingErr
			}
		case perr := <-o2iErrCh:
			if errors.Is(perr.outgoingErr, io.EOF) {
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

// forwardError is a utility struct to distinguish errors received from server.Recv/Send and client.Recv/Send calls.
// This is mostly needed for properly handling io.EOF errors which might come from all sorts of calls.
type forwardError struct {
	incomingErr error
	outgoingErr error
}

func forwardIncomingToOutgoing(ctx context.Context, incoming ServerStream, outgoing ClientStream, message bridgedesc.Message) forwardError {
	msg := message.New()

	for {
		if err := incoming.Recv(ctx, msg); err != nil {
			return forwardError{incomingErr: err}
		}

		if err := outgoing.Send(ctx, msg); err != nil {
			return forwardError{outgoingErr: err}
		}

		proto.Reset(msg)
	}
}

func forwardOutgoingToIncoming(ctx context.Context, outgoing ClientStream, incoming ServerStream, message bridgedesc.Message) forwardError {
	msg := message.New()

	for {
		if err := outgoing.Recv(ctx, msg); err != nil {
			return forwardError{outgoingErr: err}
		}

		if err := incoming.Send(ctx, msg); err != nil {
			return forwardError{incomingErr: err}
		}

		proto.Reset(msg)
	}
}
