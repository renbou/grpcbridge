package bridgedesc

import (
	"net/http"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ParseTarget parses the complete description of a target using the specified registries, and sets the target's name.
//
// A target description without services doesn't make sense in the context of grpcbridge,
// because the file registry might contain an arbitrary number of services which aren't actually served by the target itself.
// For example, a target's service can depend on health.proto, but not be actually running the base gRPC healthcheck service.
// For this reason, the services available in the files are filtered and only the definitions of the ones requested are parsed.
func ParseTarget(name string, files FileRegistry, types TypeRegistry, svcNames []protoreflect.FullName) *Target {
	targetDesc := &Target{Name: name, FileRegistry: files, TypeRegistry: types, Services: make([]Service, len(svcNames))}

	for i, name := range svcNames {
		// err ignored, returns NotFound and we don't care about it, the service will just be returned without defined methods.
		desc, _ := files.FindDescriptorByName(name)
		svcDesc, ok := desc.(protoreflect.ServiceDescriptor)

		if ok {
			parseServiceDescriptors(&targetDesc.Services[i], svcDesc)
		}

		targetDesc.Services[i].Name = name
	}

	return targetDesc
}

func parseServiceDescriptors(desc *Service, sd protoreflect.ServiceDescriptor) {
	desc.Methods = make([]Method, sd.Methods().Len())

	iterateProtoList(sd.Methods(), func(i int, md protoreflect.MethodDescriptor) {
		parseMethodDescriptor(&desc.Methods[i], sd, md)
	})
}

func parseMethodDescriptor(desc *Method, sd protoreflect.ServiceDescriptor, md protoreflect.MethodDescriptor) {
	desc.RPCName = CanonicalRPCName(sd.FullName(), md.Name())
	desc.Input = DynamicMessage(md.Input())
	desc.Output = DynamicMessage(md.Output())
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

	desc.Bindings = make([]Binding, 1+len(httpRule.AdditionalBindings))
	parseBinding(&desc.Bindings[0], httpRule)

	for i, binding := range httpRule.AdditionalBindings {
		parseBinding(&desc.Bindings[i+1], binding)
	}
}

func parseBinding(desc *Binding, rule *annotations.HttpRule) {
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
