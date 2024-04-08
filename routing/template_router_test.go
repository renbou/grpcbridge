package routing

import (
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/bridgetest"
	"google.golang.org/grpc/codes"
)

type httpRouter interface {
	RouteHTTP(req *http.Request) (grpcadapter.ClientConn, HTTPRoute, error)
}

type patternTestCase struct {
	method        string
	path          string
	routedMethod  string
	routedPattern string
	routedParams  map[string]string
	statusCode    codes.Code
}

func checkPatternTestCase(t *testing.T, router httpRouter, tc *patternTestCase) {
	t.Helper()

	// Act
	_, route, err := router.RouteHTTP(&http.Request{Method: tc.method, URL: &url.URL{RawPath: tc.path}})

	// Assert
	if cmpErr := bridgetest.StatusCodeIs(err, tc.statusCode); cmpErr != nil {
		t.Fatalf("RouteHTTP(%s, %q) returned error = %q with unexpected code: %s", tc.method, tc.path, err, cmpErr)
	}

	if tc.statusCode != codes.OK {
		return
	}

	if route.Binding == nil || route.Method == nil || route.Service == nil || route.Target == nil {
		t.Fatalf("RouteHTTP(%s, %q) returned route with missing info about binding, method, service, or target: %#v", tc.method, tc.path, route)
	}

	if route.Binding.Pattern != tc.routedPattern {
		t.Errorf("RouteHTTP(%s, %q) matched route with pattern = %q, want %q", tc.method, tc.path, route.Binding.Pattern, tc.routedPattern)
	}

	if route.Method.RPCName != tc.routedMethod {
		t.Errorf("RouteHTTP(%s, %q) matched route with method = %q, want %q", tc.method, tc.path, route.Method.RPCName, tc.routedMethod)
	}

	if diff := cmp.Diff(tc.routedParams, route.PathParams); diff != "" {
		t.Errorf("RouteHTTP(%s, %q) matched route with path params differing from expected (-want+got):\n%s", tc.method, tc.path, diff)
	}
}

