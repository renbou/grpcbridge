package routing

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/syncset"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ServiceRouterOpts define all the optional settings which can be set for [ServiceRouter].
type ServiceRouterOpts struct {
	Logger bridgelog.Logger
}

func (o ServiceRouterOpts) withDefaults() ServiceRouterOpts {
	if o.Logger == nil {
		o.Logger = bridgelog.Discard()
	}

	return o
}

// ServiceRouter is a router meant for routing HTTP or gRPC requests with the standard gRPC method format of the form "/package.Service/Method".
// It relies on all targets having unique gRPC service names, and it doesn't even take the method name into account,
// which allows it to run solely based on the knowledge of a target's service names.
// This is useful as it allows description resolvers to be greatly simplified or optimized.
// For example, the reflection resolver in [github.com/renbou/grpcbridge/reflection] has an OnlyServices option for this specific case.
type ServiceRouter struct {
	pool       grpcadapter.ClientPool
	logger     bridgelog.Logger
	watcherSet *syncset.SyncSet[string]

	// map from gRPC service name to the target name, used for actually routing requests.
	// using a sync.Map makes sense here because writes to it happen very rarely compared to reads.
	routes sync.Map // protoreflect.FullName -> serviceRoute

	// used to synchronize updates, doesn't affect reading.
	mu sync.Mutex

	// map from target name to its gRPC service names,
	// used for keeping track of a service's routes and manipulating the routing mapping on updates.
	svcRoutes map[string][]protoreflect.FullName
}

// NewServiceRouter initializes a new [ServiceRouter] with the specified connection pool and options.
//
// The connection pool will be used to perform a simple retrieval of the connection to a target by its name.
// for more complex connection routing this router's methods can be wrapped to return a
// different connection based on the matched method and HTTP/GRPC request information.
func NewServiceRouter(pool grpcadapter.ClientPool, opts ServiceRouterOpts) *ServiceRouter {
	opts = opts.withDefaults()

	return &ServiceRouter{
		pool:       pool,
		logger:     opts.Logger.WithComponent("grpcbridge.routing"),
		watcherSet: syncset.New[string](),
		svcRoutes:  make(map[string][]protoreflect.FullName),
	}
}

// RouteGRPC routes the gRPC request based on its method, which is retrieved using [grpc.Method].
// The context is expected to be a stream/request context from a valid gRPC request,
// however all the necessary information can be added to it manually for routing some custom requests using [grpc.NewContextWithServerTransportStream].
//
// Errors returned by RouteGRPC are gRPC status.Status errors with the code set accordingly.
// Currently, the Internal, Unimplemented, and Unavailable codes are returned.
//
// Performance-wise it is notable that updates to the routing information don't block RouteGRPC, happening fully in the background.
func (sr *ServiceRouter) RouteGRPC(ctx context.Context) (grpcadapter.ClientConn, GRPCRoute, error) {
	rpcName, ok := grpc.Method(ctx)
	if !ok {
		sr.logger.Error("no method name in request context, unable to route request")
		return nil, GRPCRoute{}, status.Errorf(codes.Internal, "grpcbridge: no method name in request context")
	}

	svc, method, ok := parseRPCName(rpcName)
	if !ok {
		// https://github.com/grpc/grpc-go/blob/6fbcd8a889526b3307c3a33cba5b1d2190f0fe11/server.go#L1755
		return nil, GRPCRoute{}, status.Errorf(codes.Unimplemented, "malformed method name: %q", rpcName)
	}

	routeAny, ok := sr.routes.Load(svc)
	if !ok {
		// https://github.com/grpc/grpc-go/blob/6fbcd8a889526b3307c3a33cba5b1d2190f0fe11/server.go#L1805
		return nil, GRPCRoute{}, status.Errorf(codes.Unimplemented, "unknown service %v", svc)
	}

	route := routeAny.(serviceRoute)

	conn, ok := sr.pool.Get(route.target.Name)
	if !ok {
		return nil, GRPCRoute{}, status.Errorf(codes.Unavailable, "no connection available to target %q", route.target.Name)
	}

	return conn, GRPCRoute{
		Target:  route.target,
		Service: route.service,
		Method:  bridgedesc.DummyMethod(protoreflect.FullName(svc), protoreflect.Name(method)),
	}, nil
}

// RouteHTTP implements routing for POST HTTP requests, using the request path as the method name.
// It simulates pattern-based routing with default bindings, but is much more efficient than [PatternRouter.RouteHTTP]
// because it relies solely on the service name part of the request path and doesn't have to perform any pattern-matching.
//
// See [ServiceRouter.RouteGRPC] for more details, as this method is very similar,
// with the only notable differences being the different status codes returned, which are more suitable for HTTP.
// Additionally, an error implementing interface { HTTPStatus() int } can be returned, which should be used to set a custom status code.
func (sr *ServiceRouter) RouteHTTP(r *http.Request) (grpcadapter.ClientConn, HTTPRoute, error) {
	if r.Method != http.MethodPost {
		return nil, HTTPRoute{}, &httpStatusError{code: http.StatusMethodNotAllowed, err: status.Errorf(codes.Unimplemented, http.StatusText(http.StatusMethodNotAllowed))}
	}

	rpcName := r.URL.RawPath

	svc, method, ok := parseRPCName(rpcName)
	if !ok {
		// NotFound here because a 404 is what will be given by pattern-based routers and other HTTP-like routers,
		// it makes more sense than returning a 400 or some other error.
		return nil, HTTPRoute{}, status.Errorf(codes.NotFound, http.StatusText(http.StatusNotFound))
	}

	routeAny, ok := sr.routes.Load(svc)
	if !ok {
		return nil, HTTPRoute{}, status.Errorf(codes.NotFound, http.StatusText(http.StatusNotFound))
	}

	route := routeAny.(serviceRoute)

	conn, ok := sr.pool.Get(route.target.Name)
	if !ok {
		return nil, HTTPRoute{}, status.Errorf(codes.Unavailable, "no connection available to target %q", route.target.Name)
	}

	methodDesc := bridgedesc.DummyMethod(protoreflect.FullName(svc), protoreflect.Name(method))
	binding := bridgedesc.DefaultBinding(methodDesc)

	return conn, HTTPRoute{
		Target:     route.target,
		Service:    route.service,
		Method:     methodDesc,
		Binding:    binding,
		PathParams: nil, // PatternRouter also returns nil
	}, nil
}

