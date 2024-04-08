package descparse

import (
	"fmt"
	"net/http"

	"github.com/renbou/grpcbridge/bridgedesc"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// ParseResults contain both the fully parsed description and a list of service names which were not found in the descriptors.
type ParseResults struct {
	Desc            *bridgedesc.Target
	MissingServices []protoreflect.FullName
}

// ParseFileDescriptors parses a set of file descriptors into the grpcbridge description format.
func ParseFileDescriptors(serviceNames []protoreflect.FullName, descriptors *descriptorpb.FileDescriptorSet) (ParseResults, error) {
	// All files need to be parsed because we expect to receive the services and their transitive dependencies.
	// It's valid for a service to depend on a proto with an unused service definition,
	// i.e. depending on health.proto but not actually running the healthcheck service as-is,
	// so the unneeded service definitions need to be filtered out manually later.
	files, err := protodesc.NewFiles(descriptors)
	if err != nil {
		return ParseResults{}, fmt.Errorf("constructing proto file registry from descriptors: %w", err)
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
		return ParseResults{Desc: targetDesc}, nil
	}

	missingServices := make([]protoreflect.FullName, 0, len(svcIndexes))
	for name := range svcIndexes {
		missingServices = append(missingServices, name)
	}

	return ParseResults{Desc: targetDesc, MissingServices: missingServices}, nil
}

func parseServiceDescriptors(desc *bridgedesc.Service, sd protoreflect.ServiceDescriptor) {
	desc.Methods = make([]bridgedesc.Method, sd.Methods().Len())

	iterateProtoList(sd.Methods(), func(i int, md protoreflect.MethodDescriptor) {
		parseMethodDescriptor(&desc.Methods[i], sd, md)
	})
}

func parseMethodDescriptor(desc *bridgedesc.Method, sd protoreflect.ServiceDescriptor, md protoreflect.MethodDescriptor) {
	desc.RPCName = bridgedesc.CanonicalRPCName(sd.FullName(), md.Name())
	desc.Input = bridgedesc.DynamicMessage(md.Input())
	desc.Output = bridgedesc.DynamicMessage(md.Output())
	desc.ClientStreaming = md.IsStreamingClient()
	desc.ServerStreaming = md.IsStreamingServer()

	if !proto.HasExtension(md.Options(), annotations.E_Http) {
		return
	}

	ext := proto.GetExtension(md.Options(), annotations.E_Http)
	httpRule, ok := ext.(*annotations.HttpRule)
	if !ok {
		return
	}

	desc.Bindings = make([]bridgedesc.Binding, 1+len(httpRule.AdditionalBindings))
	parseBinding(&desc.Bindings[0], httpRule)

	for i, binding := range httpRule.AdditionalBindings {
		parseBinding(&desc.Bindings[i+1], binding)
	}
}

func parseBinding(desc *bridgedesc.Binding, rule *annotations.HttpRule) {
	switch pattern := rule.GetPattern().(type) {
	case *annotations.HttpRule_Get:
		desc.HTTPMethod = http.MethodGet
		desc.Pattern = pattern.Get
	case *annotations.HttpRule_Put:
		desc.HTTPMethod = http.MethodPut
		desc.Pattern = pattern.Put
	case *annotations.HttpRule_Post:
		desc.HTTPMethod = http.MethodPost
		desc.Pattern = pattern.Post
	case *annotations.HttpRule_Delete:
		desc.HTTPMethod = http.MethodDelete
		desc.Pattern = pattern.Delete
	case *annotations.HttpRule_Patch:
		desc.HTTPMethod = http.MethodPatch
		desc.Pattern = pattern.Patch
	case *annotations.HttpRule_Custom:
		desc.HTTPMethod = pattern.Custom.GetKind()
		desc.Pattern = pattern.Custom.GetPath()
	}

	desc.RequestBodyPath = rule.GetBody()
	desc.ResponseBodyPath = rule.GetResponseBody()
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
