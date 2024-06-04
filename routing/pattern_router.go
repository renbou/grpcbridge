package routing

import (
	"container/list"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	httprule "github.com/renbou/grpcbridge/internal/httprule/gwbased"
	"github.com/renbou/grpcbridge/internal/syncset"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PatternRouterOpts define all the optional settings which can be set for [PatternRouter].
type PatternRouterOpts struct {
	// Logs are discarded by default.
	Logger bridgelog.Logger
}

func (o PatternRouterOpts) withDefaults() PatternRouterOpts {
	if o.Logger == nil {
		o.Logger = bridgelog.Discard()
	}

	return o
}

// PatternRouter is a router meant for routing HTTP requests with non-gRPC URLs/contents.
// It uses pattern-based route matching like the one used in [gRPC-Gateway], but additionally supports dynamic routing updates
// via [PatternRouterWatcher], meant to be used with a description resolver such as the one in the [github.com/renbou/grpcbridge/reflection] package.
//
// Unlike gRPC-Gateway it doesn't support POST->GET fallbacks and X-HTTP-Method-Override,
// since such features can easily become a source of security issues for an unsuspecting developer.
// By the same logic, request paths aren't cleaned, i.e. multiple slashes, ./.. elements aren't removed.
//
// [gRPC-Gateway]: https://github.com/grpc-ecosystem/grpc-gateway
type PatternRouter struct {
	pool       grpcadapter.ClientPool
	logger     bridgelog.Logger
	routes     *mutablePatternRoutingTable
	watcherSet *syncset.SyncSet[string]
}

// NewPatternRouter initializes a new [PatternRouter] with the specified connection pool and options.
//
// The connection pool will be used to perform a simple retrieval of the connection to a target by its name.
// for more complex connection routing this router's [PatternRouter.RouteHTTP] can be wrapped to return a
// different connection based on the matched method and HTTP request parameters.
func NewPatternRouter(pool grpcadapter.ClientPool, opts PatternRouterOpts) *PatternRouter {
	opts = opts.withDefaults()

	return &PatternRouter{
		pool:       pool,
		logger:     opts.Logger.WithComponent("grpcbridge.routing"),
		routes:     newMutablePatternRoutingTable(),
		watcherSet: syncset.New[string](),
	}
}

// RouteHTTP routes the HTTP request based on its URL path and method
// using the target descriptions received via updates through [PatternRouterWatcher.UpdateDesc].
//
// Errors returned by RouteHTTP are gRPC status.Status errors with the code set accordingly.
// Currently, the NotFound, InvalidArgument, and Unavailable codes are returned.
// Additionally, it can return an error implementing interface { HTTPStatus() int } to set a custom status code, but it doesn't currently do so.
//
// Performance-wise it is notable that updates to the routing information don't block RouteHTTP, happening fully in the background.
func (pr *PatternRouter) RouteHTTP(r *http.Request) (grpcadapter.ClientConn, HTTPRoute, error) {
	// Try to follow the same steps as in https://github.com/grpc-ecosystem/grpc-gateway/blob/main/runtime/mux.go#L328 (ServeMux.ServeHTTP).
	// Specifically, use RawPath for pattern matching, since it will be properly decoded by the pattern itself.
	path := r.URL.RawPath
	if path == "" {
		path = r.URL.Path
	}

	if !strings.HasPrefix(path, "/") {
		return nil, HTTPRoute{}, status.Error(codes.InvalidArgument, http.StatusText(http.StatusBadRequest))
	}

	pathComponents := strings.Split(path[1:], "/")
	lastPathComponent := pathComponents[len(pathComponents)-1]
	matchComponents := make([]string, len(pathComponents))

	var routeErr error
	var matched bool
	var matchedRoute HTTPRoute

	pr.routes.iterate(r.Method, func(target *bridgedesc.Target, route *patternRoute) bool {
		var verb string
		patternVerb := route.pattern.Verb()

		verbIdx := -1
		if patternVerb != "" && strings.HasSuffix(lastPathComponent, ":"+patternVerb) {
			verbIdx = len(lastPathComponent) - len(patternVerb) - 1
		}

		// path segments consisting only of verbs aren't allowed
		if verbIdx == 0 {
			routeErr = status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
			return false
		}

		matchComponents = matchComponents[:len(pathComponents)]
		copy(matchComponents, pathComponents)

		if verbIdx > 0 {
			matchComponents[len(matchComponents)-1], verb = lastPathComponent[:verbIdx], lastPathComponent[verbIdx+1:]
		}

		// Perform unescaping as specified for gRPC transcoding in https://github.com/googleapis/googleapis/blob/e0677a395947c2f3f3411d7202a6868a7b069a41/google/api/http.proto#L295.
		params, err := route.pattern.MatchAndEscape(matchComponents, verb, runtime.UnescapingModeAllExceptReserved)
		if err != nil {
			var mse runtime.MalformedSequenceError
			if ok := errors.As(err, &mse); ok {
				routeErr = status.Error(codes.InvalidArgument, err.Error())
				return false
			}

			// Ignore runtime.ErrNotMatch
			return true
		}

		// Avoid returning empty maps when not needed.
		if len(params) == 0 {
			params = nil
		}

		// Found match
		matched = true
		matchedRoute = HTTPRoute{
			Target:     target,
			Service:    route.service,
			Method:     route.method,
			Binding:    route.binding,
			PathParams: params,
		}
		return false
	})

	if routeErr != nil {
		return nil, HTTPRoute{}, routeErr
	} else if !matched {
		return nil, HTTPRoute{}, status.Error(codes.NotFound, http.StatusText(http.StatusNotFound))
	}

	conn, ok := pr.pool.Get(matchedRoute.Target.Name)
	if !ok {
		return nil, HTTPRoute{}, status.Errorf(codes.Unavailable, "no connection available to target %q", matchedRoute.Target.Name)
	}

	return conn, matchedRoute, nil
}

// Watch starts watching the specified target for description changes.
// It returns a [*PatternRouterWatcher] through which new updates for this target can be applied.
//
// It is an error to try Watch()ing the same target multiple times on a single PatternRouter instance,
// the previous [PatternRouterWatcher] must be explicitly closed before launching a new one.
// Instead of trying to synchronize such procedures, however, it's better to have a properly defined lifecycle
// for each possible target, with clear logic about when it gets added or removed to/from all the components of a bridge.
func (pr *PatternRouter) Watch(target string) (*PatternRouterWatcher, error) {
	if pr.watcherSet.Add(target) {
		return &PatternRouterWatcher{pr: pr, target: target, logger: pr.logger.With("target", target)}, nil
	}

	return nil, ErrAlreadyWatching
}

// PatternRouterWatcher is a description update watcher created for a specific target in the context of a [PatternRouter] instance.
// New PatternRouterWatchers are created through [PatternRouter.Watch].
type PatternRouterWatcher struct {
	pr     *PatternRouter
	logger bridgelog.Logger
	target string
	closed atomic.Bool
}

// UpdateDesc updates the description of the target this watcher is watching.
// After the watcher is Close()d, UpdateDesc becomes a no-op, to avoid writing meaningless updates to the router.
// Note that desc.Name must match the target this watcher was created for, otherwise the update will be ignored.
//
// Updates to the routing information are made without any locking,
// instead replacing the currently present info with the updated one using an atomic pointer.
//
// UpdateDesc returns only when the routing state has been completely updated on the router,
// which should be used to synchronize the target description update polling/watching logic.
func (prw *PatternRouterWatcher) UpdateDesc(desc *bridgedesc.Target) {
	if prw.closed.Load() {
		return
	}

	if desc.Name != prw.target {
		// use PatternRouter logger without the "target" field
		prw.pr.logger.Error("PatternRouterWatcher got update for different target, will ignore", "watcher_target", prw.target, "update_target", desc.Name)
		return
	}

	routes := buildPatternRoutes(desc, prw.logger)
	prw.pr.routes.addTarget(desc, routes)
}

// ReportError is currently a no-op, present simply to implement the Watcher interface
// of the grpcbridge description resolvers, such as the one in [github.com/renbou/grpcbridge/reflection].
func (prw *PatternRouterWatcher) ReportError(error) {}

// Close closes the watcher, preventing further updates from being applied to the router through it.
// It is an error to call Close() multiple times on the same watcher, and doing so will result in a panic.
func (prw *PatternRouterWatcher) Close() {
	if !prw.closed.CompareAndSwap(false, true) {
		panic("grpcbridge: PatternRouterWatcher.Close() called multiple times")
	}

	// Fully remove the target's routes, only then mark the watcher as closed.
	prw.pr.routes.removeTarget(prw.target)
	prw.pr.watcherSet.Remove(prw.target)
}

func buildPatternRoutes(desc *bridgedesc.Target, logger bridgelog.Logger) map[string][]patternRoute {
	builder := newPatternRouteBuilder()

	for svcIdx := range desc.Services {
		svc := &desc.Services[svcIdx]
		methods := desc.Services[svcIdx].Methods // avoid copying the whole desc structures in loop
		for methodIdx := range methods {
			method := &methods[methodIdx]

			if len(method.Bindings) < 1 {
				if routeErr := builder.addDefault(svc, method); routeErr != nil {
					logger.Error("failed to add default HTTP binding for gRPC method with no defined bindings",
						"service", svc.Name, "method", method.RPCName,
						"error", routeErr,
					)
				} else {
					logger.Debug("added default HTTP binding for gRPC method", "service", svc.Name, "method", method.RPCName)
				}
				continue
			}

			for bindingIdx := range method.Bindings {
				binding := &method.Bindings[bindingIdx]
				if routeErr := builder.addBinding(svc, method, binding); routeErr != nil {
					logger.Error("failed to add HTTP binding for gRPC method",
						"service", svc.Name, "method", method.RPCName,
						"binding.method", binding.HTTPMethod, "binding.pattern", binding.Pattern,
						"error", routeErr,
					)
				} else {
					logger.Debug("added HTTP binding for gRPC method",
						"service", svc.Name, "method", method.RPCName,
						"binding.method", binding.HTTPMethod, "binding.pattern", binding.Pattern,
					)
				}
			}
		}
	}

	return builder.routes
}

type patternRoute struct {
	service *bridgedesc.Service
	method  *bridgedesc.Method
	binding *bridgedesc.Binding
	pattern runtime.Pattern
}

func buildPattern(route string) (runtime.Pattern, error) {
	compiler, err := httprule.Parse(route)
	if err != nil {
		return runtime.Pattern{}, fmt.Errorf("parsing route: %w", err)
	}

	tp := compiler.Compile()

	pattern, routeErr := runtime.NewPattern(tp.Version, tp.OpCodes, tp.Pool, tp.Verb)
	if routeErr != nil {
		return runtime.Pattern{}, fmt.Errorf("creating route pattern matcher: %w", routeErr)
	}

	return pattern, nil
}

// patternRouteBuilder is a helper structure used for building the routing table for a single target.
type patternRouteBuilder struct {
	routes map[string][]patternRoute // http method -> routes
}

func newPatternRouteBuilder() patternRouteBuilder {
	return patternRouteBuilder{
		routes: make(map[string][]patternRoute),
	}
}

func (rb *patternRouteBuilder) addBinding(s *bridgedesc.Service, m *bridgedesc.Method, b *bridgedesc.Binding) error {
	pr, err := buildPattern(b.Pattern)
	if err != nil {
		return fmt.Errorf("building pattern for %s: %w", b.HTTPMethod, err)
	}

	rb.routes[b.HTTPMethod] = append(rb.routes[b.HTTPMethod], patternRoute{
		service: s,
		method:  m,
		binding: b,
		pattern: pr,
	})
	return nil
}

func (rb *patternRouteBuilder) addDefault(s *bridgedesc.Service, m *bridgedesc.Method) error {
	// Default gRPC form, as specified in https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests.
	return rb.addBinding(s, m, bridgedesc.DefaultBinding(m))
}

// targetPatternRoutes define the routes for a single target+method combination.
type targetPatternRoutes struct {
	target *bridgedesc.Target
	routes []patternRoute
}

type methodPatternRoutes struct {
	method string
	link   *list.Element // Value of type targetPatternRoutes
}

// mutablePatternRoutingTable is a pattern-based routing table which can be modified with all operations protected by a mutex.
type mutablePatternRoutingTable struct {
	// protects all the routing state, including the pointer to the static table,
	// which must be updated while the mutex is held to avoid overwriting new state with old state.
	mu          sync.Mutex
	static      atomic.Pointer[staticPatternRoutingTable]
	routes      map[string]*list.List            // http method -> linked list of targetPatternRoutes
	targetLinks map[string][]methodPatternRoutes // target -> list of elements to be modified
}

func newMutablePatternRoutingTable() *mutablePatternRoutingTable {
	mt := &mutablePatternRoutingTable{
		routes:      make(map[string]*list.List),
		targetLinks: make(map[string][]methodPatternRoutes),
	}

	mt.static.Store(&staticPatternRoutingTable{})

	return mt
}

// addTargets adds or updates the routes of a target and updates the static pointer.
func (mt *mutablePatternRoutingTable) addTarget(target *bridgedesc.Target, routes map[string][]patternRoute) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	newMethodLinks := make([]methodPatternRoutes, 0, len(routes))

	// Update existing elements without recreating them.
	for _, link := range mt.targetLinks[target.Name] {
		patternRoutes, ok := routes[link.method]
		if !ok {
			// delete existing link, no more routes for this method
			mt.removeRoute(link.method, link.link)
			continue
		}

		// set new pattern routes via the link & mark link as in-use
		link.link.Value = targetPatternRoutes{target: target, routes: patternRoutes}
		newMethodLinks = append(newMethodLinks, link)

		// mark method as handled, we don't need to re-add it
		delete(routes, link.method)
	}

	// Add routes for new methods.
	for method, patternRoutes := range routes {
		link := mt.addRoute(method, targetPatternRoutes{target: target, routes: patternRoutes})
		newMethodLinks = append(newMethodLinks, methodPatternRoutes{method: method, link: link})
	}

	mt.targetLinks[target.Name] = newMethodLinks

	mt.static.Store(mt.commit())
}

