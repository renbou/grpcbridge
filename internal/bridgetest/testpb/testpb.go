package testpb

import (
	"fmt"

	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// TestServiceFDs is a set of the raw protobuf file descriptors for TestService.
var (
	TestServiceFDs           *descriptorpb.FileDescriptorSet
	TestServiceFileRegistry  *protoregistry.Files
	TestServiceTypesRegistry *dynamicpb.Types
)

func init() {
	file_testsvc_proto_init()
	TestServiceFDs = fileDescriptors(File_testsvc_proto)
	TestServiceFileRegistry = fileRegistry(File_testsvc_proto.Path(), TestServiceFDs)
	TestServiceTypesRegistry = dynamicpb.NewTypes(TestServiceFileRegistry)
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

func fileRegistry(name string, set *descriptorpb.FileDescriptorSet) *protoregistry.Files {
	registry, err := protodesc.NewFiles(set)
	if err != nil {
		panic(fmt.Sprintf("failed to parse file descriptor set for %s as protoregistry.Files: %s", name, err))
	}

	return registry
}
