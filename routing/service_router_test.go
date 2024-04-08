package routing

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/internal/bridgetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type fakeServerTransportStream struct {
	grpc.ServerTransportStream
	method string
}

func (sts fakeServerTransportStream) Method() string {
	return sts.method
}

type grpcTestCase struct {
	method        string
	routedService protoreflect.FullName
	routedMethod  string
	statusCode    codes.Code
}

func checkGRPCTestCase(t *testing.T, sr *ServiceRouter, tc *grpcTestCase) {
	t.Helper()

	// Act
	_, route, err := sr.RouteGRPC(grpc.NewContextWithServerTransportStream(context.Background(), fakeServerTransportStream{method: tc.method}))

	// Assert
	if cmpErr := bridgetest.StatusCodeIs(err, tc.statusCode); cmpErr != nil {
		t.Fatalf("RouteGRPC(%s) returned error = %q with unexpected code: %s", tc.method, err, cmpErr)
	}

	if tc.statusCode != codes.OK {
		return
	}

	if route.Method == nil || route.Service == nil || route.Target == nil {
		t.Fatalf("RouteGRPC(%s) returned route with missing info about method, service, or target: %#v", tc.method, route)
	}

	if route.Method.RPCName != tc.routedMethod {
		t.Errorf("RouteGRPC(%s) returned route with method = %q, want %q", tc.method, route.Method.RPCName, tc.routedMethod)
	}

	if route.Service.Name != tc.routedService {
		t.Errorf("RouteGRPC(%s) returned route with service = %q, want %q", tc.method, route.Service.Name, tc.routedService)
	}
}

// Test_ServiceRouter_RouteGRPC_Ok tests that ServiceRouter routes GRPC requests correctly.
func Test_ServiceRouter_RouteGRPC_Ok(t *testing.T) {
	t.Parallel()

	// Arrange
	sr := NewServiceRouter(nilConnPool{}, ServiceRouterOpts{})

	watcher, err := sr.Watch(testTarget.Name)
	if err != nil {
		t.Fatalf("Watch(%q) returned non-nil error = %q", testTarget.Name, err)
	}

	defer watcher.Close()
	watcher.UpdateDesc(&testTarget)

	tests := []grpcTestCase{
		{method: "/grpcbridge.routing.v1.PureGRPCSvc/Get", routedService: "grpcbridge.routing.v1.PureGRPCSvc", routedMethod: "/grpcbridge.routing.v1.PureGRPCSvc/Get", statusCode: codes.OK},
		{method: "grpcbridge.routing.v1.PureGRPCSvc/Get", routedService: "grpcbridge.routing.v1.PureGRPCSvc", routedMethod: "/grpcbridge.routing.v1.PureGRPCSvc/Get", statusCode: codes.OK},
		{method: "grpcbridge.routing.v1.PureGRPCSvc/UndefinedMethodButDefinedService", routedService: "grpcbridge.routing.v1.PureGRPCSvc", routedMethod: "/grpcbridge.routing.v1.PureGRPCSvc/UndefinedMethodButDefinedService", statusCode: codes.OK},
		{method: "/grpcbridge.routing.v2.RestSvc/GetEntity", routedService: "grpcbridge.routing.v2.RestSvc", routedMethod: "/grpcbridge.routing.v2.RestSvc/GetEntity", statusCode: codes.OK},
		{method: "/grpcbridge.unknown.Service/Get", statusCode: codes.Unimplemented},
		{method: "not a method", statusCode: codes.Unimplemented},
		{method: "/grpcbridge.missing_method.Service", statusCode: codes.Unimplemented},
	}

	// Run in non-parallel subtest so that watcher.Close() runs AFTER all the subtests.
	t.Run("cases", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.method, func(t *testing.T) {
				t.Parallel()

				// Act & Assert
				checkGRPCTestCase(t, sr, &tt)
			})
		}
	})
}

// Test_ServiceRouter_RouteHTTP_Ok tests that ServiceRouter routes HTTP requests correctly.
func Test_ServiceRouter_RouteHTTP_Ok(t *testing.T) {
	t.Parallel()

	// Arrange
	sr := NewServiceRouter(nilConnPool{}, ServiceRouterOpts{})

	watcher, err := sr.Watch(testTarget.Name)
	if err != nil {
		t.Fatalf("Watch(%q) returned non-nil error = %q", testTarget.Name, err)
	}

	defer watcher.Close()
	watcher.UpdateDesc(&testTarget)

	tests := []patternTestCase{
		{method: "POST", path: "/grpcbridge.routing.v1.PureGRPCSvc/Get", routedMethod: "/grpcbridge.routing.v1.PureGRPCSvc/Get", routedPattern: "/grpcbridge.routing.v1.PureGRPCSvc/Get", statusCode: codes.OK},
		{method: "GET", path: "/grpcbridge.routing.v1.PureGRPCSvc/Get", statusCode: codes.Unimplemented},
		{method: "POST", path: "/grpcbridge.routing.v1.PureGRPCSvc/List", routedMethod: "/grpcbridge.routing.v1.PureGRPCSvc/List", routedPattern: "/grpcbridge.routing.v1.PureGRPCSvc/List", statusCode: codes.OK},
		{method: "POST", path: "/grpcbridge.routing.v1.PureGRPCSvc/Unknown", routedMethod: "/grpcbridge.routing.v1.PureGRPCSvc/Unknown", routedPattern: "/grpcbridge.routing.v1.PureGRPCSvc/Unknown", statusCode: codes.OK},
		{method: "POST", path: "not a method", statusCode: codes.NotFound},
		{method: "POST", path: "/grpcbridge.missing_method.Service", statusCode: codes.NotFound},
		{method: "POST", path: "/grpcbridge.unknown.Service/Get", statusCode: codes.NotFound},
	}

	// Run in non-parallel subtest so that watcher.Close() runs AFTER all the subtests.
	t.Run("cases", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
				t.Parallel()

				// Act & Assert
				checkPatternTestCase(t, sr, &tt)
			})
		}
	})
}

