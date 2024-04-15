package grpcbridge

import (
	"log/slog"

	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/routing"
)

var _ = slog.Logger{}

// Option configures common grpcbridge options, such as the logger.
type Option interface {
	RouterOption
}

type Router interface {
	routing.GRPCRouter
	routing.HTTPRouter
}

type WebBridge struct{}

// WithLogger configures the logger to be used by grpcbridge components. By default all logs are discarded.
//
// Taking the full Logger interface allows you to configure all functionality however you want,
// however you can also use [bridgelog.WrapPlainLogger] to wrap a basic logger such as [slog.Logger].
func WithLogger(logger bridgelog.Logger) Option {
	return newFuncOption(func(o *options) {
		o.logger = logger
	})
}

type options struct {
	logger bridgelog.Logger
}

func defaultOptions() options {
	return options{logger: bridgelog.Discard()}
}

type funcOption struct {
	f func(*options)
}

func (f *funcOption) applyRouter(o *routerOptions) {
	f.f(&o.common)
}

func newFuncOption(f func(*options)) Option {
	return &funcOption{f: f}
}
