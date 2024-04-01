package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
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

	connPool := grpcadapter.NewDialedPool(func(ctx context.Context, s string) (*grpc.ClientConn, error) {
		return grpc.DialContext(ctx, s, grpc.WithTransportCredentials(insecure.NewCredentials()))
	})

	resolverBuilder := reflection.NewResolverBuilder(connPool, reflection.ResolverOpts{
		Logger:       logger,
		PollInterval: time.Second * 5,
	})

	router := routing.NewServiceRouter(connPool, routing.ServiceRouterOpts{Logger: logger})

	grpcProxy := grpcbridge.NewGRPCProxy(router, grpcbridge.GPRCProxyOpts{Logger: logger})

	for _, cfg := range cfg.Services {
		_ = connPool.Build(context.Background(), cfg.Name, cfg.Target)

		lw := &loggingWatcher{logger: logger}
		rw := router.Watcher(cfg.Name)

		_, err = resolverBuilder.Build(cfg.Name, &aggregateWatcher{watchers: []reflection.Watcher{lw, rw}})
		if err != nil {
			logger.Error("Failed to build resolver", "error", err)
			return err
		}
	}

	lis, err := net.Listen("tcp", ":11111")
	if err != nil {
		logger.Error("Failed to listen", "error", err)
		return err
	}

	grpcServer := grpc.NewServer(grpcProxy.AsOption())

	go func() {
		_ = grpcServer.Serve(lis)
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	<-shutdownCh

	grpcServer.Stop()

	return nil
}
