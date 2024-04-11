// Package routing provides routers for handling gRPC and HTTP requests in grpcbridge.
// Currently, two routers are present with pretty different logic in regard to routing:
//
//  1. [ServiceRouter], which can route gRPC and HTTP requests according to the path definition in gRPC's [PROTOCOL-HTTP2] spec,
//     and only requires information about the target's available service names, not complete method and binding definitions.
//  2. [PatternRouter], which can route HTTP requests according to the path templates defined in gRPC Transcoding's [http.proto],
//     but requires full method and binding definitions for each of a target's services.
//     It is compatible with the routing used in the [gRPC-Gateway] apart from slight security-related changes.
//
// Both of these routers are used by grpcbridge itself to optimize and perform routing in different use-cases,
// such as HTTP-to-gRPC transcoding, gRPC proxying, gRPC-Web bridging, etc.
//
// [PROTOCOL-HTTP2]: https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests
// [http.proto]: https://github.com/googleapis/googleapis/blob/master/google/api/http.proto
// [gRPC-Gateway]: https://github.com/grpc-ecosystem/grpc-gateway
package routing

import (
	"errors"

	"github.com/renbou/grpcbridge/bridgedesc"
)

// GRPCRoute contains the matched route information for a single gRPC request, returned by the RouteGRPC method of the routers.
type GRPCRoute struct {
	Target  *bridgedesc.Target
	Service *bridgedesc.Service
	Method  *bridgedesc.Method
}

// HTTPRoute contains the matched route information for a single specific HTTP request, returned by the RouteHTTP method of the routers.
type HTTPRoute struct {
	Target  *bridgedesc.Target
	Service *bridgedesc.Service
	Method  *bridgedesc.Method
	Binding *bridgedesc.Binding
	// Matched, URL-decoded path parameters defined by the binding pattern.
	// See https://github.com/googleapis/googleapis/blob/e0677a395947c2f3f3411d7202a6868a7b069a41/google/api/http.proto#L295
	// for information about how exactly different kinds of parameters are decoded.
	PathParams map[string]string
}

// ErrAlreadyWatching is returned by the Watch() method on routers when a watcher already exists
// for the specified target and a new one should not be created without closing the previous one first.
var ErrAlreadyWatching = errors.New("target already being watched")

type httpStatusError struct {
	code int
	err  error
}

func (e *httpStatusError) Error() string {
	return e.err.Error()
}

func (e *httpStatusError) Unwrap() error {
	return e.err
}

func (e *httpStatusError) HTTPStatus() int {
	return e.code
}
