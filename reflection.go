package grpcbridge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/reflection"
	"github.com/renbou/grpcbridge/routing"
	"google.golang.org/grpc"
)

// RouterOption configures all the components needed to set up grpcbridge routing - gRPC reflection, client pools, logging.
type RouterOption interface {
	applyRouter(*routerOptions)
}

// ReflectionRouter provides the default [Router] implementation used by grpcbridge.
// It uses the [gRPC reflection Protocol] to retrieve Protobuf contracts of target gRPC services added via the [ReflectionRouter.Add] method,
// which launches a new [reflection.Resolver] to perform polling for contract updates, represented as [bridgedesc.Target] structures, in the background.
//
// Routing is performed using [routing.ServiceRouter] for gRPC requests based on the requested gRPC service name,
// and [routing.PatternRouter] for HTTP requests, supporting familiar pattern-based routing defined by Google's [HTTP to gRPC Transcoding spec].
//
// [grpcadapter.AdaptedClientPool] is used by the various components as a centralized gRPC client pool,
// providing support for features such as gRPC stream receiving/sending cancelation,
// waiting for client liveness and reconnections, and more.
//
// [gRPC reflection Protocol]: https://github.com/grpc/grpc/blob/master/doc/server-reflection.md
// [HTTP to gRPC Transcoding spec]: https://cloud.google.com/endpoints/docs/grpc/transcoding#adding_transcoding_mappings.
type ReflectionRouter struct {
	connpool        *grpcadapter.AdaptedClientPool
	resolverBuilder *reflection.ResolverBuilder
	patternRouter   *routing.PatternRouter
	serviceRouter   *routing.ServiceRouter

	// mu protects the router state.
	mu      sync.Mutex
	targets map[string]targetState
}

// NewReflectionRouter constructs a new [*ReflectionRouter] with the given options.
// When no options are provided, the respective components are initialized with their default options.
func NewReflectionRouter(opts ...RouterOption) *ReflectionRouter {
	options := defaultRouterOptions()

	for _, opt := range opts {
		opt.applyRouter(&options)
	}

	options.resolverOpts.Logger = options.common.logger

	connpool := grpcadapter.NewAdaptedClientPool(options.clientPoolOpts)
	resolverBuilder := reflection.NewResolverBuilder(connpool, options.resolverOpts)
	patternRouter := routing.NewPatternRouter(connpool, routing.PatternRouterOpts{Logger: options.common.logger})
	serviceRouter := routing.NewServiceRouter(connpool, routing.ServiceRouterOpts{Logger: options.common.logger})

	return &ReflectionRouter{
		connpool:        connpool,
		resolverBuilder: resolverBuilder,
		patternRouter:   patternRouter,
		serviceRouter:   serviceRouter,
		targets:         make(map[string]targetState),
	}
}

// Add adds a new target by an arbitrary name and its gRPC target to the router with the possibility to override the default options.
// It returns true if a new target was added, and false if the target was already present,
// in which case all the components are updated to use the new options.
// Unless a custom connection constructor was specified using [WithConnFunc],
// the target name syntax must adhere to standard gRPC rules defined in https://github.com/grpc/grpc/blob/master/doc/naming.md.
//
// When add returns, it means that the target was added to the router components,
// but no guarantees are made as to when the target can actually be returned by the router.
// For example, the initial resolution of the target's Protobuf contracts can fail,
// in which case the routers will have no information available regarding the target's routes.
//
// TODO(renbou): support option overriding.
// TODO(renbou): support modifications to existing targets.
// TODO(renbou): support using ServiceRouter for HTTP routing when simplified reflection resolving is enabled.
func (r *ReflectionRouter) Add(name string, target string, opts ...RouterOption) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.targets[name]; ok {
		return false, errors.New("ReflectionRouter currently doesn't support adding the same target twice")
	} else if len(opts) > 0 {
		return false, errors.New("ReflectionRouter currently doesn't support per-target option overrides")
	}

	poolController, err := r.connpool.New(name, target)
	if err != nil {
		return false, fmt.Errorf("creating new gRPC client for target %q: %w", target, err)
	}

	patternWatcher, err := r.patternRouter.Watch(name)
	if err != nil {
		return false, fmt.Errorf("failed to watch target %q with PatternRouter, this should never happen: %w", name, err)
	}

	serviceWatcher, err := r.serviceRouter.Watch(name)
	if err != nil {
		return false, fmt.Errorf("failed to watch target %q with ServiceRouter, this should never happen: %w", name, err)
	}

	watcher := &aggregateWatcher{watchers: []closableWatcher{patternWatcher, serviceWatcher}}
	resolver := r.resolverBuilder.Build(name, watcher)

	r.targets[name] = targetState{resolver: resolver, poolController: poolController, watcher: watcher}

	return true, nil
}

