package testpb

import (
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// TestServiceFDs is a set of the raw protobuf file descriptors for TestService.
var TestServiceFDs *descriptorpb.FileDescriptorSet

func init() {
	file_testsvc_proto_init()
	TestServiceFDs = fileDescriptors(File_testsvc_proto)
}

func fileDescriptors(desc protoreflect.FileDescriptor) *descriptorpb.FileDescriptorSet {
	protoMap := make(map[string]*descriptorpb.FileDescriptorProto)
	queue := []protoreflect.FileDescriptor{desc}

	for len(queue) > 0 {
		fd := queue[0]
		queue = queue[1:]

		if _, ok := protoMap[fd.Path()]; ok {
			continue
		}

		protoMap[fd.Path()] = protodesc.ToFileDescriptorProto(fd)

		for i := range fd.Imports().Len() {
			queue = append(queue, fd.Imports().Get(i))
		}
	}

	protos := make([]*descriptorpb.FileDescriptorProto, 0, len(protoMap))
	for _, proto := range protoMap {
		protos = append(protos, proto)
	}

	return &descriptorpb.FileDescriptorSet{File: protos}
}
