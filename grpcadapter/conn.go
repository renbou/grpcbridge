package grpcadapter

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ClientConn struct {
	mu sync.Mutex

	// dialed & closed synchronize the dialing/closing procedures which happen in different goroutines.
	dialed bool
	closed bool

	// dialedCh is closed to notify clients that the connection has been dialed,
	// and provides an additional synchronization mechanism where the mutex isn't needed.
	dialedCh chan struct{}

	conn *grpc.ClientConn
	err  error
}

// NewClientConn calls dial in a separate goroutine,
// and returns a ClientConn that will be ready when the dial completes.
// Using the ClientConn prior to the dial completing is valid, but any calls will return an Unavailable status error.
func NewClientConn(dial func() (*grpc.ClientConn, error)) *ClientConn {
	cc := &ClientConn{
		err:      status.Error(codes.Unavailable, "grpcbridge: connection not yet dialed"),
		dialedCh: make(chan struct{}),
	}

	go func() {
		conn, err := dial()

		// critical section to be observed in close()
		cc.mu.Lock()
		defer cc.mu.Unlock()

		// closed before dial() has completed, need to close the connection now
		if cc.closed {
			cc.applyClose()
		} else {
			cc.conn = conn
			cc.err = err
		}

		cc.dialed = true
		close(cc.dialedCh)
	}()

	return cc
}

// Close requests a close of this connection.
// If the connection has not yet been dialed, it will be closed when the dial completes.
func (cc *ClientConn) Close() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// avoid errors when getting called twice
	if cc.closed {
		return
	}

	cc.closed = true

	// if this is true, then close() has been called AFTER dial() completed, connection is properly closed;
	// otherwise, close() has been called before dial() completed, and the connection will be closed in dial().
	if cc.dialed {
		cc.applyClose()
	}
}

func (cc *ClientConn) BiDiStream(ctx context.Context, method string) (*BiDiStream, error) {
	conn, err := cc.getConn(ctx)
	if err != nil {
		return nil, err
	}

	// Create new context for the whole stream operation, and use the passed context only for the actual initialization.
	streamCtx, cancel := context.WithCancel(context.Background())
	wrapped := &BiDiStream{
		closeFunc: sync.OnceFunc(cancel), // guaranteed to be called by Recv/Send on failure, otherwise needs to be called by caller
	}

	err = wrapped.withCtx(ctx, func() error {
		wrapped.stream, err = conn.NewStream(streamCtx, &grpc.StreamDesc{ClientStreams: true, ServerStreams: true}, method)
		return err
	})
	if err != nil {
		stErr := status.Convert(err)
		return nil, status.Errorf(stErr.Code(), "initiating duplex stream %q: %s", method, stErr.Message())
	}

	return wrapped, nil
}

func (cc *ClientConn) getConn(ctx context.Context) (*grpc.ClientConn, error) {
	select {
	case <-ctx.Done():
		return nil, ctxRPCErr(ctx.Err())
	case <-cc.dialedCh:
	}

	return cc.conn, cc.err
}

// close needs to be called while cc.mu is held.
func (cc *ClientConn) applyClose() {
	// TODO: does this need logging? it only returns an error if called twice...
	_ = cc.conn.Close()

	// Clear conn and update error to avoid accidental use of connection after it has been closed.
	cc.conn = nil
	cc.err = status.Error(codes.Unavailable, "grpcbridge: connection closed")
}