// Watch starts watching the specified target for description changes.
// It returns a [*ServiceRouterWatcher] through which new updates for this target can be applied.
//
// It is an error to try Watch()ing the same target multiple times on a single ServiceRouter instance,
// See the comment for [PatternRouter.Watch] for some extra detail regarding this API mechanic.
func (sr *ServiceRouter) Watch(target string) (*ServiceRouterWatcher, error) {
	if sr.watcherSet.Add(target) {
		return &ServiceRouterWatcher{sr: sr, target: target}, nil
	}

	return nil, ErrAlreadyWatching
}

// ServiceRouterWatcher is a description update watcher created for a specific target in the context of a [ServiceRouter] instance.
// New ServiceRouterWatchers are created through [ServiceRouter.Watch].
type ServiceRouterWatcher struct {
	sr     *ServiceRouter
	target string
	closed atomic.Bool
}

// UpdateDesc updates the description of the target this watcher is watching.
// It follows the same semantics as [PatternRouterWatcher.UpdateDesc],
// the documentation for which goes into more detail.
func (srw *ServiceRouterWatcher) UpdateDesc(desc *bridgedesc.Target) {
	if srw.closed.Load() {
		return
	}

	if desc.Name != srw.target {
		srw.sr.logger.Error("ServiceRouterWatcher got update for different target, will ignore", "watcher_target", srw.target, "update_target", desc.Name)
		return
	}

	srw.sr.updateRoutes(desc)
}

// ReportError is currently a no-op, present simply to implement the Watcher interface
// of the grpcbridge description resolvers, such as the one in [github.com/renbou/grpcbridge/reflection].
func (srw *ServiceRouterWatcher) ReportError(error) {
	// TODO(renbou): introduce a circuit breaker mechanism when errors are reported multiple times?
}

// Close closes the watcher, preventing further updates from being applied to the router through it.
// It is an error to call Close() multiple times on the same watcher, and doing so will result in a panic.
func (srw *ServiceRouterWatcher) Close() {
	if !srw.closed.CompareAndSwap(false, true) {
		panic("grpcbridge: ServiceRouterWatcher.Close() called multiple times")
	}

	srw.sr.removeTarget(srw.target)
	srw.sr.watcherSet.Remove(srw.target)
}

type serviceRoute struct {
	target  *bridgedesc.Target
	service *bridgedesc.Service
}

func (sr *ServiceRouter) updateRoutes(desc *bridgedesc.Target) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	newSvcRoutes := make([]protoreflect.FullName, 0, len(desc.Services))
	presentSvcRoutes := make(map[protoreflect.FullName]struct{}, len(desc.Services))

	// Handle current routes
	for i := range desc.Services {
		svc := &desc.Services[i]

		// Add new routes
		route, ok := sr.routes.LoadOrStore(svc.Name, serviceRoute{target: desc, service: svc})
		if !ok {
			sr.logger.Debug("adding route", "target", desc.Name, "service", svc.Name)
		} else if ok && route.(serviceRoute).target.Name != desc.Name {
			// Since this router has no way to distinguish which routes go where,
			// it's better to avoid overwriting a route due to some accidental mistake by the user.
			sr.logger.Warn("ServiceRouter encountered gRPC service route conflict, keeping previous route",
				"service", svc.Name,
				"previous_target", route.(serviceRoute).target.Name,
				"new_target", desc.Name,
			)
			continue
		}

		// Mark route as present to avoid removing it
		presentSvcRoutes[svc.Name] = struct{}{}
		newSvcRoutes = append(newSvcRoutes, svc.Name)
	}

	// Remove outdated routes
	for _, route := range sr.svcRoutes[desc.Name] {
		if _, ok := presentSvcRoutes[route]; !ok {
			sr.logger.Debug("removing route", "target", desc.Name, "service", route)
			sr.routes.Delete(route)
		}
	}

	sr.svcRoutes[desc.Name] = newSvcRoutes
}

func (sr *ServiceRouter) removeTarget(target string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	routes := sr.svcRoutes[target]

	for _, route := range routes {
		sr.routes.Delete(route)
	}

	delete(sr.svcRoutes, target)
}

func parseRPCName(rpcName string) (protoreflect.FullName, string, bool) {
	// Support both "/package.Service/Method" and "package.Service/Method" formats.
	if len(rpcName) > 0 && rpcName[0] == '/' {
		rpcName = rpcName[1:]
	}

	svc, method, ok := strings.Cut(rpcName, "/")
	return protoreflect.FullName(svc), method, ok
}
