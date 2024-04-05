package reflection

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
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

type ConnPool interface {
	Get(target string) grpcadapter.Connection
}

type Watcher interface {
	UpdateDesc(*bridgedesc.Target)
	ReportError(error)
}

type ResolverOpts struct {
	Logger bridgelog.Logger
	// 5 minutes by default, 1 second minimum.
	PollInterval time.Duration
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

type ResolverBuilder struct {
	opts   ResolverOpts
	logger bridgelog.Logger
	pool   *grpcadapter.DialedPool
}

func NewResolverBuilder(pool *grpcadapter.DialedPool, opts ResolverOpts) *ResolverBuilder {
	opts = opts.withDefaults()

	// additionally ignore gRPC services like reflection, health, channelz, etc.
	opts.IgnorePrefixes = append(opts.IgnorePrefixes, "grpc.")

	return &ResolverBuilder{
		opts:   opts,
		pool:   pool,
		logger: opts.Logger.WithComponent("grpcbridge.reflection"),
	}
}

func (rb *ResolverBuilder) Build(name string, watcher Watcher) (*resolver, error) {
	r := &resolver{
		opts:           rb.opts,
		name:           name,
		logger:         rb.logger.With("resolver.target", name),
		pool:           rb.pool,
		watcher:        watcher,
		methodPriority: slices.Clone(reflectionMethods),
		done:           make(chan struct{}),
	}

	go r.watch()

	return r, nil
}

type resolver struct {
	opts    ResolverOpts
	name    string
	logger  bridgelog.Logger
	pool    *grpcadapter.DialedPool
	watcher Watcher
	// hash of all file descriptors retrieved on the previous iteration
	lastProtoHash string
	// hash of all service names retrieved on the previous iteration,
	// needed additionally to lastProtoHash because proto files aren't retrieved when OnlyServices is true
	lastServicesHash string
	// methodPriority keeps a prioritized version of the global reflectionMethods list for this specific service
	methodPriority []string
	done           chan struct{}
}

func (r *resolver) watch() {
	for {
		state, err := r.resolve()
		if err == nil && state != nil { // state is nil when it hasn't changed
			r.watcher.UpdateDesc(state)
		} else if err != nil {
			r.watcher.ReportError(err)
			r.logger.Error("resolution unrecoverably failed, will retry again later", "error", err)
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

// resolve attempts to perform resolution using all of the available methods,
// and remembers the one which succeeded to avoid having to complete unneeded requests the next time.
func (r *resolver) resolve() (*bridgedesc.Target, error) {
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

func (r *resolver) resolveWithMethod(method string) (*bridgedesc.Target, error) {
	cc := r.pool.Get(r.name)
	if cc == nil {
		return nil, fmt.Errorf("no connection available in pool for target %q", r.name)
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

	parsed, err := parseFileDescriptors(serviceNames, descriptors)
	if err != nil {
		return nil, err
	} else if !r.opts.OnlyServices && len(parsed.missingServices) > 0 {
		r.logger.Warn("resolver received file descriptors with missing gRPC service definitions", "missing_services", parsed.missingServices)
	}

	// Save the hash only at the end, when we can be sure that the new set is fully valid.
	r.lastProtoHash = newProtoHash
	r.lastServicesHash = newServicesHash
	r.logger.Debug("resolver successfully updated file descriptors", "proto_hash", newProtoHash, "services_hash", newServicesHash)

	return parsed.targetDesc, nil
}

// listServiceNames returns a deduplicated and validates list of service names.
func (r *resolver) listServiceNames(c *client) ([]protoreflect.FullName, error) {
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
func (r *resolver) fileDescriptorsBySymbols(c *client, symbols []protoreflect.FullName) (*descriptorpb.FileDescriptorSet, []namedProtoBundle, error) {
	return r.fileDescriptors(func() ([][]byte, error) {
		return c.fileDescriptorsBySymbols(symbols)
	})
}

// fileDescriptorsByFilenames returns a parsed and deduplicated list of file descriptors for the specified filenames.
func (r *resolver) fileDescriptorsByFilenames(c *client, filenames []string) (*descriptorpb.FileDescriptorSet, []namedProtoBundle, error) {
	return r.fileDescriptors(func() ([][]byte, error) {
		return c.fileDescriptorsByFilenames(filenames)
	})
}

// retrieveDependencies attempts to perform a BFS traversal of the file descriptors' dependency graph,
// retrieving all the missing file descriptors via FileByFilename reflection requests.
// This is needed because some
func (r *resolver) retrieveDependencies(c *client, descriptors *descriptorpb.FileDescriptorSet, bundles *[]namedProtoBundle) error {
	present := make(map[string]struct{}, len(descriptors.File))
	missing := make(map[string]struct{}, len(descriptors.File))

	updatePresentDescriptorSet(descriptors, present)
	growMissingDescriptorSet(descriptors, present, missing)

	var missingList []string

	for i := 0; i < r.opts.RecursionLimit && len(missing) > 0; i++ {
		missingList = slices.Grow(missingList, len(missing))
		for dep := range missing {
			missingList = append(missingList, dep)
		}

		depDescriptors, depBundles, err := r.fileDescriptorsByFilenames(c, missingList)
		if err != nil {
			return err
		}

		updatePresentDescriptorSet(depDescriptors, present)
		shrinkMissingDescriptorSet(depDescriptors, missing)

		if len(missing) > 0 {
			return fmt.Errorf("server didn't provide file descriptors with paths %q while performing recursive dependency retrieval", missing)
		}

		growMissingDescriptorSet(depDescriptors, present, missing)
		descriptors.File = append(descriptors.File, depDescriptors.File...)
		*bundles = append(*bundles, depBundles...)
	}

	if len(missing) > 0 {
		return fmt.Errorf("failed to retrieve all file descriptors' dependencies in %d attempts", r.opts.RecursionLimit)
	}

	return nil
}

func (r *resolver) fileDescriptors(f func() ([][]byte, error)) (*descriptorpb.FileDescriptorSet, []namedProtoBundle, error) {
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
