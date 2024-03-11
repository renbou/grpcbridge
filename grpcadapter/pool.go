package grpcadapter

import (
	"context"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
)

type ClientConnPool struct {
	mu     sync.Mutex
	dialer func(context.Context, string) (*grpc.ClientConn, error)
	conns  map[string]*ClientConnPoolController
}

func NewClientConnPool(dialer func(context.Context, string) (*grpc.ClientConn, error)) *ClientConnPool {
	return &ClientConnPool{
		dialer: dialer,
		conns:  make(map[string]*ClientConnPoolController),
	}
}

// Build creates a new ClientConn and adds it to the pool without waiting for its dial to go through.
// Calling build using the same name without closing the previous connection will return the existing connection.
func (p *ClientConnPool) Build(ctx context.Context, name, target string) *ClientConnPoolController {
	p.mu.Lock()
	defer p.mu.Unlock()

	conn, ok := p.conns[name]
	if ok {
		return conn
	}

	unwrapped := NewClientConn(func() (*grpc.ClientConn, error) {
		return p.dialer(ctx, target)
	})

	conn = &ClientConnPoolController{
		pool: p,
		name: name,
		cc:   unwrapped,
	}
	p.conns[name] = conn

	return conn
}

func (p *ClientConnPool) Get(name string) *ClientConn {
	p.mu.Lock()
	defer p.mu.Unlock()

	controller, ok := p.conns[name]
	if !ok {
		return nil
	}

	return controller.cc
}

func (p *ClientConnPool) remove(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.conns, name)
}

// ClientConnPoolController wraps a ClientConn created using a ClientConnPool,
// providing methods to control the ClientConn's own and pool lifecycle.
type ClientConnPoolController struct {
	pool *ClientConnPool

	name   string
	cc     *ClientConn
	closed atomic.Bool
}

// Close removes this connection from the pool and closes it.
// Calling close multiple times or concurrently will intentionally result in a panic to avoid bugs.
func (pw *ClientConnPoolController) Close() {
	if pw.closed.Swap(true) {
		panic("grpcbridge: pooled client conn closed multiple times")
	}

	pw.pool.remove(pw.name)
	pw.cc.Close()
}
