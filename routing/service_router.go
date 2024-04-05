package routing

import (
	"context"
	"strings"
	"sync"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type ServiceRouterOpts struct {
	Logger bridgelog.Logger
}

func (o ServiceRouterOpts) withDefaults() ServiceRouterOpts {
	if o.Logger == nil {
		o.Logger = defaultServiceRouterOpts.Logger
	}

	return o
}

var defaultServiceRouterOpts = ServiceRouterOpts{
	Logger: bridgelog.Discard(),
}

type ServiceRouter struct {
	pool   ConnPool
	logger bridgelog.Logger

	// protects the routing info
	// TODO(renbou): separate synchronization mechanisms for these maps, since only the routes are needed for actual routing,
	// 	and can be kept as a sync.Map, while svcRoutes are used just to remove the unnecessary routes,
	//  and also don't require a full lock to be held at all times.
	mu sync.Mutex
	// map from gRPC service name to the target name, used for actually routing requests.
	routes map[protoreflect.FullName]string
	// map from target name to its gRPC service names,
	// used for keeping track of a service's routes and manipulating the routing mapping on updates.
	svcRoutes map[string][]protoreflect.FullName
}

func NewServiceRouter(pool *grpcadapter.DialedPool, opts ServiceRouterOpts) *ServiceRouter {
	opts = opts.withDefaults()

	return &ServiceRouter{
		pool:      pool,
		logger:    opts.Logger.WithComponent("grpcbridge.route"),
		routes:    make(map[protoreflect.FullName]string),
		svcRoutes: make(map[string][]protoreflect.FullName),
	}
}

func (sr *ServiceRouter) RouteGRPC(ctx context.Context) (grpcadapter.ClientConn, *bridgedesc.Method, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	rpcName, ok := grpc.Method(ctx)
	if !ok {
		sr.logger.Error("no method name in stream context, unable to route request")
		return nil, nil, status.Errorf(codes.Internal, "grpcbridge: no method name in stream context, unable to route request")
	}

	svc, method, ok := parseRPCName(rpcName)
	if !ok {
		return nil, nil, status.Errorf(codes.Unimplemented, "no route for path of invalid gRPC format")
	}

	route, ok := sr.routes[svc]
	if !ok {
		return nil, nil, status.Errorf(codes.Unimplemented, "no route for gRPC service %q", svc)
	}

	conn := sr.pool.Get(route)
	if conn == nil {
		return nil, nil, status.Errorf(codes.Unavailable, "no connection to available in pool for target %q", route)
	}

	return conn, bridgedesc.DummyMethod(protoreflect.FullName(svc), protoreflect.Name(method)), nil
}

func (sr *ServiceRouter) Watcher(target string) *ServiceRouterWatcher {
	return &ServiceRouterWatcher{
		sr:     sr,
		target: target,
	}
}

func (sr *ServiceRouter) updateRoutes(target string, state *bridgedesc.Target) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	newSvcRoutes := make([]protoreflect.FullName, 0, len(state.Services))
	presentSvcRoutes := make(map[protoreflect.FullName]struct{}, len(state.Services))

	// Handle current routes
	for _, svc := range state.Services {
		// Add new routes
		route, ok := sr.routes[svc.Name]
		if !ok {
			sr.logger.Debug("adding route", "service", target, "route", svc.Name)

			sr.routes[svc.Name] = target
		} else if ok && route != target {
			// Since this router has no way to distinguish which routes go where,
			// it's better to avoid overwriting a route due to some accidental mistake by the user.
			sr.logger.Warn("ServiceRouter encountered gRPC service route conflict, keeping previous route",
				"route", svc.Name,
				"previous_target", route,
				"new_target", target,
			)
			continue
		}

		// Mark route as present to avoid removing it
		presentSvcRoutes[svc.Name] = struct{}{}
		newSvcRoutes = append(newSvcRoutes, svc.Name)
	}

	// Remove outdated routes
	for _, route := range sr.svcRoutes[target] {
		if _, ok := presentSvcRoutes[route]; !ok {
			sr.logger.Debug("removing route", "service", target, "route", route)

			delete(sr.routes, route)
		}
	}

	sr.svcRoutes[target] = newSvcRoutes
}

func (sr *ServiceRouter) cleanRoutes(target string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	routes := sr.svcRoutes[target]

	for _, route := range routes {
		delete(sr.routes, route)
	}

	delete(sr.svcRoutes, target)
}

type ServiceRouterWatcher struct {
	sr     *ServiceRouter
	target string
}

func (srw *ServiceRouterWatcher) UpdateDesc(desc *bridgedesc.Target) {
	srw.sr.updateRoutes(srw.target, desc)
}

func (srw *ServiceRouterWatcher) ReportError(error) {
	// error ignored because it can't be meaningfully used with this type of routing,
	// where the router can't tell which requests the error should be applied to,
	// as all of the routing is done based on information from the discovery resolver.
	// TODO(renbou): introduce a circuit breaker mechanism when errors are reported multiple times?
}

func (srw *ServiceRouterWatcher) Close() {
	srw.sr.cleanRoutes(srw.target)
}

func parseRPCName(rpcName string) (protoreflect.FullName, string, bool) {
	// Support both "/package.Service/Method" and "package.Service/Method" formats.
	if len(rpcName) > 0 && rpcName[0] == '/' {
		rpcName = rpcName[1:]
	}

	svc, method, ok := strings.Cut(rpcName, "/")
	return protoreflect.FullName(svc), method, ok
}
