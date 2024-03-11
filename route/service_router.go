package route

import (
	"strings"
	"sync"

	"github.com/renbou/grpcbridge"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/reflection"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type serviceRoute struct {
	name string
	desc reflection.ServiceDesc
}

type ServiceRouter struct {
	pool   *grpcadapter.ClientConnPool
	logger grpcbridge.Logger

	mu sync.Mutex // protects the routing info
	// map from gRPC service name to its bridged name and gRPC description,
	// used for actually routing requests
	routes map[string]*serviceRoute
	// map from bridged service name to its gRPC service names,
	// used for keeping track of a service's routes and removing them from/adding them to the routing map when needed.
	svcRoutes map[string][]string
}

func NewServiceRouter(logger grpcbridge.Logger, pool *grpcadapter.ClientConnPool) *ServiceRouter {
	return &ServiceRouter{
		pool:      pool,
		logger:    logger.WithComponent("grpcbridge.route"),
		routes:    make(map[string]*serviceRoute),
		svcRoutes: make(map[string][]string),
	}
}

func (sr *ServiceRouter) Route(path string) (*grpcadapter.ClientConn, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	svc, _, ok := parseGRPCMethod(path)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "no route for path of invalid gRPC format")
	}

	route, ok := sr.routes[svc]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "no route for gRPC service %q", svc)
	}

	conn := sr.pool.Get(route.name)
	if conn == nil {
		return nil, status.Errorf(codes.Unavailable, "no connection to available in pool for target %q", route.name)
	}

	return conn, nil
}

func (sr *ServiceRouter) Watcher(name string) *ServiceRouterWatcher {
	return &ServiceRouterWatcher{
		sr:   sr,
		name: name,
	}
}

func (sr *ServiceRouter) updateRoutes(name string, state *reflection.DiscoveryState) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	newSvcRoutes := make([]string, 0, len(state.Services))
	presentSvcRoutes := make(map[string]struct{}, len(state.Services))

	// Handle current routes
	for _, svc := range state.Services {
		// Add new routes
		route, ok := sr.routes[string(svc.Name)]
		if !ok {
			sr.logger.Debug("adding route", "service", name, "route", svc.Name)

			route = &serviceRoute{name: name}
			sr.routes[string(svc.Name)] = route
		} else if ok && route.name != name {
			// Since this router has no way to distinguish which routes go where,
			// it's better to avoid overwriting a route due to some accidental mistake by the user.
			sr.logger.Warn("ServiceRouter encountered gRPC service route conflict, keeping previous route",
				"route", svc.Name,
				"previous_service", route.name,
				"new_service", name,
			)
			continue
		}

		// Update the route description
		route.desc = svc

		// Mark route as present to avoid removing it
		presentSvcRoutes[string(svc.Name)] = struct{}{}
		newSvcRoutes = append(newSvcRoutes, string(svc.Name))
	}

	// Remove outdated routes
	for _, route := range sr.svcRoutes[name] {
		if _, ok := presentSvcRoutes[route]; !ok {
			sr.logger.Debug("removing route", "service", name, "route", route)

			delete(sr.routes, route)
		}
	}

	sr.svcRoutes[name] = newSvcRoutes
}

func (sr *ServiceRouter) cleanRoutes(name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	routes := sr.svcRoutes[name]

	for _, route := range routes {
		delete(sr.routes, route)
	}

	delete(sr.svcRoutes, name)
}

type ServiceRouterWatcher struct {
	sr   *ServiceRouter
	name string
}

func (srw *ServiceRouterWatcher) UpdateState(state *reflection.DiscoveryState) {
	srw.sr.updateRoutes(srw.name, state)
}

func (srw *ServiceRouterWatcher) ReportError(error) {
	// error ignored because it can't be meaningfully used with this type of routing,
	// where the router can't tell which requests the error should be applied to,
	// as all of the routing is done based on information from the discovery resolver.
	// TODO(renbou): remove routes for this service if multiple errors are reported without state updates?
}

func (srw *ServiceRouterWatcher) Close() {
	srw.sr.mu.Lock()
	defer srw.sr.mu.Unlock()

	srw.sr.cleanRoutes(srw.name)
}

func parseGRPCMethod(method string) (string, string, bool) {
	if len(method) < 1 || method[0] != '/' {
		return "", "", false
	}

	return strings.Cut(method[1:], "/")
}
