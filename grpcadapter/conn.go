package grpcadapter

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
)

type AdaptedClientConn struct {
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

// AdaptedDial calls dial in a separate goroutine,
// and returns an AdaptedClientConn that will be ready when the dial completes.
// Using the AdaptedClientConn prior to the dial completing is valid, but any calls will return an Unavailable status error.
func AdaptedDial(dial func() (*grpc.ClientConn, error)) *AdaptedClientConn {
	cc := &AdaptedClientConn{
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
func (cc *AdaptedClientConn) Close() {
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

func (cc *AdaptedClientConn) BiDiStream(ctx context.Context, method string) (*AdaptedBiDiStream, error) {
	conn, err := cc.getConn(ctx)
	if err != nil {
		return nil, err
	}

	// Create new context for the whole stream operation, and use the passed context only for the actual initialization.
	streamCtx, cancel := context.WithCancel(context.Background())
	wrapped := &AdaptedBiDiStream{
		closeFunc: sync.OnceFunc(cancel), // guaranteed to be called by Recv/Send on failure, otherwise needs to be called by caller
	}

	err = wrapped.withCtx(ctx, func() error {
		var newErr error
		wrapped.stream, newErr = conn.NewStream(streamCtx, &grpc.StreamDesc{ClientStreams: true, ServerStreams: true}, method)
		return newErr
	})
	if err != nil {
		stErr := status.Convert(err)
		return nil, status.Errorf(stErr.Code(), "initiating duplex stream %q: %s", method, stErr.Message())
	}

	return wrapped, nil
}

func (cc *AdaptedClientConn) getConn(ctx context.Context) (*grpc.ClientConn, error) {
	select {
	case <-ctx.Done():
		return nil, ctxRPCErr(ctx.Err())
	case <-cc.dialedCh:
	}

	if cc.conn == nil {
		return nil, cc.err
	}

	// Try waiting for the connection to recover if it has failed.
	// This helps to avoid returning errors to client requests and gRPC reflection resolver
	// when the service has just started or when some random error occurs.
	// After waiting still try to use the connection to at least get a readable error describing the failure.
	cc.waitForReady(ctx)

	return cc.conn, cc.err
}

func (cc *AdaptedClientConn) waitForReady(ctx context.Context) {
	// No point in wasting all the available time.
	// TODO(renbou): Add a "ConnectTimeout" setting to AdaptedClientConn and pool to have a default here when no timeout is set.
	ctx, cancel := ctxWithHalvedDeadline(ctx)
	defer cancel()

	state := cc.conn.GetState()

	// State will be IDLE first if something happened to the connection.
	// gRPC won't attempt a reconnect unless something happens,
	// like this manual call telling gRPC "hey, we will need a connection here!".
	if state == connectivity.Idle {
		cc.conn.Connect()
	}

	for state != connectivity.Ready {
		if !cc.conn.WaitForStateChange(ctx, state) {
			return
		}

		state = cc.conn.GetState()
	}
}

// close needs to be called while cc.mu is held.
func (cc *AdaptedClientConn) applyClose() {
	_ = cc.conn.Close() // doesn't return any meaningful errors

	// Clear conn and update error to avoid accidental use of connection after it has been closed.
	cc.conn = nil
	cc.err = status.Error(codes.Unavailable, "grpcbridge: connection closed")
}
