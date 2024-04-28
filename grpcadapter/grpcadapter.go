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

// ForwardServerToClient forwards an incoming ServerStream to an outgoing ClientStream.
// It either returns an error when the forwarding process fails, or nil when the outgoing stream returns an io.EOF on Recv.
//
// ForwardServerToClient doesn't wait for its forwarding goroutines to exit, since there might not be an easy or reliable
// way to implement context support for all the incoming/outgoing stream operations.
// As such, the caller must properly close any of the resources needed to unblock any possible Send/Recv operations to avoid goroutine leakage.
func ForwardServerToClient(ctx context.Context, params ForwardS2C) error {
	i2oErrCh := make(chan forwardError, 1)
	o2iErrCh := make(chan forwardError, 1)

	// Close forwarding in both directions when one fails.
	// However, we don't wait for them to actually exit,
	// because ServerStream operations can still block until the server handler returns.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Launch the client-to-server forwarder only when the method expects a client stream,
	// otherwise its better to actually read the incoming request before interacting with the server,
	// to make sure we aren't wasting any resources on something that is guaranteed to fail.
	if params.Method.ClientStreaming {
		go func() {
			i2oErrCh <- forwardIncomingToOutgoing(ctx, &params)
		}()
	} else {
		if err := forwardRequest(ctx, &params); err != nil {
			return err
		}

		params.Outgoing.CloseSend()
	}

	// Server-to-client forwarder must always be run, because the server might respond at any time,
	// not only after the client stream has finished, e.g. close a stream due to authentication errors.
	go func() {
		o2iErrCh <- forwardOutgoingToIncoming(ctx, &params)
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

func forwardRequest(ctx context.Context, params *ForwardS2C) error {
	msg := params.Method.Input.New()

	if err := params.Incoming.Recv(ctx, msg); err != nil {
		return err
	}

	if err := params.Outgoing.Send(ctx, msg); err != nil && errors.Is(err, io.EOF) {
		// nil returned here to continue the process in ForwardServerToClient,
		// the actual status will be returned by forwardOutgoingToIncoming on Recv().
		return nil
	} else {
		return err // we only care about the case where err == io.EOF, nil and other errors are handled normally.
	}
}

// forwardError is a utility struct to distinguish errors received from server.Recv/Send and client.Recv/Send calls.
// This is mostly needed for properly handling io.EOF errors which might come from all sorts of calls.
type forwardError struct {
	incomingErr error
	outgoingErr error
}

func forwardIncomingToOutgoing(ctx context.Context, params *ForwardS2C) forwardError {
	for {
		// It is not safe to reuse a Proto message after Outgoing.Send().
		msg := params.Method.Input.New()

		if err := params.Incoming.Recv(ctx, msg); err != nil {
			return forwardError{incomingErr: err}
		}

		if err := params.Outgoing.Send(ctx, msg); err != nil {
			return forwardError{outgoingErr: err}
		}
	}
}

func forwardOutgoingToIncoming(ctx context.Context, params *ForwardS2C) forwardError {
	for {
		// It is not safe to reuse a Proto message after Incoming.Send().
		msg := params.Method.Output.New()

		if err := params.Outgoing.Recv(ctx, msg); err != nil {
			return forwardError{outgoingErr: err}
		}

		if err := params.Incoming.Send(ctx, msg); err != nil {
			return forwardError{incomingErr: err}
		}
	}
}
