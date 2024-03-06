package reflection

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/renbou/grpcbridge"
	"github.com/renbou/grpcbridge/discovery"
	"github.com/renbou/grpcbridge/internal/resilience"
	"google.golang.org/grpc"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

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
}

var _ grpcbridge.ResolverBuilder = (*ResolverBuilder)(nil)

func NewResolverBuilder(logger grpcbridge.Logger, opts *ResolverOpts) *ResolverBuilder {
	filledOpts := opts.WithDefaults()

	// additionally ignore gRPC services like reflection, health, channelz, etc.
	filledOpts.IgnorePrefixes = append(filledOpts.IgnorePrefixes, "grpc.")

	return &ResolverBuilder{
		opts:   filledOpts,
		logger: logger.WithComponent("grpcbridge.reflection"),
	}
}

func (rb *ResolverBuilder) Build(sc *grpcbridge.ServiceConfig, cc *grpc.ClientConn, watcher discovery.Watcher) (discovery.Resolver, error) {
	r := &resolver{
		opts:    rb.opts,
		logger:  rb.logger.With("resolver.target", sc.Name),
		cc:      cc,
		watcher: watcher,
		done:    make(chan struct{}),
	}

	go r.watch()

	return r, nil
}

type resolver struct {
	opts    ResolverOpts
	logger  grpcbridge.Logger
	cc      *grpc.ClientConn
	watcher discovery.Watcher
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
			// end watch procedure. no resources apart from the ticker need cleanup.
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
func (r *resolver) resolve() (*discovery.State, error) {
	// Add explicit cancelations to each call in order to avoid blocking indefinitely,
	// since setting a single timeout for the whole stream isn't adequate in this case,
	// as different services can take very different times to fully return all the reflection info.
	ctx, canceler := resilience.ContextWithCanceler(context.Background(), r.opts.ReqTimeout)
	defer canceler.Cancel()

	client, err := connectClient(ctx, canceler, r.cc, reflectionpb.ServerReflection_ServerReflectionInfo_FullMethodName)
	if err != nil {
		return nil, err
	}

	services, err := r.listServiceNames(client)
	if err != nil {
		return nil, err
	}

	// TODO(renbou): discover methods, messages, etc for each service.
	return &discovery.State{Services: services}, nil
}

func (r *resolver) listServiceNames(c *client) ([]discovery.ServiceDesc, error) {
	services, err := c.listServices()
	if err != nil {
		return nil, err
	}

	// Avoid raising an error when an invalid service name is encountered,
	// simply ignoring it is better than completely stopping metadata discovery.
	filtered := make([]discovery.ServiceDesc, 0, len(services))
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
			filtered = append(filtered, discovery.ServiceDesc{Name: fullName})
		}
	}

	return filtered, nil
}

type client struct {
	// ClientStream instead of the properly-typed ServerReflection_ServerReflectionInfoClient
	// because both v1 and v1alpha are exactly the same, so messages for them can be interchanged.
	stream   grpc.ClientStream
	canceler *resilience.ContextCanceler
}

func connectClient(ctx context.Context, canceler *resilience.ContextCanceler, cc *grpc.ClientConn, method string) (*client, error) {
	desc := &grpc.StreamDesc{
		StreamName:    "ServerReflectionInfo",
		ClientStreams: true,
		ServerStreams: true,
	}

	stream, err := cc.NewStream(ctx, desc, method)
	if err != nil {
		return nil, fmt.Errorf("establishing reflection stream with method %q: %w", method, err)
	}

	// Stop() called here and after further calls to avoid canceling the context while data is being processed.
	canceler.Stop()

	return &client{stream: stream, canceler: canceler}, nil
}

// listServices executes the ListServices reflection request using a single timeout for both request and response.
// it expects that the response is of type ListServiceResponse, so it should be used once at the start and not
// reused alongside other requests.
func (c *client) listServices() ([]string, error) {
	c.canceler.Reset()

	// NB: if this returns an error, the stream is successfully closed.
	if err := c.stream.SendMsg(&reflectionpb.ServerReflectionRequest{
		MessageRequest: &reflectionpb.ServerReflectionRequest_ListServices{},
	}); err != nil {
		return nil, fmt.Errorf("sending ListServices request: %w", err)
	}

	m := new(reflectionpb.ServerReflectionResponse)
	if err := c.stream.RecvMsg(m); err != nil {
		return nil, fmt.Errorf("receiving response to ListServices request: %w", err)
	}

	c.canceler.Stop()

	// the client doesn't do any processing, so just return the names as is.
	services := m.GetListServicesResponse().GetService()
	serviceNames := make([]string, len(services))

	for i, s := range services {
		serviceNames[i] = s.GetName()
	}

	return serviceNames, nil
}
