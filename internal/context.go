package internal

import (
	"context"
)

type ctxWithoutCancel struct {
	context.Context
}

// Done returns nil (not cancellable)
func (c ctxWithoutCancel) Done() <-chan struct{} { return nil }

// Err returns nil (not cancellable)
func (c ctxWithoutCancel) Err() error { return nil }

// ContextWithoutCancel returns a non-cancelable ctx
func ContextWithoutCancel(ctx context.Context) context.Context {
	return ctxWithoutCancel{ctx}
}
