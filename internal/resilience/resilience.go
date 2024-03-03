package resilience

import (
	"context"
	"sync"
	"time"
)

type ContextCanceler struct {
	timeout     time.Duration
	cancelFunc  func()      // wrapper using sync.OnceFunc
	cancelTimer *time.Timer // from time.AfterFunc
}

func ContextWithCanceler(ctx context.Context, timeout time.Duration) (context.Context, *ContextCanceler) {
	ctx, cancel := context.WithCancel(ctx)

	// wrap using OnceFunc to avoid constantly re-checking the timer during resets
	cancel = sync.OnceFunc(cancel)

	return ctx, &ContextCanceler{
		timeout:     timeout,
		cancelFunc:  cancel,
		cancelTimer: time.AfterFunc(timeout, cancel),
	}
}

func (c *ContextCanceler) Stop() {
	c.cancelTimer.Stop()
}

func (c *ContextCanceler) Reset() {
	c.cancelTimer.Reset(c.timeout)
}

func (c *ContextCanceler) Cancel() {
	// No need to drain cancelTimer.C here as described in the time.AfterFunc docs.
	c.cancelTimer.Stop()
	c.cancelFunc()
}
