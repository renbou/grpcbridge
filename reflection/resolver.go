// Package reflection provides a way to receive complete service descriptions via gRPC reflection suitable for use with grpcbridge's routers.
// For more information regarding the gRPC reflection protocol, see the official "[GRPC Server Reflection Protocol]" spec.
//
// Both v1 and v1alpha reflection versions are supported,
// with the resolver being able to switch between the two and remembering the previously used version.
// See [Resolver] and [ResolverOpts] for more details and the available features.
//
// [GRPC Server Reflection Protocol]: https://github.com/grpc/grpc/blob/master/doc/server-reflection.md
package reflection

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/descparse"
	"google.golang.org/grpc/codes"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	reflectionalphapb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

var reflectionMethods = []string{
	reflectionpb.ServerReflection_ServerReflectionInfo_FullMethodName,      // v1
	reflectionalphapb.ServerReflection_ServerReflectionInfo_FullMethodName, // v1alpha
}

// ConnPool is implemented by [*grpcadapter.DialedPool] and is used by ResolverBuilder to retrieve connections.
type ConnPool interface {
	Get(target string) (grpcadapter.ClientConn, bool)
}

// Watcher defines the interface of a watcher which is notified by the resolver of updates.
type Watcher interface {
	// UpdateDesc is called when the resolver has successfully received new descriptions.
	UpdateDesc(*bridgedesc.Target)
	// ReportError is called when the resolver encounters an error, it can be safely ignored by the watcher.
	ReportError(error)
}

// ResolverOpts define all the optional settings which can be set for [Resolver].
type ResolverOpts struct {
	// Logs are discarded by default.
	Logger bridgelog.Logger
	// 5 minutes by default, 1 second minimum.
	// A random jitter of +/-10% will be added to the interval to spread out all the polling
	// requests resolvers have to do, avoiding spikes in resource usage.
	PollInterval time.Duration
	// If PollManually is true, ResolveNow has to be manually called for polling to happen.
	PollManually bool
	// 10 seconds by default, 1ms minimum.
	ReqTimeout time.Duration
	// Prefixes of service names to ignore additionally to administrative gRPC services (grpc.health, grpc.reflection, grpc.channelz, ...).
	IgnorePrefixes []string
	// Request only service names without the full method/message definitions.
	// Suitable only for use with pure-proto marshaling/unmarshaling, which retains all the unknown fields.
	// False by default.
	OnlyServices bool
	// Limit on the depth of the dependency chain of the retrieved file descriptors when working
	// with misbehaving servers such as python grpclib (see https://github.com/vmagamedov/grpclib/issues/187) which do not return the whole transitive dependency chain when asked to.
	// 100 by default, 0 minimum (only a single FileContainingSymbol request will be made and it will be expected to return the whole dependency chain).
	RecursionLimit int
}

func (opts ResolverOpts) withDefaults() ResolverOpts {
	if opts.Logger == nil {
		opts.Logger = bridgelog.Discard()
	}

	if opts.PollInterval == 0 {
		opts.PollInterval = 5 * time.Minute
	} else if opts.PollInterval < time.Second {
		opts.PollInterval = time.Second
	}

	if opts.ReqTimeout == 0 {
		opts.ReqTimeout = 10 * time.Second
	} else if opts.ReqTimeout < time.Millisecond {
		opts.ReqTimeout = time.Millisecond
	}

	if opts.RecursionLimit == 0 {
		opts.RecursionLimit = 100
	} else if opts.RecursionLimit < 0 {
		opts.RecursionLimit = 0
	}

	return opts
}

// ResolverBuilder is a builder for [Resolver]s,
// present to simplify their creation by moving all the shared parameters to the builder itself.
type ResolverBuilder struct {
	opts   ResolverOpts
	logger bridgelog.Logger
	pool   ConnPool
}

// NewResolverBuilder initializes a new [Resolver] builder with the specified connection pool and options,
// both of which will be inherited by the resolvers later created using [ResolverBuilder.Build].
func NewResolverBuilder(pool ConnPool, opts ResolverOpts) *ResolverBuilder {
	opts = opts.withDefaults()

	// additionally ignore gRPC services like reflection, health, channelz, etc.
	opts.IgnorePrefixes = append(opts.IgnorePrefixes, "grpc.")

	return &ResolverBuilder{
		opts:   opts,
		pool:   pool,
		logger: opts.Logger.WithComponent("grpcbridge.reflection"),
	}
}

