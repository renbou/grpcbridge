package grpcadapter

import (
	"context"

	"google.golang.org/protobuf/proto"
)

var _ Connection = (*wrappedConnection[*AdaptedBiDiStream])(nil)

type Connection interface {
	Close()
	BiDiStream(ctx context.Context, method string) (BiDiStream, error)
}

type BiDiStream interface {
	Send(context.Context, proto.Message) error
	Recv(context.Context, proto.Message) error
	CloseSend()
	Close()
}

type GenericConnection[BD BiDiStream] interface {
	Close()
	BiDiStream(ctx context.Context, method string) (BD, error)
}

func WrapConnection[BD BiDiStream](conn GenericConnection[BD]) Connection {
	return &wrappedConnection[BD]{conn}
}

type wrappedConnection[BD BiDiStream] struct {
	conn GenericConnection[BD]
}

func (c *wrappedConnection[BD]) Close() {
	c.conn.Close()
}

func (c *wrappedConnection[BD]) BiDiStream(ctx context.Context, method string) (BiDiStream, error) {
	return c.conn.BiDiStream(ctx, method)
}
