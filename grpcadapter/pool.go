package grpcadapter

import (
	"context"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
)

type DialedPool struct {
	dialer func(context.Context, string) (*grpc.ClientConn, error)
	conns  sync.Map // string -> *DialedPoolController
}

func NewDialedPool(dialer func(context.Context, string) (*grpc.ClientConn, error)) *DialedPool {
	return &DialedPool{dialer: dialer}
}

// Dial creates a new [*AdaptedClientConn] and adds it to the pool without waiting for its dial to go through.
// It returns a [*DialedPoolController] which can be used to close the connection and remove it from the pool.
//
// It is an error to try Dial()ing the same targetName multiple times on a single DialedPool instance,
// the previous [*DialedPoolController] must be explicitly used to close the connection before dialing a new one.
// Instead of trying to synchronize such procedures, however, it's better to have a properly defined lifecycle
// for each possible target, with clear logic about when it gets added or removed to/from all the components of a bridge.
func (p *DialedPool) Dial(ctx context.Context, targetName, dialTarget string) (*DialedPoolController, error) {
	controllerAny, loaded := p.conns.LoadOrStore(targetName, new(DialedPoolController))
	if loaded {
		return nil, ErrAlreadyDialed
	}

	conn := AdaptedDial(func() (*grpc.ClientConn, error) {
		return p.dialer(ctx, dialTarget)
	})

	controller := controllerAny.(*DialedPoolController)
	controller.pool = p
	controller.target = targetName
	controller.conn = conn

	return controller, nil
}

func (p *DialedPool) Get(target string) (ClientConn, bool) {
	controller, ok := p.conns.Load(target)
	if !ok {
		return nil, false
	}

	return controller.(*DialedPoolController).conn, true
}

// DialedPoolController wraps an AdaptedClientConn created using a DialedPool,
// providing methods to control the AdaptedClientConn's own and pool lifecycle.
type DialedPoolController struct {
	pool *DialedPool

	target string
	conn   ClientConn
	closed atomic.Bool
}

// Close removes this connection from the pool and closes it.
// Calling close multiple times or concurrently will intentionally result in a panic to avoid bugs.
func (pw *DialedPoolController) Close() {
	if pw.closed.Swap(true) {
		panic("grpcbridge: pooled client conn closed multiple times")
	}

	pw.pool.conns.Delete(pw.target)
	pw.conn.Close()
}
