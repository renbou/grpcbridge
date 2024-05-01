package grpcadapter

import (
	"context"
	"time"
)

func ctxWithHalvedDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, time.Until(deadline)/2)
}
