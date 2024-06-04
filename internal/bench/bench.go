package bench

import context "context"

type BenchService struct {
	UnimplementedBenchServer
}

func (s *BenchService) Echo(ctx context.Context, req *EchoMessage) (*EchoMessage, error) {
	return req, nil
}
