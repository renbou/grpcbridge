package reflection

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/renbou/grpcbridge/grpcadapter"
	"github.com/renbou/grpcbridge/internal/bridgetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	reflectionpb "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func mustClient(t *testing.T, conn grpcadapter.ClientConn) *client {
	client, err := connectClient(time.Second*10, conn, reflectionpb.ServerReflection_ServerReflectionInfo_FullMethodName)
	if err != nil {
		t.Fatalf("connectClient() returned non-nil error = %q", err)
	}

	t.Cleanup(client.close)

	return client
}

type staticReflectionServer struct {
	reflectionpb.UnimplementedServerReflectionServer
	resp *reflectionpb.ServerReflectionResponse
}

func (s *staticReflectionServer) ServerReflectionInfo(stream reflectionpb.ServerReflection_ServerReflectionInfoServer) error {
	for {
		_, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}

		if err := stream.Send(s.resp); err != nil {
			return err
		}
	}
}

func isFailedResponseCheck(err error) error {
	const needle = "received response to different request instead"

	if err == nil {
		return fmt.Errorf("got nil error, want it to contain %q", needle)
	} else if !strings.Contains(err.Error(), needle) {
		return fmt.Errorf("got error = %q, want %q", err, needle)
	}

	return nil
}

// Test_client_UnimplementedErrors tests that client methods return a status error with the Unimplemented code
// when a gRPC server doesn't implement the used reflection method.
func Test_client_UnimplementedErrors(t *testing.T) {
	t.Parallel()

	server, _, conn := bridgetest.MustGRPCServer(t)
	defer server.Stop()

	// Run in non-parallel subtest so that server.Stop() runs AFTER all the subtests.
	t.Run("cases", func(t *testing.T) {
		t.Run("listServiceNames", func(t *testing.T) {
			test_client_UnimplementedErrors_listServiceNames(t, conn)
		})

		t.Run("fileDescriptorsBySymbols", func(t *testing.T) {
			test_client_UnimplementedErrors_fileDescriptorsBySymbols(t, conn)
		})

		t.Run("fileDescriptorsByFilenames", func(t *testing.T) {
			test_client_UnimplementedErrors_fileDescriptorsByFilenames(t, conn)
		})
	})
}

func test_client_UnimplementedErrors_listServiceNames(t *testing.T, conn grpcadapter.ClientConn) {
	t.Parallel()

	// Arrange
	client := mustClient(t, conn)

	// Act
	_, err := client.listServiceNames()

	// Assert
	if cmpErr := bridgetest.StatusCodeIs(err, codes.Unimplemented); cmpErr != nil {
		t.Errorf("listServiceNames() returned error = %q with unexpected code: %s", err, cmpErr)
	}
}

func test_client_UnimplementedErrors_fileDescriptorsBySymbols(t *testing.T, conn grpcadapter.ClientConn) {
	t.Parallel()

	// Arrange
	client := mustClient(t, conn)

	// Act
	_, err := client.fileDescriptorsBySymbols([]protoreflect.FullName{"package.Service"})

	// Assert
	if cmpErr := bridgetest.StatusCodeIs(err, codes.Unimplemented); cmpErr != nil {
		t.Errorf("fileDescriptorsBySymbols() returned error = %q with unexpected code: %s", err, cmpErr)
	}
}

func test_client_UnimplementedErrors_fileDescriptorsByFilenames(t *testing.T, conn grpcadapter.ClientConn) {
	t.Parallel()

	// Arrange
	client := mustClient(t, conn)

	// Act
	_, err := client.fileDescriptorsByFilenames([]string{"file.proto"})

	// Assert
	if cmpErr := bridgetest.StatusCodeIs(err, codes.Unimplemented); cmpErr != nil {
		t.Errorf("fileDescriptorsByFilenames() returned error = %q with unexpected code: %s", err, cmpErr)
	}
}

