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
	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/config"
	"github.com/renbou/grpcbridge/reflection"
	"github.com/renbou/grpcbridge/routing"
	"github.com/renbou/grpcbridge/transcoding"
	"github.com/renbou/grpcbridge/webbridge"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := mainImpl(); err != nil {
		os.Exit(1)
	}
}

type aggregateWatcher struct {
	watchers []reflection.Watcher
}

func (aw *aggregateWatcher) UpdateDesc(state *bridgedesc.Target) {
	for _, w := range aw.watchers {
		w.UpdateDesc(state)
	}
}

func (aw *aggregateWatcher) ReportError(err error) {
	for _, w := range aw.watchers {
		w.ReportError(err)
	}
}

type loggingWatcher struct {
	logger bridgelog.Logger
}

func (lw *loggingWatcher) UpdateDesc(desc *bridgedesc.Target) {
	lw.logger.Info("Target description updated", "state", desc)
}

func (lw *loggingWatcher) ReportError(err error) {}

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

	connPool := grpcadapter.NewAdaptedClientPool(grpcadapter.AdaptedClientPoolOpts{
		DefaultOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	})

	resolverBuilder := reflection.NewResolverBuilder(connPool, reflection.ResolverOpts{
		Logger:       logger,
		PollInterval: time.Second * 5,
	})

	grpcRouter := routing.NewServiceRouter(connPool, routing.ServiceRouterOpts{Logger: logger})
	httpRouter := routing.NewPatternRouter(connPool, routing.PatternRouterOpts{Logger: logger})
	transcoder := transcoding.NewStandardTranscoder(transcoding.StandardTranscoderOpts{})

	grpcProxy := grpcbridge.NewGRPCProxy(grpcRouter, grpcbridge.GPRCProxyOpts{Logger: logger})
	httpBridge := webbridge.NewHTTPTranscodedBridge(httpRouter, transcoder)

	for _, cfg := range cfg.Services {
		_, _ = connPool.New(cfg.Name, cfg.Target)

		lw := &loggingWatcher{logger: logger}
		gw, err := grpcRouter.Watch(cfg.Name)
		if err != nil {
			logger.Error("failed to create watcher for grpc router", "error", err)
			return err
		}

		hw, err := httpRouter.Watch(cfg.Name)
		if err != nil {
			logger.Error("failed to create watcher for http router", "error", err)
			return err
		}

		_ = resolverBuilder.Build(cfg.Name, &aggregateWatcher{watchers: []reflection.Watcher{lw, gw, hw}})
	}

	lis, err := net.Listen("tcp", ":11111")
	if err != nil {
		logger.Error("Failed to listen", "error", err)
		return err
	}

	grpcServer := grpc.NewServer(grpcProxy.AsServerOption())

	go func() {
		_ = grpcServer.Serve(lis)
	}()

	httpServer := &http.Server{
		Addr:    ":22222",
		Handler: httpBridge,
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
