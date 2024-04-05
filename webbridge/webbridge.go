package webbridge

import (
	"net/http"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/grpcadapter"
)

type HTTPRouter interface {
	RouteHTTP(*http.Request) (grpcadapter.ClientConn, *bridgedesc.Method, error)
}
