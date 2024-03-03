package grpcbridge

import (
	"github.com/renbou/grpcbridge/discovery"
	"google.golang.org/grpc"
)

type PlainLogger interface {
	Debug(string, ...any)
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}

type Logger interface {
	PlainLogger
	With(args ...any) Logger
	WithComponent(string) Logger
}

// WrapPlainLogger wraps a logger which implements at least PlainLogger with extra methods needed to implement Logger.
// If the logger provides a method, it will be used instead of the wrapper's.
//
// It is generically typed to allow calling Logger-like methods on the wrapped logger
// which return T instead of the interface, which wouldn't work due to Go's type system:
//
//	if impl, ok := logger.(interface{ WithComponent(string) T }); ok {
//		return WrapPlainLogger(impl.WithComponent(component))
//	} else if impl, ok := logger.(interface{ WithComponent(string) PlainLogger }); ok {
//		return impl.WithComponent(component)
//	}
func WrapPlainLogger[T PlainLogger](logger T) Logger {
	return &wrappedLogger[T]{underlying: logger}
}

type Config struct {
	Services map[string]ServiceConfig
}

type ServiceConfig struct {
	Name   string
	Target string
}

type ResolverBuilder interface {
	Build(*ServiceConfig, *grpc.ClientConn, discovery.Watcher) (discovery.Resolver, error)
}

type wrappedLogger[T PlainLogger] struct {
	underlying PlainLogger
	component  string
	args       []any
}

func (wl *wrappedLogger[T]) formatArgs(args []any) []any {
	finalLen := len(args)

	if wl.component != "" {
		finalLen += 2
	}

	if len(wl.args) > 0 {
		finalLen += len(wl.args)
	}

	if finalLen == len(args) {
		return args
	}

	newArgs := append(make([]any, 0, finalLen), args...)

	if wl.component != "" {
		newArgs = append(newArgs, "component", wl.component)
	}

	if len(wl.args) > 0 {
		newArgs = append(newArgs, wl.args...)
	}

	return newArgs
}

func (wl *wrappedLogger[T]) WithComponent(component string) Logger {
	cp := *wl

	if impl, ok := wl.underlying.(interface{ WithComponent(string) T }); ok {
		cp.underlying = impl.WithComponent(component)
	} else if impl, ok := wl.underlying.(interface{ WithComponent(string) PlainLogger }); ok {
		cp.underlying = impl.WithComponent(component)
	} else {
		cp.component = component
	}

	return &cp
}

func (wl *wrappedLogger[T]) With(args ...any) Logger {
	cp := *wl

	if impl, ok := wl.underlying.(interface{ With(args ...any) T }); ok {
		cp.underlying = impl.With(args...)
	} else if impl, ok := wl.underlying.(interface{ With(args ...any) PlainLogger }); ok {
		cp.underlying = impl.With(args...)
	} else {
		cp.args = append(wl.args, args...)
	}

	return &cp
}

func (wl *wrappedLogger[T]) Debug(format string, args ...any) {
	wl.underlying.Debug(format, wl.formatArgs(args)...)
}

func (wl *wrappedLogger[T]) Info(format string, args ...any) {
	wl.underlying.Info(format, wl.formatArgs(args)...)
}

func (wl *wrappedLogger[T]) Warn(format string, args ...any) {
	wl.underlying.Warn(format, wl.formatArgs(args)...)
}

func (wl *wrappedLogger[T]) Error(format string, args ...any) {
	wl.underlying.Error(format, wl.formatArgs(args)...)
}
