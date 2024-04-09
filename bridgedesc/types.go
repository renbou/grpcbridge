package bridgedesc

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

var emptyMessageInstance Message = ConcreteMessage[emptypb.Empty]()

// FileResolver contains basic methods needed to discover proto files by their name and defined symbols.
// It is used for implementing gRPC reflection and by [ParseTarget] for parsing [Target] definitions.
type FileResolver interface {
	protodesc.Resolver
}

// TypeResolver contains the methods needed to resolve proto types for implementing gRPC reflection
// or performing operations such as marshaling/unmarshaling protos to JSON.
// It is implemented by *protoregistry.Types and *dynamicpb.Types.
type TypeResolver interface {
	protoregistry.MessageTypeResolver
	protoregistry.ExtensionTypeResolver
}

type Target struct {
	// A target's name is arbitrary and only makes sense in the context of grpcbridge.
	Name string
	// FileRegistry is the resolver of proto files defined for this target, using which the description was parsed.
	FileResolver FileResolver
	// TypeRegistry is the resolver of proto type descriptors defined for this target, it should be derived from the FileRegistry.
	TypeResolver TypeResolver
	// Services contain the descriptions of services available in this target.
	// Note that the FileRegistry might define more services to be available in the files,
	// however, this list contains only the services that are actually served by the target.
	// The list of services isn't required to contain full descriptions of all service methods,
	// and can simply contain the names on the services,
	// in which case it would still be usable for cases like gRPC proxying and gRPC-Web bridging.
	Services []Service
}

type Service struct {
	Name    protoreflect.FullName
	Methods []Method
}

type Method struct {
	RPCName         string
	Input           Message
	Output          Message
	ClientStreaming bool
	ServerStreaming bool
	Bindings        []Binding
}

// DummyMethod constructs a dummy method description containing emptypb.Empty messages as Input and Output.
// Proto properly unmarshals any message into an empty one, keeping all the fields as protoimpl.UnknownFields,
// so this can be used for unmarshaling/marshaling proto messages without needing to know their complete definitions,
// Useful for cases such as barebones gRPC proxying and gRPC-Web bridging.
func DummyMethod(svcName protoreflect.FullName, methodName protoreflect.Name) *Method {
	return &Method{
		RPCName:         CanonicalRPCName(svcName, methodName),
		Input:           emptyMessageInstance,
		Output:          emptyMessageInstance,
		ClientStreaming: true,
		ServerStreaming: true,
	}
}

// CanonicalRPCName returns the canonical gRPC method name, as specified in the gRPC [PROTOCOL-HTTP2] spec.
//
// [PROTOCOL-HTTP2]: https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md#requests
func CanonicalRPCName(svcName protoreflect.FullName, methodName protoreflect.Name) string {
	return fmt.Sprintf("/%s/%s", svcName, methodName)
}

type Binding struct {
	HTTPMethod       string
	Pattern          string
	RequestBodyPath  string
	ResponseBodyPath string
}

func DefaultBinding(method *Method) *Binding {
	return &Binding{
		HTTPMethod:       "POST",
		Pattern:          method.RPCName,
		RequestBodyPath:  "*",
		ResponseBodyPath: "", // "When omitted, the entire response message will be used as the HTTP response body.", from http.proto
	}
}

type Message interface {
	New() proto.Message
}

// DynamicMessage returns a Message implementation which uses [dynamicpb.NewMessage] to dynamically create messages based on a [protoreflect.MessageDescriptor].
func DynamicMessage(desc protoreflect.MessageDescriptor) Message {
	return dynamicMessage{desc}
}

// ConcreteMessage returns a Message implementation which simply allocates the concrete proto.Message implementation on each New call.
func ConcreteMessage[T any, PT interface {
	*T
	proto.Message
}]() Message {
	return concreteMessage[T, PT]{}
}

type concreteMessage[T any, PT interface {
	*T
	proto.Message
}] struct{}

func (cm concreteMessage[T, PT]) New() proto.Message {
	return PT(new(T))
}

type dynamicMessage struct {
	desc protoreflect.MessageDescriptor
}

func (dm dynamicMessage) New() proto.Message {
	return dynamicpb.NewMessage(dm.desc)
}
