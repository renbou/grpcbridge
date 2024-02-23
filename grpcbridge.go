package grpcbridge

import "context"

type Logger interface {
	DebugContext(context.Context, string, ...any)
	InfoContext(context.Context, string, ...any)
	WarnContext(context.Context, string, ...any)
	ErrorContext(context.Context, string, ...any)
}

type Config struct {
	Services map[string]ServiceConfig
}

type ServiceConfig struct {
	Target string
}
