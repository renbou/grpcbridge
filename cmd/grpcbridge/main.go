package main

import (
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/renbou/grpcbridge"
	"github.com/renbou/grpcbridge/discovery"
	"github.com/renbou/grpcbridge/discovery/reflection"
	"github.com/renbou/grpcbridge/internal/config"
	"google.golang.org/grpc"
)

func main() {
	if err := mainImpl(); err != nil {
		os.Exit(1)
	}
}

type loggingWatcher struct {
	logger grpcbridge.Logger
}

func (lw *loggingWatcher) UpdateState(state *discovery.State) {
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

	resolverBuilder := reflection.NewResolverBuilder(logger, &reflection.ResolverOpts{
		PollInterval: time.Second * 5,
	})

	for _, cfg := range cfg.Services {
		cc, err := grpc.Dial(cfg.Target, grpc.WithInsecure())
		if err != nil {
			logger.Error("Failed to dial target", "target", cfg.Target, "error", err)
			return err
		}

		_, err = resolverBuilder.Build(&cfg, cc, &loggingWatcher{logger: logger})
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
