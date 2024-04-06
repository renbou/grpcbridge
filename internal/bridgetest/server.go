package bridgetest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/renbou/grpcbridge/grpcadapter"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const TestTarget = "bridgetest"

func MustGRPCServer(tb testing.TB, prepareFuncs ...func(*grpc.Server)) (*grpc.Server, *grpcadapter.DialedPool, grpcadapter.ClientConn) {
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

	pool := grpcadapter.NewDialedPool(func(ctx context.Context, s string) (*grpc.ClientConn, error) {
		return grpc.DialContext(ctx, s,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
				return listener.Dial()
			}),
		)
	})

	// Timeout just to avoid freezing tests.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	controller := pool.Build(ctx, TestTarget, "bufconn")
	tb.Cleanup(controller.Close) // intentionally added after listener.Close cleanup for the connection to be closed before the listener

	conn, _ := pool.Get(TestTarget)

	return server, pool, conn
}
