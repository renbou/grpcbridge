package grpcadapter

import (
	"context"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/status"
)

type adaptedClientState struct {
	conn *grpc.ClientConn
	err  error
}

type AdaptedClientConn struct {
	state atomic.Pointer[adaptedClientState]
}

// AdaptClient wraps an existing gRPC client with an adapter implementing [ClientConn].
// The returned client is instantly ready to be used, and will be valid until [*AdaptedClientConn.Close] is called,
// after which any stream initiations via the client will return an error and the underlying gRPC client will be closed, too.
func AdaptClient(conn *grpc.ClientConn) *AdaptedClientConn {
	state := &adaptedClientState{
		conn: conn,
	}

	adapted := new(AdaptedClientConn)
	adapted.state.Store(state)

	return adapted
}

// Close marks this connection as closed and closes the underlying gRPC client.
// Calling Close concurrently won't cause issues, but why would you want to even do it...
func (cc *AdaptedClientConn) Close() {
	state := cc.state.Load()
	if state.conn == nil {
		return
	}

	_ = state.conn.Close() // doesn't return any meaningful errors

	cc.state.Store(&adaptedClientState{
		conn: nil,
		err:  status.Error(codes.Unavailable, "grpcbridge: connection closed"),
	})
}

func (cc *AdaptedClientConn) Stream(ctx context.Context, method string) (ClientStream, error) {
	conn, err := cc.getConn(ctx)
	if err != nil {
		return nil, err
	}

	// Create new context for the whole stream operation, and use the passed context only for the actual initialization.
	streamCtx, cancel := context.WithCancel(context.Background())
	wrapped := &AdaptedClientStream{
		// guaranteed to be called by Recv/Send on failure, otherwise needs to be called by caller.
		// we initialize it here because the initial NewStream() can also fail and execute Close().
		closeFunc: sync.OnceFunc(cancel),
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
	state := cc.state.Load()

	if state.conn != nil {
		// Try waiting for the connection to recover if it has failed.
		// This helps to avoid returning errors to client requests and gRPC reflection resolver
		// when the service has just started or when some random error occurs.
		// After waiting still try to use the connection to at least get a readable error describing the failure.
		cc.waitForReady(ctx, state.conn)
	}

	return state.conn, state.err
}

func (*AdaptedClientConn) waitForReady(ctx context.Context, conn *grpc.ClientConn) {
	// No point in wasting all the available time.
	// TODO(renbou): Add a "ConnectTimeout" setting to AdaptedClientConn and pool to have a default here when no timeout is set.
	ctx, cancel := ctxWithHalvedDeadline(ctx)
	defer cancel()

	connState := conn.GetState()

	// State will be IDLE first if something happened to the connection.
	// gRPC won't attempt a reconnect unless something happens,
	// like this manual call telling gRPC "hey, we will need a connection here!".
	if connState == connectivity.Idle {
		conn.Connect()
	}

	for connState != connectivity.Ready {
		if !conn.WaitForStateChange(ctx, connState) {
			return
		}

		connState = conn.GetState()
	}
}
