package grpcadapter

import (
	"context"
	"errors"

	"github.com/renbou/grpcbridge/bridgedesc"
	"google.golang.org/grpc/metadata"
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
	Header() metadata.MD
	Trailer() metadata.MD
	CloseSend()
	Close()
}

type ServerStream interface {
	Send(context.Context, proto.Message) error
	Recv(context.Context, proto.Message) error
	SetHeader(metadata.MD)
	SetTrailer(metadata.MD)
}

type ForwardParams struct {
	Target  *bridgedesc.Target
	Service *bridgedesc.Service
	Method  *bridgedesc.Method

	Incoming ServerStream
	Outgoing ClientConn
}

type Forwarder interface {
	Forward(context.Context, ForwardParams) error
}

type MetadataFilter interface {
	FilterRequestMD(metadata.MD) metadata.MD
	FilterResponseMD(metadata.MD) metadata.MD
	FilterTrailerMD(metadata.MD) metadata.MD
}
