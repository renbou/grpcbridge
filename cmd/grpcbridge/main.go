package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/renbou/grpcbridge"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/config"
	"github.com/renbou/grpcbridge/reflection"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := mainImpl(); err != nil {
		os.Exit(1)
	}
}

type loggingWatcher struct {
	logger grpcbridge.Logger
}

func (lw *loggingWatcher) UpdateState(state *reflection.DiscoveryState) {
	lw.logger.Info("State updated", "state", state)
}

func (lw *loggingWatcher) ReportError(err error) {
	lw.logger.Error("Error reported", "error", err)
}

func mainImpl() error {
	logger := grpcbridge.WrapPlainLogger(slog.New(slog.NewJSONHandler(
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

	connPool := grpcadapter.NewClientConnPool(func(ctx context.Context, s string) (*grpc.ClientConn, error) {
		return grpc.DialContext(ctx, s, grpc.WithTransportCredentials(insecure.NewCredentials()))
	})

	resolverBuilder := reflection.NewResolverBuilder(logger, connPool, &reflection.ResolverOpts{
		PollInterval: time.Second * 5,
	})

	for _, cfg := range cfg.Services {
		_ = connPool.Build(context.Background(), cfg.Name, cfg.Target)

		_, err = resolverBuilder.Build(cfg.Name, &loggingWatcher{logger: logger})
		if err != nil {
			logger.Error("Failed to build resolver", "error", err)
			return err
		}
	}

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	<-shutdownCh

	return nil
}
