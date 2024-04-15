package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/renbou/grpcbridge"
	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := mainImpl(); err != nil {
		os.Exit(1)
	}
}

func mainImpl() error {
	logger := bridgelog.WrapPlainLogger(slog.New(slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{Level: config.LogLevel()},
	)))

	cfg, err := config.Load(logger, os.Args[1:])
	if errors.As(err, new(config.FlagError)) {
		return nil
	} else if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return err
	}

	router := grpcbridge.NewReflectionRouter(
		grpcbridge.WithLogger(logger),
		grpcbridge.WithDialOpts(grpc.WithTransportCredentials(insecure.NewCredentials())),
		grpcbridge.WithReflectionPollInterval(time.Second*5),
	)

	for _, cfg := range cfg.Services {
		if _, err := router.Add(cfg.Name, cfg.Target); err != nil {
			logger.Error("Failed to add service", "service", cfg.Name, "error", err)
			return err
		}
	}

	lis, err := net.Listen("tcp", ":11111")
	if err != nil {
		logger.Error("Failed to listen", "error", err)
		return err
	}

	proxy := grpcbridge.NewGRPCProxy(router, grpcbridge.WithLogger(logger))
	grpcServer := grpc.NewServer(proxy.AsServerOption())

	go func() {
		_ = grpcServer.Serve(lis)
	}()

	bridge := grpcbridge.NewWebBridge(router, grpcbridge.WithLogger(logger))

	httpServer := &http.Server{
		Addr:    ":22222",
		Handler: bridge,
	}

	go func() {
		_ = httpServer.ListenAndServe()
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	<-shutdownCh

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	grpcServer.Stop()
	httpServer.Shutdown(ctx)

	return nil
}