// Test_PatternRouter_RouteHTTP_Ok tests that PatternRouter routes HTTP requests correctly.
func Test_PatternRouter_RouteHTTP_Ok(t *testing.T) {
	t.Parallel()

	// Arrange
	pr := NewPatternRouter(nilConnPool{}, PatternRouterOpts{})

	watcher, err := pr.Watch(testTarget.Name)
	if err != nil {
		t.Fatalf("Watch(%q) returned non-nil error = %q", testTarget.Name, err)
	}

	defer watcher.Close()
	watcher.UpdateDesc(&testTarget) // this returns when the update has been applied

	tests := []patternTestCase{
		{method: "POST", path: "not a path", statusCode: codes.InvalidArgument},
		{method: "POST", path: "/grpcbridge.routing.v1.PureGRPCSvc/Get", routedMethod: "/grpcbridge.routing.v1.PureGRPCSvc/Get", routedPattern: "/grpcbridge.routing.v1.PureGRPCSvc/Get", routedParams: nil, statusCode: codes.OK},
		{method: "GET", path: "/grpcbridge.routing.v1.PureGRPCSvc/Get", statusCode: codes.NotFound},
		{method: "POST", path: "/grpcbridge.routing.v1.PureGRPCSvc/List", routedMethod: "/grpcbridge.routing.v1.PureGRPCSvc/List", routedPattern: "/grpcbridge.routing.v1.PureGRPCSvc/List", routedParams: nil, statusCode: codes.OK},
		{method: "PATCH", path: "/grpcbridge.routing.v1.PureGRPCSvc/List", statusCode: codes.NotFound},
		{method: "POST", path: "/grpcbridge.routing.v1.PureGRPCSvc/Unknown", statusCode: codes.NotFound},
		{method: "POST", path: "/api/v1/entities", routedMethod: "/grpcbridge.routing.v1.RestSvc/CreateEntity", routedPattern: "/api/v1/entities", routedParams: nil, statusCode: codes.OK},
		{method: "POST", path: "/api/v1/entity", routedMethod: "/grpcbridge.routing.v1.RestSvc/CreateEntity", routedPattern: "/api/v1/entity", routedParams: nil, statusCode: codes.OK},
		{method: "GET", path: "/api/v1/entity/1", routedMethod: "/grpcbridge.routing.v1.RestSvc/GetEntity", routedPattern: "/api/v1/entity/{entity_id=*}", routedParams: map[string]string{"entity_id": "1"}, statusCode: codes.OK},
		{method: "GET", path: "/api/v1/entity/asdf1234", routedMethod: "/grpcbridge.routing.v1.RestSvc/GetEntity", routedPattern: "/api/v1/entity/{entity_id=*}", routedParams: map[string]string{"entity_id": "asdf1234"}, statusCode: codes.OK},
		{method: "GET", path: "/api/v1/entity/asdf1234/sub", statusCode: codes.NotFound},
		{method: "GET", path: "/api/v1/entity/1:fakeverb", routedMethod: "/grpcbridge.routing.v1.RestSvc/GetEntity", routedPattern: "/api/v1/entity/{entity_id=*}", routedParams: map[string]string{"entity_id": "1:fakeverb"}, statusCode: codes.OK},
		{method: "GET", path: "/api/v1/entities", routedMethod: "/grpcbridge.routing.v1.RestSvc/ListEntities", routedPattern: "/api/v1/entities", routedParams: nil, statusCode: codes.OK},
		{method: "PUT", path: "/api/v1/entity/%61%29%30%20%0a%2b", routedMethod: "/grpcbridge.routing.v1.RestSvc/UpdateEntity", routedPattern: "/api/v1/entity/{entity_id}", routedParams: map[string]string{"entity_id": "a)0 \n+"}, statusCode: codes.OK},
		{method: "PATCH", path: "/api/v1/entity/entity?+", routedMethod: "/grpcbridge.routing.v1.RestSvc/UpdateEntity", routedPattern: "/api/v1/entity/{entity_id}", routedParams: map[string]string{"entity_id": "entity?+"}, statusCode: codes.OK},
		{method: "DELETE", path: "/api/v1/entity/", routedMethod: "/grpcbridge.routing.v1.RestSvc/DeleteEntity", routedPattern: "/api/v1/entity/{entity_id}", routedParams: map[string]string{"entity_id": ""}, statusCode: codes.OK},
		{method: "POST", path: "/api/v2/entity:create", routedMethod: "/grpcbridge.routing.v2.RestSvc/CreateEntity", routedPattern: "/api/v2/entity:create", routedParams: nil, statusCode: codes.OK},
		{method: "post", path: "/api/v2/entity:create", statusCode: codes.NotFound},
		{method: "GET", path: "/api/v2/entity/asdf1234", routedMethod: "/grpcbridge.routing.v2.RestSvc/GetEntity", routedPattern: "/api/v2/entity/{entity_id}", routedParams: map[string]string{"entity_id": "asdf1234"}, statusCode: codes.OK},
		{method: "POST", path: "/api/v2/entity/testentity/test/sub:watch", routedMethod: "/grpcbridge.routing.v2.RestSvc/WatchEntity", routedPattern: "/api/v2/entity/{entity_id}/{path=**}:watch", routedParams: map[string]string{"entity_id": "testentity", "path": "test/sub"}, statusCode: codes.OK},
		{method: "POST", path: "/api/v2/entity/testentity/test/:watch", statusCode: codes.NotFound},
		{method: "POST", path: "/api/v2/entities/all/test/sub:watch", routedMethod: "/grpcbridge.routing.v2.RestSvc/WatchEntity", routedPattern: "/api/v2/entities/{path=all/**}:watch", routedParams: map[string]string{"path": "all/test/sub"}, statusCode: codes.OK},
		{method: "POST", path: "/api/v2/entity/testentity/%61%29%30%20%0a%2b/sub:watch", routedMethod: "/grpcbridge.routing.v2.RestSvc/WatchEntity", routedPattern: "/api/v2/entity/{entity_id}/{path=**}:watch", routedParams: map[string]string{"entity_id": "testentity", "path": "a%290 \n%2b/sub"}, statusCode: codes.OK},
	}

	// Run in non-parallel subtest so that watcher.Close() runs AFTER all the subtests.
	t.Run("cases", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
				t.Parallel()

				// Arrange
				// Need to wait for routes to be synced to other goroutines, since they're stored as an atomic.
				for {
					table := pr.routes.static.Load()
					if len(table.routes) > 0 {
						break
					}

					runtime.Gosched()
				}

				// Act & Assert
				checkPatternTestCase(t, pr, &tt)
			})
		}
	})
}

