package grpcadapter

import (
	"slices"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
)

type AdaptedClientPoolOpts struct {
	// NewClientFunc can be used to specify an alternative connection constructor to be used instead of [grpc.NewClient].
	NewClientFunc func(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error)

	// DefaultOpts specify the default options to be used when creating a new client, by default no opts are used.
	DefaultOpts []grpc.DialOption
}

func (o AdaptedClientPoolOpts) withDefaults() AdaptedClientPoolOpts {
	if o.NewClientFunc == nil {
		o.NewClientFunc = grpc.NewClient
	}

	return o
}

type AdaptedClientPool struct {
	opts  AdaptedClientPoolOpts
	conns sync.Map // string -> *DialedPoolController
}

func NewAdaptedClientPool(opts AdaptedClientPoolOpts) *AdaptedClientPool {
	return &AdaptedClientPool{opts: opts.withDefaults()}
}

// New creates a new [*AdaptedClientConn] using [grpc.NewClient] or the NewClientFunc specified in [AdaptedClientPoolOpts]
// using the merged set of default options and the options passed to New.
// It returns a [*AdaptedClientPoolController] which can be used to close the connection and remove it from the pool.
//
// It is an error to call New() with the same targetName multiple times on a single DialedPool instance,
// the previous [*AdaptedClientPoolController] must be explicitly used to close the connection before dialing a new one.
// Instead of trying to synchronize such procedures, however, it's better to have a properly defined lifecycle
// for each possible target, with clear logic about when it gets added or removed to/from all the components of a bridge.
func (p *AdaptedClientPool) New(targetName, dialTarget string, opts ...grpc.DialOption) (*AdaptedClientPoolController, error) {
	controllerAny, loaded := p.conns.LoadOrStore(targetName, new(AdaptedClientPoolController))
	if loaded {
		return nil, ErrAlreadyDialed
	}

	conn, err := p.opts.NewClientFunc(dialTarget, slices.Concat(p.opts.DefaultOpts, opts)...)
	if err != nil {
		return nil, err
	}

	controller := controllerAny.(*AdaptedClientPoolController)
	controller.pool = p
	controller.target = targetName
	controller.client = AdaptClient(conn)

	return controller, nil
}

func (p *AdaptedClientPool) Get(target string) (ClientConn, bool) {
	controller, ok := p.conns.Load(target)
	if !ok {
		return nil, false
	}

	return controller.(*AdaptedClientPoolController).client, true
}

// AdaptedClientPoolController wraps an AdaptedClientConn created using a DialedPool,
// providing methods to control the AdaptedClientConn's own and pool lifecycle.
type AdaptedClientPoolController struct {
	pool *AdaptedClientPool

	target string
	client ClientConn
	closed atomic.Bool
}

// Close removes this connection from the pool and closes it.
// Calling close multiple times or concurrently will intentionally result in a panic to avoid bugs.
// TODO(renbou): implement graceful connection closure.
func (pw *AdaptedClientPoolController) Close() {
	if pw.closed.Swap(true) {
		panic("grpcbridge: pooled client conn closed multiple times")
	}

	pw.pool.conns.Delete(pw.target)
	pw.client.Close()
}
