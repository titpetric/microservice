package inject

import (
	"context"

	"github.com/twitchtv/twirp"
	"go.elastic.co/apm"
)

func NewServerHooks() *twirp.ServerHooks {
	return &twirp.ServerHooks{
		Error: func(ctx context.Context, err twirp.Error) context.Context {
			apm.CaptureError(ctx, err).Send()
			return ctx
		},
	}
}
