package bridgedesc

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type emptyMessage struct{}

func (emptyMessage) New() proto.Message {
	return new(emptypb.Empty)
}

var emptyMessageInstance Message = emptyMessage{}

func DummyMethod(service protoreflect.FullName, method protoreflect.Name) *Method {
	return &Method{
		RPCName:         fmt.Sprintf("/%s/%s", service, method),
		Input:           emptyMessageInstance,
		Output:          emptyMessageInstance,
		ClientStreaming: true,
		ServerStreaming: true,
	}
}
