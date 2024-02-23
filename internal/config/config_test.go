package config

import (
	"log/slog"
	"os"
	"testing"
)

// chdir from https://github.com/golang/go/issues/45182, while t.Chdir isn't made public
func chdir(t *testing.T, dir string) func() {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting current working directory prior to test execution: %s", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("changing directory to %s: %s", dir, err)
	}

	return func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restoring working directory to %s: %s", wd, err)
		}
	}
}

// Test_discoverPath runs without t.Parallel because it would interfere
// with other tests when changing directories.
func Test_discoverPath(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		files []string

		discovered string
		origin     discoveryOrigin
		ok         bool
	}{
		{
			name: "env",
			env: map[string]string{
				"GRPCBRIDGE_CONFIG": "config.hcl",
			},
			discovered: "config.hcl",
			origin:     discoveryOriginEnv,
			ok:         true,
		},
		{
			name:       "grpcbridge.hcl",
			files:      []string{"grpcbridge.hcl"},
			discovered: "grpcbridge.hcl",
			origin:     discoveryOriginAuto,
			ok:         true,
		},
		{
			name:       "grpcbridge.json",
			files:      []string{"grpcbridge.json"},
			discovered: "grpcbridge.json",
			origin:     discoveryOriginAuto,
			ok:         true,
		},
		{
			name:       "config.hcl",
			files:      []string{"config.hcl"},
			discovered: "config.hcl",
			origin:     discoveryOriginAuto,
			ok:         true,
		},
		{
			name:       "config.json",
			files:      []string{"config.json"},
			discovered: "config.json",
			origin:     discoveryOriginAuto,
			ok:         true,
		},
		{
			name:       "prefer specific hcl",
			files:      []string{"grpcbridge.hcl", "config.hcl"},
			discovered: "grpcbridge.hcl",
			origin:     discoveryOriginAuto,
			ok:         true,
		},
		{
			name:       "prefer specific json",
			files:      []string{"grpcbridge.json", "config.json"},
			discovered: "grpcbridge.json",
			origin:     discoveryOriginAuto,
			ok:         true,
		},
		{
			name: "prefer env",
			env: map[string]string{
				"GRPCBRIDGE_CONFIG": "config.hcl",
			},
			files:      []string{"grpcbridge.hcl"},
			discovered: "config.hcl",
			origin:     discoveryOriginEnv,
			ok:         true,
		},
		{
			name:   "fail",
			origin: discoveryOriginNone,
			ok:     false,
		},
	}

	for _, tt := range tests {
		// Subtests also run without t.Parallel for the same reason as the main test
		t.Run(tt.name, func(t *testing.T) {
			defer chdir(t, t.TempDir())()

			// Prepare environment variables and files for this single test.
			// The files will be automatically cleared thanks to t.TempDir.
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			for _, filename := range tt.files {
				f, err := os.Create(filename)
				if err != nil {
					t.Fatalf("creating test file %s: %s", filename, err)
				}

				_ = f.Close()
			}

			discovered, origin, ok := discoverPath()

			if discovered != tt.discovered {
				t.Errorf("DiscoverConfig() returned config file = %q, want %q", discovered, tt.discovered)
			}

			if origin != tt.origin {
				t.Errorf("DiscoverConfig() returned origin = %s, want %s", origin, tt.origin)
			}

			if ok != tt.ok {
				t.Errorf("DiscoverConfig() returned ok = %t, want %t", ok, tt.ok)
			}
		})
	}
}

// Test_logLevel runs without t.Parallel because it would interfere
// with other tests when changing environment variables.
func Test_logLevel(t *testing.T) {
	tests := []struct {
		value     string
		wantLevel slog.Level
	}{
		{
			value:     "",
			wantLevel: slog.LevelInfo,
		},
		{
			value:     "debug",
			wantLevel: slog.LevelDebug,
		},
		{
			value:     "info",
			wantLevel: slog.LevelInfo,
		},
		{
			value:     "warn",
			wantLevel: slog.LevelWarn,
		},
		{
			value:     "error",
			wantLevel: slog.LevelError,
		},
		{
			value:     "invalid",
			wantLevel: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		// Subtests also run without t.Parallel for the same reason as the main test.
		t.Run(tt.value, func(t *testing.T) {
			// Arrange
			t.Setenv(envPrefix+"LOG_LEVEL", tt.value)

			// Act
			level := LogLevel()

			// Assert
			if level != tt.wantLevel {
				t.Errorf("LogLevel() = %v, want %v", level, tt.wantLevel)
			}
		})
	}
}