// Build initializes a new [Resolver] for the specified target and launches its poller goroutine.
// The new resolver will use the pool specified in [NewResolverBuilder] to retrieve connections to the target.
func (rb *ResolverBuilder) Build(target string, watcher Watcher) *Resolver {
	r := &Resolver{
		opts:           rb.opts,
		target:         target,
		logger:         rb.logger.With("resolver.target", target),
		pool:           rb.pool,
		watcher:        watcher,
		methodPriority: slices.Clone(reflectionMethods),
		done:           make(chan struct{}),
	}

	r.newResolveNow() // set a valid notifyResolveNow before returning the resolver
	go r.watch()

	return r
}

// Resolver is the gRPC reflection-based description resolver, initialized using a [ResolverBuilder].
// It polls the target for which it was created for in a separate poller goroutine which is created during initialization,
// and must be destroyed using [Resolver.Close] to avoid a goroutine leak.
// By default polling happens with an interval specified by [ResolverOpts].PollInterval,
// but this can be disabled by setting [ResolverOpts].PollManually to true,
// in which case [Resolver.ResolveNow] can be used to trigger new polls.
type Resolver struct {
	opts    ResolverOpts
	target  string
	logger  bridgelog.Logger
	pool    ConnPool
	watcher Watcher
	// hash of all file descriptors retrieved on the previous iteration
	lastProtoHash string
	// hash of all service names retrieved on the previous iteration,
	// needed additionally to lastProtoHash because proto files aren't retrieved when OnlyServices is true
	lastServicesHash string
	// methodPriority keeps a prioritized version of the global reflectionMethods list for this specific service
	methodPriority   []string
	done             chan struct{}
	notifyResolveNow atomic.Pointer[func()]
	resolveNow       chan struct{}
}

// ResolveNow triggers a new poll to be performed after the current one, if any, is finished.
// This method doesn't block and just writes a signal which will be handled by the poller goroutine in the background.
func (r *Resolver) ResolveNow() {
	(*r.notifyResolveNow.Load())()
}

// Close closes the resolver, releasing any associated resources (i.e. the poller goroutine).
// It waits for the poller goroutine to acknowledge the closure, to avoid potential resource leakage.
// Calling Close() more than once will panic.
func (r *Resolver) Close() {
	r.done <- struct{}{}
}

func (r *Resolver) watch() {
	for {
		state, err := r.resolve()
		if err == nil && state != nil { // state is nil when it hasn't changed
			r.watcher.UpdateDesc(state)
		} else if err != nil {
			r.watcher.ReportError(err)
			r.logger.Error("resolution unrecoverably failed, will retry again later", "error", err)
		}

		select {
		case <-r.afterInterval():
		case <-r.resolveNow:
			r.newResolveNow()
		case <-r.done:
			close(r.done)
			return
		}
	}
}

func (r *Resolver) afterInterval() <-chan time.Time {
	// time.Ticker isn't used because we need the pollInterval to be *in between* polls,
	// not to actually resolve once every pollInterval, which might
	// lead to resolve() being called straight after the last one returning.
	if r.opts.PollManually {
		return nil
	}

	return time.After(r.opts.PollInterval)
}

func (r *Resolver) newResolveNow() {
	r.resolveNow = make(chan struct{}) // no race because this is set/used during creation and in watch() only

	// need atomic store because ResolveNow() can be called concurrently
	f := sync.OnceFunc(func() {
		close(r.resolveNow)
	})
	r.notifyResolveNow.Store(&f)
}