// removeTarget removes all routes of a target and updates the static pointer.
func (mt *mutablePatternRoutingTable) removeTarget(target string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	for _, link := range mt.targetLinks[target] {
		mt.removeRoute(link.method, link.link)
	}

	delete(mt.targetLinks, target)

	mt.static.Store(mt.commit())
}

func (mt *mutablePatternRoutingTable) removeRoute(method string, link *list.Element) {
	lst := mt.routes[method]
	lst.Remove(link)
	if lst.Len() == 0 {
		delete(mt.routes, method)
	}
}

func (mt *mutablePatternRoutingTable) addRoute(method string, route targetPatternRoutes) *list.Element {
	lst, ok := mt.routes[method]
	if !ok {
		lst = list.New()
		mt.routes[method] = lst
	}

	return lst.PushBack(route)
}

func (mt *mutablePatternRoutingTable) commit() *staticPatternRoutingTable {
	routes := make(map[string]*list.List, len(mt.routes))
	for method, list := range mt.routes {
		routes[method] = cloneLinkedList(list)
	}

	return &staticPatternRoutingTable{routes: routes}
}

func (mt *mutablePatternRoutingTable) iterate(method string, fn func(target *bridgedesc.Target, route *patternRoute) bool) {
	mt.static.Load().iterate(method, fn)
}

// staticPatternRoutingTable is a pattern-based routing table which can only be read.
// it is created by mutablePatternRoutingTable when modifications occur.
type staticPatternRoutingTable struct {
	routes map[string]*list.List // http method -> linked list of targetPatternRoutes
}

func (st *staticPatternRoutingTable) iterate(method string, fn func(target *bridgedesc.Target, route *patternRoute) bool) {
	list, ok := st.routes[method]
	if !ok {
		return
	}

	for e := list.Front(); e != nil; e = e.Next() {
		pr := e.Value.(targetPatternRoutes)
		for i := range pr.routes {
			if !fn(pr.target, &pr.routes[i]) {
				return
			}
		}
	}
}

func cloneLinkedList(l *list.List) *list.List {
	cp := list.New()
	for e := l.Front(); e != nil; e = e.Next() {
		cp.PushBack(e.Value)
	}
	return cp
}
