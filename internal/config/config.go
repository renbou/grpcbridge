package config

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/renbou/grpcbridge"
)

// FlagError is a special type marking that the error is a result of a flag parsing error.
type FlagError struct {
	error
}

const envPrefix = "GRPCBRIDGE_"

//go:generate stringer -type=discoveryOrigin -trimprefix=discoveryOrigin
type discoveryOrigin int

const (
	discoveryOriginNone discoveryOrigin = iota
	discoveryOriginEnv
	discoveryOriginAuto
)

func discoverPath() (string, discoveryOrigin, bool) {
	if value, ok := os.LookupEnv(envPrefix + "CONFIG"); ok {
		return value, discoveryOriginEnv, true
	}

	for _, filename := range []string{"grpcbridge.hcl", "grpcbridge.json", "config.hcl", "config.json"} {
		if _, err := os.Stat(filename); err == nil {
			return filename, discoveryOriginAuto, true
		}
	}

	return "", discoveryOriginNone, false
}

func LogLevel() slog.Level {
	levelMapping := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}

	if level, ok := levelMapping[os.Getenv(envPrefix+"LOG_LEVEL")]; ok {
		return level
	}

	return slog.LevelInfo
}

func Load(ctx context.Context, logger grpcbridge.Logger, args []string) (*grpcbridge.Config, error) {
	var configPath string

	// Try to get config path from the command line for easy route.
	fs := flag.NewFlagSet("grpcbridge", flag.ContinueOnError)
	fs.Func("config", "Manually specified path to the config file. By default, the config is autodiscovered.", func(s string) error {
		if _, err := os.Stat(s); err != nil {
			return fmt.Errorf("config file %q does not exist or is inaccessible: %s", s, err)
		}

		configPath = s

		return nil
	})

	if err := fs.Parse(args); err != nil {
		return nil, FlagError{fmt.Errorf("parsing flags: %w", err)}
	}

	// Otherwise, perform discovery.
	if configPath == "" {
		discovered, origin, ok := discoverPath()
		if !ok {
			logger.WarnContext(ctx, "No config file found, and no --config flag was provided. Will use default configuration.")
		} else {
			configPath = discovered
			logger.InfoContext(ctx, fmt.Sprintf("Using discovered config file %q", configPath), "discovery_origin", origin)
		}
	}

	// Finally, return the default config, or read the config from the file.
	if configPath == "" {
		return new(grpcbridge.Config), nil
	}

	return ReadHCL(configPath)
}
