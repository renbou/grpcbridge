package grpcbridge

import (
	"context"

	"github.com/renbou/grpcbridge/grpcadapter"
)

type ForwarderOption interface {
	applyForwarder(o *forwarderOptions)
}

type Forwarder struct {
	pf *grpcadapter.ProxyForwarder
}

func NewForwarder(opts ...ForwarderOption) *Forwarder {
	options := defaultForwarderOptions()

	for _, opt := range opts {
		opt.applyForwarder(&options)
	}

	filter := grpcadapter.NewProxyMDFilter(options.mdFilterOpts)
	pf := grpcadapter.NewProxyForwarder(grpcadapter.ProxyForwarderOpts{
		Filter: filter,
	})

	return &Forwarder{pf: pf}
}

func (f *Forwarder) Forward(ctx context.Context, params grpcadapter.ForwardParams) error {
	return f.pf.Forward(ctx, params)
}

type forwarderOptions struct {
	common       options
	mdFilterOpts grpcadapter.ProxyMDFilterOpts
}

func defaultForwarderOptions() forwarderOptions {
	return forwarderOptions{common: defaultOptions()}
}
