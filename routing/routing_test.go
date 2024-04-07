package routing

import (
	"testing"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/grpcadapter"
	"go.uber.org/goleak"
)

// nilConnPool implements ConnPool without returning any connection for tests.
type nilConnPool struct{}

func (nilConnPool) Get(target string) (grpcadapter.ClientConn, bool) {
	return nil, true
}

var testTarget = bridgedesc.Target{
	Name: "routing-testsvc",
	Services: []bridgedesc.Service{
		{
			Name: "grpcbridge.routing.v1.PureGRPCSvc",
			Methods: []bridgedesc.Method{
				{RPCName: "/grpcbridge.routing.v1.PureGRPCSvc/Get"},
				{RPCName: "/grpcbridge.routing.v1.PureGRPCSvc/List"},
			},
		},
		{
			Name: "grpcbridge.routing.v1.RestSvc",
			Methods: []bridgedesc.Method{
				{
					RPCName: "/grpcbridge.routing.v1.RestSvc/CreateEntity",
					Bindings: []bridgedesc.Binding{
						{HTTPMethod: "POST", Pattern: "/api/v1/entities"},
						{HTTPMethod: "POST", Pattern: "/api/v1/entity"},
					},
				},
				{
					RPCName: "/grpcbridge.routing.v1.RestSvc/GetEntity",
					Bindings: []bridgedesc.Binding{
						{HTTPMethod: "GET", Pattern: "/api/v1/entity/{entity_id=*}"}, // equivalent to {entity_id}
					},
				},
				{
					RPCName: "/grpcbridge.routing.v1.RestSvc/ListEntities",
					Bindings: []bridgedesc.Binding{
						{HTTPMethod: "GET", Pattern: "/api/v1/entities"},
					},
				},
				{
					RPCName: "/grpcbridge.routing.v1.RestSvc/UpdateEntity",
					Bindings: []bridgedesc.Binding{
						{HTTPMethod: "PUT", Pattern: "/api/v1/entity/{entity_id}"},
						{HTTPMethod: "PATCH", Pattern: "/api/v1/entity/{entity_id}"},
					},
				},
				{
					RPCName: "/grpcbridge.routing.v1.RestSvc/DeleteEntity",
					Bindings: []bridgedesc.Binding{
						{HTTPMethod: "DELETE", Pattern: "/api/v1/entity/{entity_id}"},
					},
				},
			},
		},
		{
			Name: "grpcbridge.routing.v2.RestSvc",
			Methods: []bridgedesc.Method{
				{
					RPCName: "/grpcbridge.routing.v2.RestSvc/CreateEntity",
					Bindings: []bridgedesc.Binding{
						{HTTPMethod: "POST", Pattern: "/api/v2/entity:create"},
					},
				},
				{
					RPCName: "/grpcbridge.routing.v2.RestSvc/GetEntity",
					Bindings: []bridgedesc.Binding{
						{HTTPMethod: "GET", Pattern: "/api/v2/entity/{entity_id}"},
					},
				},
				{
					RPCName: "/grpcbridge.routing.v2.RestSvc/WatchEntity",
					Bindings: []bridgedesc.Binding{
						{HTTPMethod: "POST", Pattern: "/api/v2/entity/{entity_id}/{path=**}:watch"},
						{HTTPMethod: "POST", Pattern: "/api/v2/entities/{path=all/**}:watch"},
					},
				},
			},
		},
	},
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
