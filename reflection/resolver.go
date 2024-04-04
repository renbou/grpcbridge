package reflection

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/bridgelog"
	"github.com/renbou/grpcbridge/grpcadapter"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

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
	// with misbehaving servers such as python grpclib which do not return the whole transitive dependency chain when asked to.
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
	logger  bridgelog.Logger
	pool    *grpcadapter.DialedPool
	watcher Watcher
	done    chan struct{}
}

func (r *resolver) watch() {
	for {
		state, err := r.resolve()
		if err == nil {
			r.watcher.UpdateDesc(state)
		} else {
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

// TODO(renbou): support v1alpha and v1 at the same time, using some mechanism like this:
// - try v1
// - if it fails, try v1alpha
// - if it works, then remember this and use it the next time
// - if it fails, return an error
// Then if v1alpha starts failing, try switching to v1, and repeat.
// NB: "fails" here means any error, not just "Unimplemented", because servers might misbehave in various ways.
func (r *resolver) resolve() (*bridgedesc.Target, error) {
	cc := r.pool.Get(r.name)
	if cc == nil {
		return nil, fmt.Errorf("no connection available in pool for target %q", r.name)
	}

	client, err := connectClient(r.opts.ReqTimeout, cc, reflectionpb.ServerReflection_ServerReflectionInfo_FullMethodName)
	if err != nil {
		return nil, err
	}

	defer client.close()

	serviceNames, err := r.listServiceNames(client)
	if err != nil {
		return nil, err
	}

	descriptors, err := r.fileDescriptorsBySymbols(client, serviceNames)
	if err != nil {
		return nil, err
	}

	if err := r.retrieveDependencies(client, descriptors); err != nil {
		return nil, err
	}

	parsed, err := parseFileDescriptors(serviceNames, descriptors)
	if err != nil {
		return nil, err
	} else if !r.opts.OnlyServices && len(parsed.missingServices) > 0 {
		r.logger.Warn("resolver received file descriptors with missing gRPC service definitions", "missing_services", parsed.missingServices)
	}

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
func (r *resolver) fileDescriptorsBySymbols(c *client, symbols []protoreflect.FullName) (*descriptorpb.FileDescriptorSet, error) {
	return r.fileDescriptors(func() ([][]byte, error) {
		return c.fileDescriptorsBySymbols(symbols)
	})
}

// fileDescriptorsByFilenames returns a parsed and deduplicated list of file descriptors for the specified filenames.
func (r *resolver) fileDescriptorsByFilenames(c *client, filenames []string) (*descriptorpb.FileDescriptorSet, error) {
	return r.fileDescriptors(func() ([][]byte, error) {
		return c.fileDescriptorsByFilenames(filenames)
	})
}

// retrieveDependencies attempts to perform a BFS traversal of the file descriptors' dependency graph,
// retrieving all the missing file descriptors via FileByFilename reflection requests.
// This is needed because some
func (r *resolver) retrieveDependencies(c *client, descriptors *descriptorpb.FileDescriptorSet) error {
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

		depDescriptors, err := r.fileDescriptorsByFilenames(c, missingList)
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
	}

	if len(missing) > 0 {
		return fmt.Errorf("failed to retrieve all file descriptors' dependencies in %d attempts", r.opts.RecursionLimit)
	}

	return nil
}

func (r *resolver) fileDescriptors(f func() ([][]byte, error)) (*descriptorpb.FileDescriptorSet, error) {
	// This empty set will be handled properly, all services will be simply returned without any labeled methods.
	if r.opts.OnlyServices {
		return &descriptorpb.FileDescriptorSet{}, nil
	}

	protoBytes, err := f()
	if err != nil {
		return nil, err
	}

	processed := make(map[string]struct{}, len(protoBytes))
	set := &descriptorpb.FileDescriptorSet{File: make([]*descriptorpb.FileDescriptorProto, 0, len(protoBytes))}

	// TODO(renbou): deduplicate parsing by caching the parsed descriptors by hashes of the bytes?
	// in 99% of the cases, the same file descriptors will be returned, since it's not like protos change that often.
	// maybe even completely avoid sending the update to the watcher when nothing has changed...
	for _, bytes := range protoBytes {
		fd := new(descriptorpb.FileDescriptorProto)
		if err := proto.Unmarshal(bytes, fd); err != nil {
			return nil, fmt.Errorf("unmarshaling file descriptor: %w", err)
		}

		// duplicate files can be received and it's okay as said in the comment for client.fileDescriptorsBySymbols,
		// but we need to deduplicate them to actually parse them without errors into protoregistry.Files
		if _, ok := processed[fd.GetName()]; ok {
			continue
		}

		set.File = append(set.File, fd)
	}

	return set, nil
}