// resolve attempts to perform resolution using all of the available methods,
// and remembers the one which succeeded to avoid having to complete unneeded requests the next time.
func (r *Resolver) resolve() (*bridgedesc.Target, error) {
	var errs []error

	for i, method := range r.methodPriority {
		state, err := r.resolveWithMethod(method)
		if status.Code(err) == codes.Unimplemented {
			// Unimplemented can be returned by different implementations at different moments (connection, request, etc)
			// which is why this check belongs here instead of the reflection client.
			r.logger.Debug("resolution failed with Unimplemented status, will retry using different method", "method", method, "error", err)
			errs = append(errs, err)
			continue
		} else if err == nil {
			// Reprioritize the method since it worked.
			r.methodPriority[0], r.methodPriority[i] = r.methodPriority[i], r.methodPriority[0]
		}

		return state, err
	}

	return nil, fmt.Errorf("all reflection methods failed: %w", errors.Join(errs...))
}

func (r *Resolver) resolveWithMethod(method string) (*bridgedesc.Target, error) {
	cc, ok := r.pool.Get(r.target)
	if !ok {
		return nil, fmt.Errorf("no connection available in pool for target %q", r.target)
	}

	client, err := connectClient(r.opts.ReqTimeout, cc, method)
	if err != nil {
		return nil, err
	}

	defer client.close()

	serviceNames, err := r.listServiceNames(client)
	if err != nil {
		return nil, err
	}

	// Attempt to retrieve file descriptors solely using FileContainingSymbol requests.
	// Most valid implementations should return the whole set here.
	descriptors, bundles, err := r.fileDescriptorsBySymbols(client, serviceNames)
	if err != nil {
		return nil, err
	}

	// Additionally check that we actually got the whole set of file descriptors along with any dependencies.
	// If not, try to iteratively retrieve the set of missed dependencies via FileByFilename requests.
	if err := r.retrieveDependencies(client, descriptors, &bundles); err != nil {
		return nil, err
	}

	newProtoHash := hashNamedProtoBundles(bundles)
	newServicesHash := hashServiceNames(serviceNames)

	if r.lastProtoHash == newProtoHash && r.lastServicesHash == newServicesHash {
		r.logger.Debug("resolver received the same file descriptors and service names as the previous iteration, skipping update", "proto_hash", newProtoHash, "services_hash", newServicesHash)
		return nil, nil
	}

	parsed, err := descparse.ParseFileDescriptors(serviceNames, descriptors)
	if err != nil {
		return nil, err
	} else if !r.opts.OnlyServices && len(parsed.MissingServices) > 0 {
		r.logger.Warn("resolver received file descriptors with missing gRPC service definitions", "missing_services", parsed.MissingServices)
	}

	// Save the hash only at the end, when we can be sure that the new set is fully valid.
	parsed.Desc.Name = r.target
	r.lastProtoHash = newProtoHash
	r.lastServicesHash = newServicesHash
	r.logger.Debug("resolver successfully updated file descriptors", "proto_hash", newProtoHash, "services_hash", newServicesHash)

	return parsed.Desc, nil
}

// listServiceNames returns a deduplicated and validates list of service names.
func (r *Resolver) listServiceNames(c *client) ([]protoreflect.FullName, error) {
	services, err := c.listServiceNames()
	if err != nil {
		return nil, err
	}

	// Avoid raising an error when an invalid or duplicate service name is encountered,
	// simply ignoring it is better than completely stopping metadata discovery.
	processed := make(map[protoreflect.FullName]struct{}, len(services))
	filteredNames := make([]protoreflect.FullName, 0, len(services))
	for _, s := range services {
		fullName := protoreflect.FullName(s)
		if !fullName.IsValid() {
			r.logger.Warn("resolver received invalid gRPC service name", "service", fullName)
			continue
		} else if _, ok := processed[fullName]; ok {
			r.logger.Warn("resolver received duplicate gRPC service name", "service", fullName)
			continue
		}

		processed[fullName] = struct{}{}

		index := slices.IndexFunc(r.opts.IgnorePrefixes, func(prefix string) bool {
			return strings.HasPrefix(string(fullName), prefix)
		})

		if index == -1 {
			filteredNames = append(filteredNames, fullName)
		}
	}

	return filteredNames, nil
}

