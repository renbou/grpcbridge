package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	gotestapiv1 "gotestapi/gen/proto/gotestapi/v1"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(logging.UnaryServerInterceptor(gRPCLogger)),
		grpc.ChainStreamInterceptor(logging.StreamServerInterceptor(gRPCLogger)),
	)

	service := new(veggieShopService)
	go service.cleaner()

	// Register only v1 gRPC reflection. grpcbridge should properly handle such cases.
	gotestapiv1.RegisterVeggieShopServiceServer(server, service)
	reflection.RegisterV1(server)

	go func() {
		slog.Info("serving test gRPC", "listen_addr", listenAddr)
		if err := server.Serve(lis); err != nil {
			fatalf("failed to serve", err)
		}
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	<-shutdownCh

	slog.Warn("received termination signal, performing graceful shutdown")

	// Initiate graceful shutdown in separate goroutine, then,
	// after a timeout, force the server stop.
	gracefulCh := make(chan struct{})
	go func() {
		defer close(gracefulCh)
		server.GracefulStop()
	}()

	select {
	case <-time.After(time.Second * 3):
	case <-gracefulCh:
	}
	server.Stop()
}

func fatalf(msg string, err error) {
	slog.Error(fmt.Sprintf("%s: %s", msg, err))
	os.Exit(1)
}
