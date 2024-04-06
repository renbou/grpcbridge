package grpcadapter

import (
	"context"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
)

type DialedPool struct {
	mu     sync.Mutex
	dialer func(context.Context, string) (*grpc.ClientConn, error)
	conns  map[string]*DialedPoolController
}

func NewDialedPool(dialer func(context.Context, string) (*grpc.ClientConn, error)) *DialedPool {
	return &DialedPool{
		dialer: dialer,
		conns:  make(map[string]*DialedPoolController),
	}
}

// Build creates a new AdaptedClientConn and adds it to the pool without waiting for its dial to go through.
// Calling build using the same target name without closing the previous connection will return the existing connection.
func (p *DialedPool) Build(ctx context.Context, targetName, dialTarget string) *DialedPoolController {
	p.mu.Lock()
	defer p.mu.Unlock()

	conn, ok := p.conns[targetName]
	if ok {
		return conn
	}

	unwrapped := AdaptedDial(func() (*grpc.ClientConn, error) {
		return p.dialer(ctx, dialTarget)
	})

	conn = &DialedPoolController{
		pool: p,
		name: targetName,
		conn: WrapClientConn(unwrapped),
	}
	p.conns[targetName] = conn

	return conn
}

func (p *DialedPool) Get(target string) (ClientConn, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	controller, ok := p.conns[target]
	if !ok {
		return nil, false
	}

	return controller.conn, true
}

func (p *DialedPool) remove(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.conns, name)
}

// DialedPoolController wraps an AdaptedClientConn created using a DialedPool,
// providing methods to control the AdaptedClientConn's own and pool lifecycle.
type DialedPoolController struct {
	pool *DialedPool

	name   string
	conn   ClientConn
	closed atomic.Bool
}

// Close removes this connection from the pool and closes it.
// Calling close multiple times or concurrently will intentionally result in a panic to avoid bugs.
func (pw *DialedPoolController) Close() {
	if pw.closed.Swap(true) {
		panic("grpcbridge: pooled client conn closed multiple times")
	}

	pw.pool.remove(pw.name)
	pw.conn.Close()
}
