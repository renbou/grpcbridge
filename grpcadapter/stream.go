package grpcadapter

import (
	"context"
	"sync/atomic"

	"github.com/renbou/grpcbridge/internal/rpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type AdaptedClientStream struct {
	closeFunc  func() // cancelFunc from context used to initialize stream, wrapped with sync.Once
	stream     grpc.ClientStream
	sendActive atomic.Bool
	recvActive atomic.Bool
}

func (s *AdaptedClientStream) Send(ctx context.Context, msg proto.Message) error {
	if !s.sendActive.CompareAndSwap(false, true) {
		panic("grpcbridge: Send() called concurrently on gRPC client stream")
	}
	defer s.sendActive.Store(false)

	return s.withCtx(ctx, func() error { return s.stream.SendMsg(msg) })
}

func (s *AdaptedClientStream) Recv(ctx context.Context, msg proto.Message) error {
	if !s.recvActive.CompareAndSwap(false, true) {
		panic("grpcbridge: Recv() called concurrently on gRPC client stream")
	}
	defer s.recvActive.Store(false)

	return s.withCtx(ctx, func() error { return s.stream.RecvMsg(msg) })
}

func (s *AdaptedClientStream) Header() metadata.MD {
	md, _ := s.stream.Header()
	return md
}

func (s *AdaptedClientStream) Trailer() metadata.MD {
	return s.stream.Trailer()
}

func (s *AdaptedClientStream) CloseSend() {
	// never returns an error, and we don't care about it anyway, just like with Close()
	_ = s.stream.CloseSend()
}

func (s *AdaptedClientStream) Close() {
	s.closeFunc()
}

// TODO(renbou): this and all ServerStream implementations can be optimized to use a queue instead of running a new goroutine each time.
// Not sure though if this will help that much, which is why it's currently implemented this way.
func (s *AdaptedClientStream) withCtx(ctx context.Context, f func() error) error {
	errChan := make(chan error, 1)
	go func() {
		errChan <- f()
	}()

	select {
	case <-ctx.Done():
		// Automatically close the stream, because currently send()/recv() calls aren't synchronized enough
		// to guarantee that SendMsg/RecvMsg won't be called concurrently.
		s.Close()
		return rpcutil.ContextError(ctx.Err())
	case err := <-errChan:
		return err
	}
}
