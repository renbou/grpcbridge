package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/renbou/grpcbridge/internal/config"
)

func main() {
	if err := mainImpl(context.Background()); err != nil {
		os.Exit(1)
	}
}

func mainImpl(ctx context.Context) error {
	logger := slog.New(slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{Level: config.LogLevel()},
	))

	cfg, err := config.Load(ctx, logger, os.Args[1:])
	if errors.As(err, new(config.FlagError)) {
		return nil
	} else if err != nil {
		logger.ErrorContext(ctx, "Failed to load configuration", "error", err)
		return err
	}

	_ = cfg

	return nil
}
