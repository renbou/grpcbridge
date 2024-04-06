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
	"github.com/renbou/grpcbridge/internal/countmap"
	"github.com/renbou/grpcbridge/internal/httprule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HTTPRoute struct {
	Binding    *bridgedesc.Binding
	PathParams map[string]string
}

type PatternRouterOpts struct {
	Logger bridgelog.Logger
}

func (o PatternRouterOpts) withDefaults() PatternRouterOpts {
	if o.Logger == nil {
		o.Logger = bridgelog.Discard()
	}

	return o
}

type PatternRouter struct {
	pool           ConnPool
	logger         bridgelog.Logger
	routes         *mutablePatternRoutingTable
	watcherCounter *countmap.CountMap[string, int]
}

func NewPatternRouter(pool ConnPool, opts PatternRouterOpts) *PatternRouter {
	opts = opts.withDefaults()

	return &PatternRouter{
		pool:           pool,
		logger:         opts.Logger.WithComponent("grpcbridge.routing"),
		routes:         newMutablePatternRoutingTable(),
		watcherCounter: countmap.New[string, int](),
	}
}

func (pr *PatternRouter) RouteHTTP(r *http.Request) (grpcadapter.ClientConn, HTTPRoute, error) {
	// Try to follow the same steps as in https://github.com/grpc-ecosystem/grpc-gateway/blob/main/runtime/mux.go#L328 (ServeMux.ServeHTTP).
	// Specifically, use RawPath for pattern matching, since it will be properly decoded by the pattern itself.
	path := r.URL.RawPath
	if !strings.HasPrefix(path, "/") {
		return nil, HTTPRoute{}, status.Error(codes.InvalidArgument, http.StatusText(http.StatusBadRequest))
	}

	pathComponents := strings.Split(path[1:], "/")
	lastPathComponent := pathComponents[len(pathComponents)-1]
	matchComponents := make([]string, len(pathComponents))

	var routeErr error
	var matchedTarget string
	var matchedRoute HTTPRoute

	pr.routes.iterate(r.Method, func(target string, route *patternRoute) bool {
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

		// Found match
		matchedTarget = target
		matchedRoute = HTTPRoute{Binding: route.binding, PathParams: params}
		return false
	})

	if routeErr != nil {
		return nil, HTTPRoute{}, routeErr
	}

	conn, ok := pr.pool.Get(matchedTarget)
	if !ok {
		return nil, HTTPRoute{}, status.Errorf(codes.Unavailable, "no connection to available for target %q", matchedTarget)
	}

	return conn, matchedRoute, nil
}

func (pr *PatternRouter) Watcher(target string) *PatternRouterWatcher {
	pr.watcherCounter.Inc(target)
	return &PatternRouterWatcher{pr: pr, target: target}
}

type PatternRouterWatcher struct {
	pr     *PatternRouter
	target string
	closed atomic.Bool
}

func (prw *PatternRouterWatcher) UpdateDesc(desc *bridgedesc.Target) {
	if prw.closed.Load() {
		return
	}

	prw.pr.updateRoutes(prw.target, desc)
}

func (prw *PatternRouterWatcher) ReportError(error) {}

func (prw *PatternRouterWatcher) Close() {
	if !prw.closed.CompareAndSwap(false, true) {
		panic("grpcbridge: PatternRouterWatcher.Close() called multiple times")
	}

	if prw.pr.watcherCounter.Dec(prw.target) == 0 {
		prw.pr.cleanRoutes(prw.target)
	}
}

func (pr *PatternRouter) updateRoutes(target string, desc *bridgedesc.Target) {
	routes := buildPatternRoutes(desc, pr.logger.With("target", target))
	pr.routes.addTarget(target, routes)
}

func (pr *PatternRouter) cleanRoutes(target string) {
	pr.routes.removeTarget(target)
}

func buildPatternRoutes(desc *bridgedesc.Target, logger bridgelog.Logger) map[string][]patternRoute {
	builder := newPatternRouteBuilder()

	for svcIdx := range desc.Services {
		svc := &desc.Services[svcIdx]
		methods := desc.Services[svcIdx].Methods // avoid copying the whole desc structures in loop
		for methodIdx := range methods {
			method := &methods[methodIdx]

			if len(method.Bindings) < 1 {
				if routeErr := builder.addDefault(method); routeErr != nil {
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
				if routeErr := builder.addBinding(method, binding); routeErr != nil {
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
	binding *bridgedesc.Binding
	pattern runtime.Pattern
}

func parsePatternRoute(method *bridgedesc.Method, binding *bridgedesc.Binding) (patternRoute, error) {
	compiler, err := httprule.Parse(binding.Pattern)
	if err != nil {
		return patternRoute{}, fmt.Errorf("parsing method %s binding pattern for %s: %w", method.RPCName, binding.HTTPMethod, err)
	}

	tp := compiler.Compile()

	pattern, routeErr := runtime.NewPattern(tp.Version, tp.OpCodes, tp.Pool, tp.Verb)
	if routeErr != nil {
		return patternRoute{}, fmt.Errorf("creating method %s binding pattern matcher for %s: %w", method.RPCName, binding.HTTPMethod, routeErr)
	}

	return patternRoute{
		binding: binding,
		pattern: pattern,
	}, nil
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

func (rb *patternRouteBuilder) addBinding(method *bridgedesc.Method, binding *bridgedesc.Binding) error {
	pr, err := parsePatternRoute(method, binding)
	if err != nil {
		return err
	}

	rb.routes[binding.HTTPMethod] = append(rb.routes[binding.HTTPMethod], pr)
	return nil
}

func (rb *patternRouteBuilder) addDefault(method *bridgedesc.Method) error {
	// Default gRPC form, as specified in https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests.
	return rb.addBinding(method, &bridgedesc.Binding{
		HTTPMethod: http.MethodPost,
		Pattern:    method.RPCName,
	})
}

// targetPatternRoutes define the routes for a single target+method combination.
type targetPatternRoutes struct {
	target string
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
func (mt *mutablePatternRoutingTable) addTarget(target string, routes map[string][]patternRoute) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	newMethodLinks := make([]methodPatternRoutes, 0, len(routes))

	// Update existing elements without recreating them.
	for _, link := range mt.targetLinks[target] {
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

	mt.targetLinks[target] = newMethodLinks

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

func (mt *mutablePatternRoutingTable) iterate(method string, fn func(target string, route *patternRoute) bool) {
	mt.static.Load().iterate(method, fn)
}

// staticPatternRoutingTable is a pattern-based routing table which can only be read.
// it is created by mutablePatternRoutingTable when modifications occur.
type staticPatternRoutingTable struct {
	routes map[string]*list.List // http method -> linked list of targetPatternRoutes
}

func (st *staticPatternRoutingTable) iterate(method string, fn func(target string, route *patternRoute) bool) {
	list, ok := st.routes[method]
	if !ok {
		return
	}

	for e := list.Front(); e != nil; e = e.Next() {
		pr := e.Value.(targetPatternRoutes)
		for _, route := range pr.routes {
			if !fn(pr.target, &route) {
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
