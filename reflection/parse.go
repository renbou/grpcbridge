package reflection

import (
	"fmt"

	"github.com/renbou/grpcbridge/bridgedesc"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type parseResults struct {
	desc            *bridgedesc.Target
	missingServices []protoreflect.FullName
}

func parseFileDescriptors(name string, svcNames []protoreflect.FullName, fds *descriptorpb.FileDescriptorSet) (parseResults, error) {
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return parseResults{}, fmt.Errorf("constructing proto file registry from descriptors: %w", err)
	}

	types := dynamicpb.NewTypes(files)
	targetDesc := bridgedesc.ParseTarget(name, files, types, svcNames)

	missingServices := make([]protoreflect.FullName, 0, len(svcNames))
	for _, svcName := range svcNames {
		if _, err := files.FindDescriptorByName(svcName); err != nil {
			missingServices = append(missingServices, svcName)
		}
	}

	return parseResults{desc: targetDesc, missingServices: missingServices}, nil
}
