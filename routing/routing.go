package routing

import "github.com/renbou/grpcbridge/grpcadapter"

type ConnPool interface {
	Get(target string) (grpcadapter.ClientConn, bool)
}
