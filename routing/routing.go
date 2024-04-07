package routing

import (
	"errors"

	"github.com/renbou/grpcbridge/grpcadapter"
)

type ConnPool interface {
	Get(target string) (grpcadapter.ClientConn, bool)
}

// ErrAlreadyWatching is returned by the Watch() method on routers when a watcher already exists
// for the specified target and a new one should not be created without closing the previous one first.
var ErrAlreadyWatching = errors.New("watcher already exists for target")