// fileDescriptorsBySymbols returns a parsed and deduplicated list of file descriptors for the specified symbols.
func (r *Resolver) fileDescriptorsBySymbols(c *client, symbols []protoreflect.FullName) (*descriptorpb.FileDescriptorSet, []namedProtoBundle, error) {
	return r.fileDescriptors(func() ([][]byte, error) {
		return c.fileDescriptorsBySymbols(symbols)
	})
}

// fileDescriptorsByFilenames returns a parsed and deduplicated list of file descriptors for the specified filenames.
func (r *Resolver) fileDescriptorsByFilenames(c *client, filenames []string) (*descriptorpb.FileDescriptorSet, []namedProtoBundle, error) {
	return r.fileDescriptors(func() ([][]byte, error) {
		return c.fileDescriptorsByFilenames(filenames)
	})
}

// retrieveDependencies attempts to perform a BFS traversal of the file descriptors' dependency graph,
// retrieving all the missing file descriptors via FileByFilename reflection requests.
// This is needed because some
func (r *Resolver) retrieveDependencies(c *client, descriptors *descriptorpb.FileDescriptorSet, bundles *[]namedProtoBundle) error {
	present := make(map[string]struct{}, len(descriptors.File))
	missing := make(map[string]struct{}, len(descriptors.File))

	updatePresentDescriptorSet(descriptors, present)
	growMissingDescriptorSet(descriptors, present, missing)

	var missingList []string

	for i := 0; i < r.opts.RecursionLimit && len(missing) > 0; i++ {
		missingList = slices.Grow(missingList, len(missing))[:0]
		for dep := range missing {
			missingList = append(missingList, dep)
		}

		depDescriptors, depBundles, err := r.fileDescriptorsByFilenames(c, missingList)
		if err != nil {
			return err
		}

		// Append only the missing dependencies, since the server can return elements which are unique in the context of a single request,
		// but not in the context of a whole stream.
		for i, fd := range depDescriptors.File {
			if _, ok := present[fd.GetName()]; !ok {
				descriptors.File = append(descriptors.File, fd)
				*bundles = append(*bundles, depBundles[i])
			}
		}

		updatePresentDescriptorSet(depDescriptors, present)
		shrinkMissingDescriptorSet(depDescriptors, missing)

		if len(missing) > 0 {
			return fmt.Errorf("server didn't provide file descriptors with paths %q while performing recursive dependency retrieval", missing)
		}

		growMissingDescriptorSet(depDescriptors, present, missing)
	}

	if len(missing) > 0 {
		return fmt.Errorf("failed to retrieve all file descriptors' dependencies in %d attempts", r.opts.RecursionLimit)
	}

	return nil
}

func (r *Resolver) fileDescriptors(f func() ([][]byte, error)) (*descriptorpb.FileDescriptorSet, []namedProtoBundle, error) {
	// This empty set will be handled properly, all services will be simply returned without any labeled methods.
	if r.opts.OnlyServices {
		return &descriptorpb.FileDescriptorSet{}, nil, nil
	}

	protoBytes, err := f()
	if err != nil {
		return nil, nil, err
	}

	processed := make(map[string]struct{}, len(protoBytes))
	set := &descriptorpb.FileDescriptorSet{File: make([]*descriptorpb.FileDescriptorProto, 0, len(protoBytes))}
	bundle := make([]namedProtoBundle, 0, len(protoBytes))

	// TODO(renbou): this part can be benchmarked and optimized *probably* using vtproto, since we just need to decode a barebones proto file.
	// TODO(renbou): provide a tradeoff of memory/cpu by having optional caching of parsed protofiles? probably only after optimizing the parsing itself though.
	for _, bytes := range protoBytes {
		fd := new(descriptorpb.FileDescriptorProto)
		if err := proto.Unmarshal(bytes, fd); err != nil {
			return nil, nil, fmt.Errorf("unmarshaling file descriptor: %w", err)
		}

		// duplicate files can be received and it's okay as said in the comment for client.fileDescriptorsBySymbols,
		// but we need to deduplicate them to actually parse them without errors into protoregistry.Files
		if _, ok := processed[fd.GetName()]; ok {
			continue
		}

		set.File = append(set.File, fd)
		bundle = append(bundle, namedProtoBundle{name: fd.GetName(), proto: bytes})
	}

	return set, bundle, nil
}