// Test_client_ErrorResponses tests that client methods return a proper status error
// when the reflection server returns an ErrorResponse message.
func Test_client_ErrorResponses(t *testing.T) {
	t.Parallel()

	errorStatus := status.New(codes.Internal, "unexpected internal error")
	reflectionServer := &staticReflectionServer{resp: &reflectionpb.ServerReflectionResponse{
		MessageResponse: &reflectionpb.ServerReflectionResponse_ErrorResponse{ErrorResponse: &reflectionpb.ErrorResponse{
			ErrorCode:    int32(errorStatus.Code()),
			ErrorMessage: errorStatus.Message(),
		}},
	}}

	server, _, conn := bridgetest.MustGRPCServer(t, func(s *grpc.Server) {
		reflectionpb.RegisterServerReflectionServer(s, reflectionServer)
	})
	defer server.Stop()

	// Run in non-parallel subtest so that server.Stop() runs AFTER all the subtests.
	t.Run("cases", func(t *testing.T) {
		t.Run("listServiceNames", func(t *testing.T) {
			test_client_ErrorResponses_listServiceNames(t, conn, errorStatus)
		})

		t.Run("fileDescriptorsBySymbols", func(t *testing.T) {
			test_client_ErrorResponses_fileDescriptorsBySymbols(t, conn, errorStatus)
		})

		t.Run("fileDescriptorsByFilenames", func(t *testing.T) {
			test_client_ErrorResponses_fileDescriptorsByFilenames(t, conn, errorStatus)
		})
	})
}

func test_client_ErrorResponses_listServiceNames(t *testing.T, conn grpcadapter.ClientConn, errorStatus *status.Status) {
	t.Parallel()

	// Arrange
	client := mustClient(t, conn)

	// Act
	_, err := client.listServiceNames()

	// Assert
	if cmpErr := bridgetest.StatusIs(err, errorStatus); cmpErr != nil {
		t.Errorf("listServiceNames() returned error = %q with unexpected status: %s", err, cmpErr)
	}
}

func test_client_ErrorResponses_fileDescriptorsBySymbols(t *testing.T, conn grpcadapter.ClientConn, errorStatus *status.Status) {
	t.Parallel()

	// Arrange
	client := mustClient(t, conn)

	// Act
	_, err := client.fileDescriptorsBySymbols([]protoreflect.FullName{"package.Service"})

	// Assert
	if cmpErr := bridgetest.StatusIs(err, errorStatus); cmpErr != nil {
		t.Errorf("fileDescriptorsBySymbols() returned error = %q with unexpected status: %s", err, cmpErr)
	}
}

func test_client_ErrorResponses_fileDescriptorsByFilenames(t *testing.T, conn grpcadapter.ClientConn, errorStatus *status.Status) {
	t.Parallel()

	// Arrange
	client := mustClient(t, conn)

	// Act
	_, err := client.fileDescriptorsByFilenames([]string{"file.proto"})

	// Assert
	if cmpErr := bridgetest.StatusIs(err, errorStatus); cmpErr != nil {
		t.Errorf("fileDescriptorsByFilenames() returned error = %q with unexpected status: %s", err, cmpErr)
	}
}

// Test_client_UnexpectedResponses tests that client methods properly handle responses containing unexpected message types.
func Test_client_UnexpectedResponses(t *testing.T) {
	t.Parallel()

	// AllExtensionNumbersResponse shouldn't be returned to any of the current client requests.
	reflectionServer := &staticReflectionServer{resp: &reflectionpb.ServerReflectionResponse{
		MessageResponse: &reflectionpb.ServerReflectionResponse_AllExtensionNumbersResponse{},
	}}

	server, _, conn := bridgetest.MustGRPCServer(t, func(s *grpc.Server) {
		reflectionpb.RegisterServerReflectionServer(s, reflectionServer)
	})
	defer server.Stop()

	// Run in non-parallel subtest so that server.Stop() runs AFTER all the subtests.
	t.Run("cases", func(t *testing.T) {
		t.Run("listServiceNames", func(t *testing.T) {
			test_client_UnexpectedResponses_listServiceNames(t, conn)
		})

		t.Run("fileDescriptorsBySymbols", func(t *testing.T) {
			test_client_UnexpectedResponses_fileDescriptorsBySymbols(t, conn)
		})

		t.Run("fileDescriptorsByFilenames", func(t *testing.T) {
			test_client_UnexpectedResponses_fileDescriptorsByFilenames(t, conn)
		})
	})
}