// Test_PatternRouter_RouteHTTP_Updates tests that PatternRouter properly handles concurrently occurring updates to many targets.
// N targets are created with one of the 4 possible "behaviour" templates, which describes which updates come from the target's watcher.
// After these updates are applied, a routing test is performed to check that only the active routes for each target are routed.
func Test_PatternRouter_RouteHTTP_Updates(t *testing.T) {
	t.Parallel()

	const N = 100
	const seed = 1

	// Arrange
	// A few update templates outlining different modifications which can come to a pattern router watcher.
	updateTemplates := [][]bridgedesc.Target{
		{
			// 2 methods originally, both will be bound as defaults with POST
			{Services: []bridgedesc.Service{
				{Methods: []bridgedesc.Method{{RPCName: "/grpcbridge.routing.testsvc_%d.v1/Method"}}},
				{Methods: []bridgedesc.Method{{RPCName: "/grpcbridge.routing.testsvc_%d.v2/Method"}}},
			}},
			// then, one method receives a proper binding with a new GET method
			{Services: []bridgedesc.Service{
				{Methods: []bridgedesc.Method{{RPCName: "/grpcbridge.routing.testsvc_%d.v1/Method"}}},
				{Methods: []bridgedesc.Method{{RPCName: "/grpcbridge.routing.testsvc_%d.v2/Method", Bindings: []bridgedesc.Binding{{HTTPMethod: "GET", Pattern: "/testsvc_%d/v2/{id}"}}}}},
			}},
			// then, the methods get deleted and a new one is added with a PATCH method
			{Services: []bridgedesc.Service{
				{Methods: []bridgedesc.Method{{RPCName: "/grpcbridge.routing.testsvc_%d.v3/Method", Bindings: []bridgedesc.Binding{{HTTPMethod: "PATCH", Pattern: "/testsvc_%d/v3/{id}"}}}}},
			}},
		},
		{
			// 2 methods on a single service, both with bindings
			{Services: []bridgedesc.Service{
				{Methods: []bridgedesc.Method{
					{RPCName: "/grpcbridge.routing.testsvc_%d/MethodA", Bindings: []bridgedesc.Binding{{HTTPMethod: "GET", Pattern: "/testsvc_%d/a"}}},
					{RPCName: "/grpcbridge.routing.testsvc_%d/MethodB", Bindings: []bridgedesc.Binding{{HTTPMethod: "GET", Pattern: "/testsvc_%d/b"}}},
				}},
			}},
			// methods move to different services but keep the same bindings
			{Services: []bridgedesc.Service{
				{Methods: []bridgedesc.Method{{RPCName: "/grpcbridge.routing.testsvc_%d.v1/MethodA", Bindings: []bridgedesc.Binding{{HTTPMethod: "GET", Pattern: "/testsvc_%d/a"}}}}},
				{Methods: []bridgedesc.Method{{RPCName: "/grpcbridge.routing.testsvc_%d.v2/MethodB", Bindings: []bridgedesc.Binding{{HTTPMethod: "GET", Pattern: "/testsvc_%d/b"}}}}},
			}},
		},
		{
			// 2 methods on a single service - one with good bindings, other with invalid binding
			{Services: []bridgedesc.Service{
				{Methods: []bridgedesc.Method{
					{RPCName: "/grpcbridge.routing.testsvc_%d/MethodOk", Bindings: []bridgedesc.Binding{
						{HTTPMethod: "GET", Pattern: "/api/testsvc_%d/ok:verb"},
						{HTTPMethod: "POST", Pattern: "/api/testsvc_%d/{vals=sub/*/**}"},
						{HTTPMethod: "DELETE", Pattern: "/api/testsvc_%d/action:delete"},
					}},
					{RPCName: "/grpcbridge.routing.testsvc_%d/MethodBad", Bindings: []bridgedesc.Binding{{HTTPMethod: "GET", Pattern: "/service %d: this isn't a proper path, right?"}}},
				}},
			}},
			// the method with invalid binding is removed, the other one contains one less binding
			{Services: []bridgedesc.Service{
				{Methods: []bridgedesc.Method{
					{RPCName: "/grpcbridge.routing.testsvc_%d/MethodOk", Bindings: []bridgedesc.Binding{
						{HTTPMethod: "POST", Pattern: "/api/testsvc_%d/{vals=sub/*/**}"},
						{HTTPMethod: "DELETE", Pattern: "/api/testsvc_%d/action:delete"},
					}},
				}},
			}},
		},
		{
			// template number 4 contains a single binding, and then the watcher gets stopped, so no binding should be left over.
			{Services: []bridgedesc.Service{{Methods: []bridgedesc.Method{{RPCName: "/grpcbridge.routing.testsvc_%d/DeletedTarget"}}}}},
		},
	}

	// Test cases for each template, testing that the additions/updates/removals of patterns were handled correctly.
	templateTests := [][]patternTestCase{
		{
			// First three cases are the ones that shouldn't be present after updates, the last is the only one present.
			{method: "POST", path: "/grpcbridge.routing.testsvc_%d.v1/Method", statusCode: codes.NotFound},
			{method: "POST", path: "/grpcbridge.routing.testsvc_%d.v2/Method", statusCode: codes.NotFound},
			{method: "GET", path: "/testsvc_%d/v2/exampleid", statusCode: codes.NotFound},
			{method: "PATCH", path: "/testsvc_%d/v3/exampleid", routedMethod: "/grpcbridge.routing.testsvc_%d.v3/Method", routedPattern: "/testsvc_%d/v3/{id}", routedParams: map[string]string{"id": "exampleid"}, statusCode: codes.OK},
		},
		{
			// The patterns haven't changed, but requests should be routed to the correct path.
			{method: "GET", path: "/testsvc_%d/a", routedMethod: "/grpcbridge.routing.testsvc_%d.v1/MethodA", routedPattern: "/testsvc_%d/a", routedParams: nil, statusCode: codes.OK},
			{method: "GET", path: "/testsvc_%d/b", routedMethod: "/grpcbridge.routing.testsvc_%d.v2/MethodB", routedPattern: "/testsvc_%d/b", routedParams: nil, statusCode: codes.OK},
		},
		{
			// The removed and invalid patterns shouldn't be present, the rest should still be working.
			{method: "GET", path: "/api/testsvc_%d/ok:verb", statusCode: codes.NotFound},
			{method: "GET", path: "/service %d: this isn't a proper path, right?", statusCode: codes.NotFound},
			{method: "POST", path: "/api/testsvc_%d/sub/kek/extra/path", routedMethod: "/grpcbridge.routing.testsvc_%d/MethodOk", routedPattern: "/api/testsvc_%d/{vals=sub/*/**}", routedParams: map[string]string{"vals": "sub/kek/extra/path"}, statusCode: codes.OK},
			{method: "POST", path: "/api/testsvc_%d/sub", statusCode: codes.NotFound},
			{method: "DELETE", path: "/api/testsvc_%d/action:delete", routedMethod: "/grpcbridge.routing.testsvc_%d/MethodOk", routedPattern: "/api/testsvc_%d/action:delete", routedParams: nil, statusCode: codes.OK},
		},
		{
			// All routes of this template should be deleted due to the watcher closing.
			{method: "POST", path: "/grpcbridge.routing.testsvc_%d/DeletedTarget", statusCode: codes.NotFound},
		},
	}

	targets := buildTemplateTargets(N, seed, updateTemplates)
	pr := NewPatternRouter(nilConnPool{}, PatternRouterOpts{})

	watchers := make([]*PatternRouterWatcher, len(targets))
	for i, ti := range targets {
		var err error
		if watchers[i], err = pr.Watch(ti.name); err != nil {
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

			// Additionally separately handle targets with template 4, which should be closed completely.
			if ti.template == 3 {
				watchers[i].Close()
			}
		}()
	}

	wg.Wait()

	// Assert
	// Concurrently validate routing for each target.
	for i, ti := range targets {
		t.Run(fmt.Sprintf("%d-%s", ti.template, ti.name), func(t *testing.T) {
			t.Parallel()

			// Apply templating to each case before actually testing it.
			for _, tt := range templateTests[ti.template] {
				tt.path = fmt.Sprintf(tt.path, i)
				tt.routedMethod = fmt.Sprintf(tt.routedMethod, i)
				tt.routedPattern = fmt.Sprintf(tt.routedPattern, i)

				checkPatternTestCase(t, pr, &tt)
			}
		})
	}
}
