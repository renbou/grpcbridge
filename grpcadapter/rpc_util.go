package grpcadapter

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ctxRPCErr works like toRPCErr in rpc_util.go from grpc.
func ctxRPCErr(err error) error {
	switch err {
	case context.Canceled:
		return status.Error(codes.Canceled, context.Canceled.Error())
	case context.DeadlineExceeded:
		return status.Error(codes.DeadlineExceeded, context.DeadlineExceeded.Error())
	}

	return status.Error(codes.Unknown, err.Error())
}
