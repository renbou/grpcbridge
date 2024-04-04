package reflection

import (
	"fmt"

	"github.com/renbou/grpcbridge/bridgedesc"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func updatePresentDescriptorSet(descriptors *descriptorpb.FileDescriptorSet, present map[string]struct{}) {
	for _, fd := range descriptors.File {
		present[fd.GetName()] = struct{}{}
	}
}

func growMissingDescriptorSet(descriptors *descriptorpb.FileDescriptorSet, present map[string]struct{}, missing map[string]struct{}) {
	for _, fd := range descriptors.File {
		for _, dep := range fd.Dependency {
			if _, ok := present[dep]; !ok {
				missing[dep] = struct{}{}
			}
		}
	}
}

func shrinkMissingDescriptorSet(descriptors *descriptorpb.FileDescriptorSet, missing map[string]struct{}) {
	for _, fd := range descriptors.File {
		delete(missing, fd.GetName())
	}
}

type parseResult struct {
	targetDesc *bridgedesc.Target
	// missingServices returned separately because they are handled
	// differently depending on whether we actually need the complete definitions.
	missingServices []protoreflect.FullName
}

// parseFileDescriptors parses a whole set of file descriptors into the grpcbridge description format.
// TODO(renbou): this can be rewritten to perform incremental parsing while receiving new file descriptors
// from the reflection server, which would allow graceful degradation via loss of only specific corrupt service definitions.
// Right now if at least one of the proto files is invalid, the whole operation will fail.
func parseFileDescriptors(serviceNames []protoreflect.FullName, descriptors *descriptorpb.FileDescriptorSet) (parseResult, error) {
	// All files need to be parsed because we expect to receive the services and their transitive dependencies.
	// It's valid for a service to depend on a proto with an unused service definition,
	// i.e. depending on health.proto but not actually running the healthcheck service as-is,
	// so the unneeded service definitions need to be filtered out manually later.
	files, err := protodesc.NewFiles(descriptors)
	if err != nil {
		return parseResult{}, fmt.Errorf("constructing proto file registry from descriptors: %w", err)
	}

	targetDesc := &bridgedesc.Target{Services: make([]bridgedesc.Service, len(serviceNames))}
	svcIndexes := make(map[protoreflect.FullName]int)

	for i, name := range serviceNames {
		targetDesc.Services[i] = bridgedesc.Service{Name: name}
		svcIndexes[name] = i
	}

	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		iterateProtoList(fd.Services(), func(_ int, sd protoreflect.ServiceDescriptor) {
			svcIdx, ok := svcIndexes[sd.FullName()]
			if !ok {
				return
			}

			parseServiceDescriptors(&targetDesc.Services[svcIdx], sd)
			delete(svcIndexes, sd.FullName()) // mark as seen
		})

		return true
	})

	if len(svcIndexes) < 1 {
		return parseResult{targetDesc: targetDesc}, nil
	}

	missingServices := make([]protoreflect.FullName, 0, len(svcIndexes))
	for name := range svcIndexes {
		missingServices = append(missingServices, name)
	}

	return parseResult{targetDesc: targetDesc, missingServices: missingServices}, nil
}

func parseServiceDescriptors(desc *bridgedesc.Service, sd protoreflect.ServiceDescriptor) {
	desc.Methods = make([]bridgedesc.Method, sd.Methods().Len())

	iterateProtoList(sd.Methods(), func(i int, md protoreflect.MethodDescriptor) {
		parseMethodDescriptor(&desc.Methods[i], sd, md)
	})
}

func parseMethodDescriptor(desc *bridgedesc.Method, sd protoreflect.ServiceDescriptor, md protoreflect.MethodDescriptor) {
	desc.RPCName = bridgedesc.FormatRPCName(sd.FullName(), md.Name())
	desc.Input = bridgedesc.DynamicMessage(md.Input())
	desc.Output = bridgedesc.DynamicMessage(md.Output())
	desc.ClientStreaming = md.IsStreamingClient()
	desc.ServerStreaming = md.IsStreamingServer()
}

type protoList[T any] interface {
	Len() int
	Get(int) T
}

func iterateProtoList[T any](list protoList[T], f func(int, T)) {
	for i := range list.Len() {
		f(i, list.Get(i))
	}
}
