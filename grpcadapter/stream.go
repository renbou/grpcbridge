package grpcadapter

import (
	"context"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type BiDiStream struct {
	closeFunc func() // cancelFunc from context used to initialize stream, wrapped with sync.Once
	stream    grpc.ClientStream
	recvOps   atomic.Int32
	sendOps   atomic.Int32
}

func (s *BiDiStream) Send(ctx context.Context, msg proto.Message) error {
	if s.sendOps.Add(1) > 1 {
		panic("grpcbridge: Send() called concurrently on gRPC client stream")
	}
	defer s.sendOps.Add(-1)

	return s.withCtx(ctx, func() error { return s.stream.SendMsg(msg) })
}

func (s *BiDiStream) Recv(ctx context.Context, msg proto.Message) error {
	if s.recvOps.Add(1) > 1 {
		panic("grpcbridge: Recv() called concurrently on gRPC client stream")
	}
	defer s.recvOps.Add(-1)

	return s.withCtx(ctx, func() error { return s.stream.RecvMsg(msg) })
}

func (s *BiDiStream) Close() {
	s.closeFunc()
}

func (s *BiDiStream) withCtx(ctx context.Context, f func() error) error {
	errChan := make(chan error, 1)
	go func() {
		errChan <- f()
	}()

	select {
	case <-ctx.Done():
		// Automatically close the stream, because currently send()/recv() calls aren't synchronized enough
		// to guarantee that SendMsg/RecvMsg won't be called concurrently.
		s.Close()
		return ctxRPCErr(ctx.Err())
	case err := <-errChan:
		return err
	}
}
