package bridgetest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/renbou/grpcbridge/grpcadapter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const (
	TestTargetName = "bridgetest"
	TestDialTarget = "passthrough:///bridgetest"
)

func MustGRPCServer(tb testing.TB, prepareFuncs ...func(*grpc.Server)) (*grpc.Server, *grpcadapter.AdaptedClientPool, grpcadapter.ClientConn) {
	// Relatively big buffer to allow all test goroutines to communicate without blocking.
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()

	for _, prepare := range prepareFuncs {
		prepare(server)
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			panic(fmt.Sprintf("failed to serve test gRPC server: %s", err))
		}
	}()

	tb.Cleanup(func() {
		if err := listener.Close(); err != nil {
			tb.Errorf("unexpected error while closing bufconn.Listener: %s", err)
		}
	})

	pool := grpcadapter.NewAdaptedClientPool(grpcadapter.AdaptedClientPoolOpts{
		DefaultOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
				return listener.Dial()
			}),
		},
	})

	controller, _ := pool.New(TestTargetName, TestDialTarget)
	tb.Cleanup(controller.Close) // intentionally added after listener.Close cleanup for the connection to be closed before the listener

	conn, _ := pool.Get(TestTargetName)

	return server, pool, conn
}
