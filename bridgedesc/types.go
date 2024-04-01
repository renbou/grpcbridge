package bridgedesc

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Target struct {
	Services []Service
}

type Service struct {
	Name protoreflect.FullName
}

type Method struct {
	RPCName         string
	Input           Message
	Output          Message
	ClientStreaming bool
	ServerStreaming bool
}

type Message interface {
	New() proto.Message
}
