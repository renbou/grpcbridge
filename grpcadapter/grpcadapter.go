package grpcadapter

import (
	"context"
	"errors"

	"google.golang.org/protobuf/proto"
)

var ErrAlreadyDialed = errors.New("connection already dialed")

var (
	_ ClientConn   = (*AdaptedClientConn)(nil)
	_ ClientStream = (*AdaptedClientStream)(nil)
)

type ClientPool interface {
	Get(target string) (ClientConn, bool)
}

type ClientConn interface {
	Stream(ctx context.Context, method string) (ClientStream, error)
	Close()
}

type ClientStream interface {
	Send(context.Context, proto.Message) error
	Recv(context.Context, proto.Message) error
	CloseSend()
	Close()
}

type ServerStream interface {
	Send(context.Context, proto.Message) error
	Recv(context.Context, proto.Message) error
}
