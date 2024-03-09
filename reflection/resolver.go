package reflection

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/renbou/grpcbridge"
	"github.com/renbou/grpcbridge/grpcadapter"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type DiscoveryWatcher interface {
	UpdateState(*DiscoveryState)
	ReportError(error)
}

type DiscoveryState struct {
	Services []ServiceDesc
}

type ServiceDesc struct {
	Name protoreflect.FullName
}

type ResolverOpts struct {
	PollInterval time.Duration
	ReqTimeout   time.Duration
	// Prefixes of service names to ignore additionally to administrative gRPC services (grpc.health, grpc.reflection, grpc.channelz, ...).
	IgnorePrefixes []string
}

func (opts *ResolverOpts) WithDefaults() ResolverOpts {
	if opts == nil {
		return DefaultResolverOpts
	}

	filled := *opts

	if opts.PollInterval == 0 {
		filled.PollInterval = DefaultResolverOpts.PollInterval
	}

	if opts.ReqTimeout == 0 {
		filled.ReqTimeout = DefaultResolverOpts.ReqTimeout
	}

	return filled
}

var DefaultResolverOpts = ResolverOpts{
	PollInterval:   5 * time.Minute,
	ReqTimeout:     10 * time.Second,
	IgnorePrefixes: []string{},
}

type ResolverBuilder struct {
	opts   ResolverOpts
	logger grpcbridge.Logger
	pool   *grpcadapter.ClientConnPool
}

func NewResolverBuilder(logger grpcbridge.Logger, pool *grpcadapter.ClientConnPool, opts *ResolverOpts) *ResolverBuilder {
	filledOpts := opts.WithDefaults()

	// additionally ignore gRPC services like reflection, health, channelz, etc.
	filledOpts.IgnorePrefixes = append(filledOpts.IgnorePrefixes, "grpc.")

	return &ResolverBuilder{
		opts:   filledOpts,
		pool:   pool,
		logger: logger.WithComponent("grpcbridge.reflection"),
	}
}

func (rb *ResolverBuilder) Build(name string, watcher DiscoveryWatcher) (*resolver, error) {
	r := &resolver{
		opts:    rb.opts,
		name:    name,
		logger:  rb.logger.With("resolver.target", name),
		pool:    rb.pool,
		watcher: watcher,
		done:    make(chan struct{}),
	}

	go r.watch()

	return r, nil
}

type resolver struct {
	opts    ResolverOpts
	name    string
	logger  grpcbridge.Logger
	pool    *grpcadapter.ClientConnPool
	watcher DiscoveryWatcher
	done    chan struct{}
}

func (r *resolver) watch() {
	for {
		state, err := r.resolve()
		if err == nil {
			r.watcher.UpdateState(state)
		} else {
			r.watcher.ReportError(err)
			r.logger.Error("Resolution unrecoverably failed, will retry again later", "error", err)
		}

		select {
		// time.Ticker isn't used because we need the pollInterval to be *in between* polls,
		// not to actually resolve once every pollInterval, which might
		// lead to resolve() being called straight after the last one returning.
		case <-time.After(r.opts.PollInterval):
		case <-r.done:
			return
		}
	}
}

func (r *resolver) Close() {
	close(r.done)
}

// TODO(renbou): support v1alpha and v1 at the same time, using some mechanism like this:
// - try v1
// - if it fails, try v1alpha
// - if it works, then remember this and use it the next time
// - if it fails, return an error
// Then if v1alpha starts failing, try switching to v1, and repeat.
// NB: "fails" here means any error, not just "Unimplemented", because servers might misbehave in various ways.
func (r *resolver) resolve() (*DiscoveryState, error) {
	cc := r.pool.Get(r.name)
	if cc == nil {
		return nil, fmt.Errorf("no connection available in pool for target %q", r.name)
	}

	client, err := connectClient(r.opts.ReqTimeout, cc, reflectionpb.ServerReflection_ServerReflectionInfo_FullMethodName)
	if err != nil {
		return nil, err
	}

	defer client.close()

	services, err := r.listServiceNames(client)
	if err != nil {
		return nil, err
	}

	// TODO(renbou): discover methods, messages, etc for each service.
	return &DiscoveryState{Services: services}, nil
}

func (r *resolver) listServiceNames(c *client) ([]ServiceDesc, error) {
	services, err := c.listServices()
	if err != nil {
		return nil, err
	}

	// Avoid raising an error when an invalid service name is encountered,
	// simply ignoring it is better than completely stopping metadata discovery.
	filtered := make([]ServiceDesc, 0, len(services))
	for _, s := range services {
		fullName := protoreflect.FullName(s)
		if !fullName.IsValid() {
			r.logger.Warn("Resolver received invalid gRPC service name", "service", s)
			continue
		}

		index := slices.IndexFunc(r.opts.IgnorePrefixes, func(prefix string) bool {
			return strings.HasPrefix(s, prefix)
		})

		if index == -1 {
			filtered = append(filtered, ServiceDesc{Name: fullName})
		}
	}

	return filtered, nil
}