// Test_ServiceRouter_RouteGRPC_Updates tests that ServiceRouter properly handles concurrently occurring updates to many targets.
// N targets are created with one of the 3 possible "behaviour" templates, which describes which updates come from the target's watcher.
// After these updates are applied, a routing test is performed to check that only the active routes for each target are routed.
func Test_ServiceRouter_RouteGRPC_Updates(t *testing.T) {
	t.Parallel()

	const N = 100
	const seed = 1

	// Arrange
	// A few update templates outlining different modifications which can come to a service router watcher.
	updateTemplates := [][]bridgedesc.Target{
		{
			// 2 services originally
			{Services: []bridgedesc.Service{{Name: "grpcbridge.routing.testsvc_%d.ServiceV1"}, {Name: "grpcbridge.routing.testsvc_%d.ServiceV2"}}},
			// v1 service gets removed
			{Services: []bridgedesc.Service{{Name: "grpcbridge.routing.testsvc_%d.ServiceV2"}}},
			// v2 service gets swapped for v3
			{Services: []bridgedesc.Service{{Name: "grpcbridge.routing.testsvc_%d.ServiceV3"}}},
		},
		{
			// No services
			{Services: []bridgedesc.Service{}},
			// 1 service gets added
			{Services: []bridgedesc.Service{{Name: "grpcbridge.routing.testsvc_%d.ServiceOne"}}},
		},
		{
			// 1 service which gets removed due to the watcher closing.
			{Services: []bridgedesc.Service{{Name: "grpcbridge.routing.testsvc_%d.ServiceClosed"}}},
		},
	}

	templateTests := [][]grpcTestCase{
		{
			// Only v3 should be available
			{method: "/grpcbridge.routing.testsvc_%d.ServiceV1/MethodV1", statusCode: codes.Unimplemented},
			{method: "/grpcbridge.routing.testsvc_%d.ServiceV2/MethodV2", statusCode: codes.Unimplemented},
			{method: "/grpcbridge.routing.testsvc_%d.ServiceV3/MethodV3", routedService: "grpcbridge.routing.testsvc_%d.ServiceV3", routedMethod: "/grpcbridge.routing.testsvc_%d.ServiceV3/MethodV3", statusCode: codes.OK},
		},
		{
			// The added service should be available
			{method: "/grpcbridge.routing.testsvc_%d.ServiceOne/Method", routedService: "grpcbridge.routing.testsvc_%d.ServiceOne", routedMethod: "/grpcbridge.routing.testsvc_%d.ServiceOne/Method", statusCode: codes.OK},
		},
		{
			// The closed service shouldn't be available
			{method: "/grpcbridge.routing.testsvc_%d.ServiceClosed/Method", statusCode: codes.Unimplemented},
		},
	}

	targets := buildTemplateTargets(N, seed, updateTemplates)
	sr := NewServiceRouter(nilConnPool{}, ServiceRouterOpts{})

	watchers := make([]*ServiceRouterWatcher, len(targets))
	for i, ti := range targets {
		var err error
		if watchers[i], err = sr.Watch(ti.name); err != nil {
			t.Fatalf("Watch(%q) returned non-nil error: %q", ti.name, err)
		}
	}

	// Act
	// Apply all the updates for each templated target.
	var wg sync.WaitGroup
	wg.Add(len(targets))

	for i, ti := range targets {
		go func() {
			defer wg.Done()

			var update bridgedesc.Target

			// Templating performed in parallel for faster testing.
			for updateIdx := range updateTemplates[ti.template] {
				buildDescTemplate(&updateTemplates[ti.template][updateIdx], &update, ti.name, i)
				watchers[i].UpdateDesc(&update)
			}

			// Additionally separately handle targets with template 3, which should be closed completely.
			if ti.template == 2 {
				watchers[i].Close()
			}
		}()
	}

	wg.Wait()

	// Assert
	// Concurrently validate routing for each target
	for i, ti := range targets {
		t.Run(fmt.Sprintf("%d-%s", ti.template, ti.name), func(t *testing.T) {
			t.Parallel()

			// Apply templating to each case before actually testing it.
			for _, tt := range templateTests[ti.template] {
				tt.method = fmt.Sprintf(tt.method, i)
				tt.routedMethod = fmt.Sprintf(tt.routedMethod, i)
				tt.routedService = protoreflect.FullName(fmt.Sprintf(string(tt.routedService), i))

				checkGRPCTestCase(t, sr, &tt)
			}
		})
	}
}