// Remove removes a target by its name, stopping the reflection polling and closing the gRPC client, returning true if the target was present.
// No more requests will be routed to the target after this call returns, meaning it will block until all the components acknowledge the removal.
func (r *ReflectionRouter) Remove(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.targets[name]; !ok {
		return false
	}

	target := r.targets[name]

	// Close the watchers first to avoid any errors due to closed clients and such.
	target.watcher.Close()

	// Then close the resolver to stop polling, after this no more dependencies on the pool client should be present.
	target.resolver.Close()

	// Close the pooled connection gracefully.
	target.poolController.Close()

	delete(r.targets, name)

	return true
}

// RouteHTTP uses the router's [routing.PatternRouter] instance to perform pattern-based routing for HTTP-originating requests,
// such as simple REST-like HTTP requests, WebSocket streams, and Server-Sent Event streams.
func (r *ReflectionRouter) RouteHTTP(req *http.Request) (grpcadapter.ClientConn, routing.HTTPRoute, error) {
	return r.patternRouter.RouteHTTP(req)
}

// RouteGRPC uses the router's [routing.ServiceRouter] instance to perform service-based routing for gRPC-like requests.
// This routing method is used for gRPC proxying, as well as gRPC-Web bridging of HTTP and WebSockets.
func (r *ReflectionRouter) RouteGRPC(ctx context.Context) (grpcadapter.ClientConn, routing.GRPCRoute, error) {
	return r.serviceRouter.RouteGRPC(ctx)
}

// WithDialOpts returns a RouterOption that sets the default gRPC dial options used by [grpcadapter.AdaptedClientPool] for establishing new connections.
// Multiple WithDialOpts calls can be chained together, and they will be aggregated into a single list of options.
func WithDialOpts(opts ...grpc.DialOption) RouterOption {
	return newFuncRouterOption(func(o *routerOptions) {
		o.clientPoolOpts.DefaultOpts = append(o.clientPoolOpts.DefaultOpts, opts...)
	})
}

// WithConnFunc returns a RouterOption that sets the function that will be used by [grpcadapter.AdaptedClientPool]
// for establishing new connections instead of [grpc.NewClient].
func WithConnFunc(f func(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error)) RouterOption {
	return newFuncRouterOption(func(o *routerOptions) {
		o.clientPoolOpts.NewClientFunc = f
	})
}

// WithReflectionPollInterval returns a RouterOption that sets the interval at which [reflection.Resolver]
// will poll target gRPC servers for proto changes, by default equal to 5 minutes.
func WithReflectionPollInterval(interval time.Duration) RouterOption {
	return newFuncRouterOption(func(o *routerOptions) {
		o.resolverOpts.PollInterval = interval
	})
}

// WithDisabledReflectionPolling returns a RouterOption that disables polling for proto changes,
// meaning that [reflection.Resolver] will retrieve the proto descriptors of target servers only once.
// For more granular control over when re-resolving happens, [reflection.ResolverBuilder] should be used to manually create resolvers.
func WithDisabledReflectionPolling() RouterOption {
	return newFuncRouterOption(func(o *routerOptions) {
		o.resolverOpts.PollManually = true
	})
}

type routerOptions struct {
	common         options
	clientPoolOpts grpcadapter.AdaptedClientPoolOpts
	resolverOpts   reflection.ResolverOpts
}

func defaultRouterOptions() routerOptions {
	return routerOptions{
		common: defaultOptions(),
	}
}

type funcRouterOption struct {
	f func(*routerOptions)
}

func (f *funcRouterOption) applyRouter(o *routerOptions) {
	f.f(o)
}

func newFuncRouterOption(f func(*routerOptions)) RouterOption {
	return &funcRouterOption{f: f}
}

type targetState struct {
	resolver       *reflection.Resolver
	poolController *grpcadapter.AdaptedClientPoolController
	watcher        closableWatcher
}

// closableWatcher is implemented by routing.ServiceRouterWatcher and routing.PatternRouterWatcher.
type closableWatcher interface {
	reflection.Watcher
	Close()
}

type aggregateWatcher struct {
	watchers []closableWatcher
}

func (a *aggregateWatcher) UpdateDesc(target *bridgedesc.Target) {
	for _, w := range a.watchers {
		w.UpdateDesc(target)
	}
}

func (a *aggregateWatcher) ReportError(err error) {
	for _, w := range a.watchers {
		w.ReportError(err)
	}
}

func (a *aggregateWatcher) Close() {
	for _, w := range a.watchers {
		w.Close()
	}
}
