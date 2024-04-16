package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	gotestapiv1 "gotestapi/gen/proto/gotestapi/v1"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/renbou/grpcbridge"
	"github.com/renbou/grpcbridge/bridgelog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/test/bufconn"
)

func main() {
	logLevel := slog.LevelInfo
	if strings.ToLower(os.Getenv("LOG_LEVEL")) == "debug" {
		logLevel = slog.LevelDebug
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})).With("app", "gotestapi"))

	listenAddr := ":50051"
	if value, ok := os.LookupEnv("LISTEN_ADDR"); ok {
		listenAddr = value
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fatalf("failed to listen", err)
	}

	gRPCLogger := logging.LoggerFunc(func(ctx context.Context, level logging.Level, msg string, fields ...any) {
		// slog.Level conversion here is ok because logging.Level is defined with the same constants
		slog.Log(ctx, slog.Level(level), msg, fields...)
	})

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(logging.UnaryServerInterceptor(gRPCLogger)),
		grpc.ChainStreamInterceptor(logging.StreamServerInterceptor(gRPCLogger)),
	)

	service := new(veggieShopService)
	go service.cleaner()

	// Register only v1 gRPC reflection. grpcbridge should properly handle such cases.
	gotestapiv1.RegisterVeggieShopServiceServer(grpcServer, service)
	reflection.RegisterV1(grpcServer)

	// Embedded grpcbridge support via bufconn.
	// Currently still requires the reflection server to be present, in the future will support grpc.ServiceRegistrar.
	bridge := setupBridge(grpcServer)

	// TODO(renbou): use cmux once implemented in grpcbridge.
	httpServer := &http.Server{
		Addr:    ":50080",
		Handler: bridge,
	}

	go func() {
		slog.Info("serving test HTTP bridge", "listen_addr", ":50080")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatalf("failed to serve HTTP server on network", err)
		}
	}()

	go func() {
		slog.Info("serving test gRPC", "listen_addr", listenAddr)
		if err := grpcServer.Serve(lis); err != nil {
			fatalf("failed to serve gRPC server on network", err)
		}
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	<-shutdownCh

	slog.Warn("received termination signal, performing graceful shutdown")

	// Initiate graceful shutdown in separate goroutine, then,
	// after a timeout, force the server stop.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	gracefulCh := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		grpcServer.GracefulStop()
	}()

	go func() {
		defer wg.Done()
		httpServer.Shutdown(ctx)
	}()

	go func() {
		wg.Wait()
		defer close(gracefulCh)
	}()

	select {
	case <-ctx.Done():
	case <-gracefulCh:
	}
	grpcServer.Stop()
}

func setupBridge(server *grpc.Server) *grpcbridge.WebBridge {
	buf := bufconn.Listen(1 << 20)
	bridgeLogger := bridgelog.WrapPlainLogger(slog.Default())

	router := grpcbridge.NewReflectionRouter(
		grpcbridge.WithLogger(bridgeLogger),
		grpcbridge.WithDialOpts(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return buf.DialContext(ctx)
			}),
		),
		// Disable polling because our protos never change during runtime.
		grpcbridge.WithDisabledReflectionPolling(),
	)

	go func() {
		if err := server.Serve(buf); err != nil {
			fatalf("failed to serve gRPC server on local bufconn", err)
		}
	}()

	router.Add("gotestapi", "passthrough:///bufconn")

	return grpcbridge.NewWebBridge(router, grpcbridge.WithLogger(bridgeLogger))
}

func fatalf(msg string, err error) {
	slog.Error(fmt.Sprintf("%s: %s", msg, err))
	os.Exit(1)
}
