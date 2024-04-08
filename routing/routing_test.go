package routing

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/grpcadapter"
	"go.uber.org/goleak"
	"google.golang.org/protobuf/reflect/protoreflect"
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

type templateTargetInfo struct {
	template int
	name     string
}

func buildTemplateTargets(n int, seed int64, templates [][]bridgedesc.Target) []templateTargetInfo {
	rnd := rand.New(rand.NewSource(seed))
	targets := make([]templateTargetInfo, n)

	for i := range targets {
		targets[i] = templateTargetInfo{template: rnd.Intn(len(templates)), name: fmt.Sprintf("target_%d", i)}
	}

	return targets
}

// buildDescTemplate builds a target description from a template,
// used in router tests for generating multiple different targets with unique names based on a few templates.
func buildDescTemplate(template *bridgedesc.Target, dest *bridgedesc.Target, targetName string, targetIdx int) {
	dest.Name = targetName
	dest.Services = make([]bridgedesc.Service, len(template.Services))

	for svcIdx := range template.Services {
		templateSvc := &template.Services[svcIdx]
		destSvc := &dest.Services[svcIdx]

		destSvc.Name = protoreflect.FullName(fmt.Sprintf(string(templateSvc.Name), targetIdx))
		destSvc.Methods = make([]bridgedesc.Method, len(templateSvc.Methods))

		for methodIdx := range templateSvc.Methods {
			templateMethod := &templateSvc.Methods[methodIdx]
			destMethod := &destSvc.Methods[methodIdx]

			destMethod.RPCName = fmt.Sprintf(templateMethod.RPCName, targetIdx)
			destMethod.Bindings = make([]bridgedesc.Binding, len(templateMethod.Bindings))

			for bindingIdx := range templateMethod.Bindings {
				templateBinding := &templateMethod.Bindings[bindingIdx]
				destBinding := &destMethod.Bindings[bindingIdx]

				destBinding.HTTPMethod = templateBinding.HTTPMethod
				destBinding.Pattern = fmt.Sprintf(templateBinding.Pattern, targetIdx)
			}
		}
	}
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
