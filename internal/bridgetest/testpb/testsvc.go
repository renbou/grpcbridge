package testpb

import (
	context "context"

	"google.golang.org/protobuf/proto"
)

func PrepareResponse[T proto.Message](response T, err error) *PreparedResponse[T] {
	return &PreparedResponse[T]{Response: response, Error: err}
}

type PreparedResponse[T proto.Message] struct {
	Response T
	Error    error
}

// TestService implements the test gRPC service with handlers which simply record the requests and return predetermined responses.
type TestService struct {
	UnimplementedTestServiceServer
	UnaryBoundRequest  *Scalars
	UnaryBoundResponse *PreparedResponse[*Combined]
}

func (s *TestService) UnaryBound(ctx context.Context, req *Scalars) (*Combined, error) {
	s.UnaryBoundRequest = req
	return s.UnaryBoundResponse.Response, s.UnaryBoundResponse.Error
}

func (s *TestService) BadResponsePath(context.Context, *Scalars) (*Combined, error) {
	return new(Combined), nil
}
