package grpcadapter

import (
	"context"

	"google.golang.org/protobuf/proto"
)

var _ ClientConn = (*wrappedConnection[*AdaptedBiDiStream])(nil)

type ClientConn interface {
	Close()
	BiDiStream(ctx context.Context, method string) (BiDiStream, error)
}

type BiDiStream interface {
	Send(context.Context, proto.Message) error
	Recv(context.Context, proto.Message) error
	CloseSend()
	Close()
}

type GenericClientConn[BD BiDiStream] interface {
	Close()
	BiDiStream(ctx context.Context, method string) (BD, error)
}

func WrapClientConn[BD BiDiStream](conn GenericClientConn[BD]) ClientConn {
	return &wrappedConnection[BD]{conn}
}

type wrappedConnection[BD BiDiStream] struct {
	conn GenericClientConn[BD]
}

func (c *wrappedConnection[BD]) Close() {
	c.conn.Close()
}

func (c *wrappedConnection[BD]) BiDiStream(ctx context.Context, method string) (BiDiStream, error) {
	return c.conn.BiDiStream(ctx, method)
}
