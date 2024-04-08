package bridgedesc

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

var emptyMessageInstance Message = ConcreteMessage[emptypb.Empty]()

type Target struct {
	Name     string
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

// Proto properly unmarshals any message into an empty one, keeping all the fields as protoimpl.UnknownFields.
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
