package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	testapiv1 "testapi/gen/go/proto/testapi/v1"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type TestService struct {
	testapiv1.UnimplementedTestServiceServer
}

func (s *TestService) UnaryEcho(ctx context.Context, req *testapiv1.UnaryEchoRequest) (*testapiv1.UnaryEchoResponse, error) {
	return &testapiv1.UnaryEchoResponse{
		Message: req.Message,
	}, nil
}

func (s *TestService) ClientStreamEcho(stream testapiv1.TestService_ClientStreamEchoServer) error {
	var messages []string

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&testapiv1.ClientStreamEchoResponse{
				Accumulated: messages,
			})
		} else if err != nil {
			return status.Errorf(codes.Internal, "receiving client message: %s", err)
		}

		messages = append(messages, req.Messages...)
	}
}

func (s *TestService) ServerStreamEcho(req *testapiv1.ServerStreamEchoRequest, stream testapiv1.TestService_ServerStreamEchoServer) error {
	for i := range req.Repeat {
		if err := stream.Send(&testapiv1.ServerStreamEchoResponse{
			Message: req.Message,
			Index:   int32(i),
		}); err != nil {
			return status.Errorf(codes.Internal, "sending server stream message: %s", err)
		}
	}

	return nil
}

func (s *TestService) BiDiStreamEcho(stream testapiv1.TestService_BiDiStreamEchoServer) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return status.Errorf(codes.Internal, "receiving client message: %s", err)
		}

		response := &testapiv1.BiDiStreamEchoResponse{}

		switch x := req.Message.(type) {
		case *testapiv1.BiDiStreamEchoRequest_Text:
			response.Message = &testapiv1.BiDiStreamEchoResponse_Text{Text: x.Text}
		case *testapiv1.BiDiStreamEchoRequest_Data:
			response.Message = &testapiv1.BiDiStreamEchoResponse_Data{Data: x.Data}
		default:
			return status.Errorf(codes.InvalidArgument, "unknown message type %T", x)
		}

		if err := stream.Send(response); err != nil {
			return status.Errorf(codes.Internal, "sending bi-di stream message: %s", err)
		}
	}
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	listenAddr := ":50051"
	if value, ok := os.LookupEnv("LISTEN_ADDR"); ok {
		listenAddr = value
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fatalf("failed to listen", err)
	}

	gRPCLogger := logging.LoggerFunc(func(ctx context.Context, level logging.Level, msg string, fields ...any) {
		// slog.Level conversion here is ok because logging.Level is defined with the same constants
		slog.Log(ctx, slog.Level(level), msg, fields...)
	})

	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(logging.UnaryServerInterceptor(gRPCLogger)),
		grpc.ChainStreamInterceptor(logging.StreamServerInterceptor(gRPCLogger)),
	)

	// Register only v1 gRPC reflection. grpcbridge should properly handle such cases.
	testapiv1.RegisterTestServiceServer(server, &TestService{})
	reflection.RegisterV1(server)

	go func() {
		slog.Info("serving test gRPC", "listen_addr", listenAddr)
		if err := server.Serve(lis); err != nil {
			fatalf("failed to serve", err)
		}
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	<-shutdownCh

	slog.Warn("received termination signal, performing graceful shutdown")

	// Initiate graceful shutdown in separate goroutine, then,
	// after a timeout, force the server stop.
	gracefulCh := make(chan struct{})
	go func() {
		defer close(gracefulCh)
		server.GracefulStop()
	}()

	select {
	case <-time.After(time.Second * 3):
	case <-gracefulCh:
	}
	server.Stop()
}

func fatalf(msg string, err error) {
	slog.Error(fmt.Sprintf("%s: %s", msg, err))
	os.Exit(1)
}
