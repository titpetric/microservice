//+build wireinject

package ${SERVICE}

import (
	"context"

	"github.com/google/wire"

	"${MODULE}/inject"
)

func New(ctx context.Context) (*Server, error) {
	wire.Build(
		inject.Inject,
		wire.Struct(new(Server), "*"),
	)
	return nil, nil
}