func test_client_UnexpectedResponses_listServiceNames(t *testing.T, conn grpcadapter.ClientConn) {
	t.Parallel()

	// Arrange
	client := mustClient(t, conn)

	// Act
	_, err := client.listServiceNames()

	// Assert
	if cmpErr := isFailedResponseCheck(err); cmpErr != nil {
		t.Errorf("listServiceNames() should've failed due to invalid response type: %s", cmpErr)
	}
}

func test_client_UnexpectedResponses_fileDescriptorsBySymbols(t *testing.T, conn grpcadapter.ClientConn) {
	t.Parallel()

	// Arrange
	client := mustClient(t, conn)

	// Act
	_, err := client.fileDescriptorsBySymbols([]protoreflect.FullName{"package.Service"})

	// Assert
	if cmpErr := isFailedResponseCheck(err); cmpErr != nil {
		t.Errorf("fileDescriptorsBySymbols() should've failed due to invalid response type: %s", cmpErr)
	}
}

func test_client_UnexpectedResponses_fileDescriptorsByFilenames(t *testing.T, conn grpcadapter.ClientConn) {
	t.Parallel()

	// Arrange
	client := mustClient(t, conn)

	// Act
	_, err := client.fileDescriptorsByFilenames([]string{"file.proto"})

	// Assert
	if cmpErr := isFailedResponseCheck(err); cmpErr != nil {
		t.Errorf("fileDescriptorsByFilenames() should've failed due to invalid response type: %s", cmpErr)
	}
}

func Test_client_SendErrors(t *testing.T) {
	t.Parallel()

	server, pool, _ := bridgetest.MustGRPCServer(t)
	defer server.Stop()

	// Return a client with a connection which is immediately closed.
	closedConnClient := func(t *testing.T, name string) *client {
		controller, _ := pool.New(name, bridgetest.TestDialTarget)
		conn, _ := pool.Get(name)
		client := mustClient(t, conn)
		controller.Close()
		return client
	}

	// Run in non-parallel subtest so that server.Stop() runs AFTER all the subtests.
	t.Run("cases", func(t *testing.T) {
		t.Run("listServiceNames", func(t *testing.T) {
			test_client_listServiceNames_SendErrors(t, closedConnClient)
		})

		t.Run("fileDescriptorsBySymbols", func(t *testing.T) {
			test_client_fileDescriptorsBySymbols_SendErrors(t, closedConnClient)
		})

		t.Run("fileDescriptorsByFilenames", func(t *testing.T) {
			test_client_fileDescriptorsByFilenames_SendErrors(t, closedConnClient)
		})
	})
}

func test_client_listServiceNames_SendErrors(t *testing.T, clientFunc func(t *testing.T, name string) *client) {
	t.Parallel()

	// Arrange
	client := clientFunc(t, "listServiceNames")

	// Act
	_, err := client.listServiceNames()

	// Assert
	if cmpErr := bridgetest.StatusCodeOneOf(err, codes.Canceled, codes.Unavailable); cmpErr != nil {
		t.Errorf("listServiceNames() returned error = %q with unexpected status: %s", err, cmpErr)
	}
}

func test_client_fileDescriptorsBySymbols_SendErrors(t *testing.T, clientFunc func(t *testing.T, name string) *client) {
	t.Parallel()

	// Arrange
	client := clientFunc(t, "fileDescriptorsBySymbols")

	// Act
	_, err := client.fileDescriptorsBySymbols([]protoreflect.FullName{"package.Service"})

	// Assert
	if !errors.Is(err, io.EOF) {
		t.Errorf("fileDescriptorsBySymbols() returned error = %q, want io.EOF due to closed connection", err)
	}
}

func test_client_fileDescriptorsByFilenames_SendErrors(t *testing.T, clientFunc func(t *testing.T, name string) *client) {
	t.Parallel()

	// Arrange
	client := clientFunc(t, "fileDescriptorsByFilenames")

	// Act
	_, err := client.fileDescriptorsByFilenames([]string{"file.proto"})

	// Assert
	if !errors.Is(err, io.EOF) {
		t.Errorf("fileDescriptorsByFilenames() returned error = %q, want io.EOF due to closed connection", err)
	}
}
