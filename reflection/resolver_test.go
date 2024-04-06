package reflection

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/renbou/grpcbridge/bridgedesc"
	"github.com/renbou/grpcbridge/internal/bridgetest"
	"github.com/renbou/grpcbridge/internal/bridgetest/testpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	reflectionalphapb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type fakeWatcher struct {
	desc   *bridgedesc.Target
	errors []error
}

func (fw *fakeWatcher) UpdateDesc(desc *bridgedesc.Target) {
	fw.desc = desc
}

func (fw *fakeWatcher) ReportError(err error) {
	fw.errors = append(fw.errors, err)
}

// stupidReflectionServer emulates a gRPC reflection server
// which answers all requests only with the requested descriptor, not the whole transitive dependency chain.
type stupidReflectionServer struct {
	// alpha version to test that the resolve properly handles servers not serving v1
	reflectionalphapb.UnimplementedServerReflectionServer
	services    []string
	descriptors *descriptorpb.FileDescriptorSet
}

func (s *stupidReflectionServer) ServerReflectionInfo(stream reflectionalphapb.ServerReflection_ServerReflectionInfoServer) error {
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}

		var resp *reflectionalphapb.ServerReflectionResponse

		switch msg := req.MessageRequest.(type) {
		case *reflectionalphapb.ServerReflectionRequest_ListServices:
			resp = s.listServices()
		case *reflectionalphapb.ServerReflectionRequest_FileContainingSymbol:
			resp = s.fileContainingSymbol(msg.FileContainingSymbol)
		case *reflectionalphapb.ServerReflectionRequest_FileByFilename:
			resp = s.fileByFilename(msg.FileByFilename)
		case *reflectionalphapb.ServerReflectionRequest_AllExtensionNumbersOfType:
			resp = s.unsupported()
		case *reflectionalphapb.ServerReflectionRequest_FileContainingExtension:
			resp = s.unsupported()
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

func (s *stupidReflectionServer) listServices() *reflectionalphapb.ServerReflectionResponse {
	services := make([]*reflectionalphapb.ServiceResponse, len(s.services))

	for i, svc := range s.services {
		services[i] = &reflectionalphapb.ServiceResponse{Name: svc}
	}

	return &reflectionalphapb.ServerReflectionResponse{
		MessageResponse: &reflectionalphapb.ServerReflectionResponse_ListServicesResponse{
			ListServicesResponse: &reflectionalphapb.ListServiceResponse{Service: services},
		},
	}
}

// fileContainingSymbol currently supports only lookup by service name,
// as other symbol lookups aren't used in the reflection resolver.
func (s *stupidReflectionServer) fileContainingSymbol(symbol string) *reflectionalphapb.ServerReflectionResponse {
	for _, fd := range s.descriptors.File {
		for _, svc := range fd.Service {
			fullName := protoreflect.FullName(fd.GetPackage()).Append(protoreflect.Name(svc.GetName()))

			if string(fullName) != symbol {
				continue
			}

			fdProto, err := proto.Marshal(fd)
			if err != nil {
				return s.error(codes.Internal, fmt.Sprintf("marshaling file descriptor for symbol %s: %s", symbol, err))
			}

			return &reflectionalphapb.ServerReflectionResponse{
				MessageResponse: &reflectionalphapb.ServerReflectionResponse_FileDescriptorResponse{
					FileDescriptorResponse: &reflectionalphapb.FileDescriptorResponse{
						FileDescriptorProto: [][]byte{fdProto},
					},
				},
			}
		}
	}

	return s.error(codes.NotFound, fmt.Sprintf("symbol %s not found", symbol))
}

func (s *stupidReflectionServer) fileByFilename(filename string) *reflectionalphapb.ServerReflectionResponse {
	for _, fd := range s.descriptors.File {
		if fd.GetName() != filename {
			continue
		}

		fdProto, err := proto.Marshal(fd)
		if err != nil {
			return s.error(codes.Internal, fmt.Sprintf("marshaling file descriptor for filename %s: %s", filename, err))
		}

		return &reflectionalphapb.ServerReflectionResponse{
			MessageResponse: &reflectionalphapb.ServerReflectionResponse_FileDescriptorResponse{
				FileDescriptorResponse: &reflectionalphapb.FileDescriptorResponse{
					FileDescriptorProto: [][]byte{fdProto},
				},
			},
		}
	}

	return s.error(codes.NotFound, fmt.Sprintf("file %s not found", filename))
}

func (s *stupidReflectionServer) unsupported() *reflectionalphapb.ServerReflectionResponse {
	return s.error(codes.Unimplemented, "unimplemented")
}

func (s *stupidReflectionServer) error(code codes.Code, msg string) *reflectionalphapb.ServerReflectionResponse {
	return &reflectionalphapb.ServerReflectionResponse{
		MessageResponse: &reflectionalphapb.ServerReflectionResponse_ErrorResponse{
			ErrorResponse: &reflectionalphapb.ErrorResponse{
				ErrorCode:    int32(code),
				ErrorMessage: msg,
			},
		},
	}
}

func Test_Resolver(t *testing.T) {
	t.Parallel()

	// Arrange
	reflectionServer := &stupidReflectionServer{
		services:    []string{testpb.TestService_ServiceDesc.ServiceName},
		descriptors: testpb.TestServiceFDs,
	}

	server, pool, _ := bridgetest.MustGRPCServer(t, func(s *grpc.Server) {
		reflectionalphapb.RegisterServerReflectionServer(s, reflectionServer)
	})
	defer server.Stop()

	builder := NewResolverBuilder(pool, ResolverOpts{
		PollManually: true,
	})

	watcher := new(fakeWatcher)

	// Act
	// Build and immediatelly close the resolver.
	// watch() will execute once and report the results,
	resolver := builder.Build(bridgetest.TestTarget, watcher)
	resolver.Close()

	// Assert
	if len(watcher.errors) > 0 {
		t.Errorf("watcher received unexpected errors from the resolver: %v", watcher.errors)
	}

	if watcher.desc == nil {
		t.Errorf("watcher hasn't received any description from the resolver")
	}
}
