//+build wireinject

package stats

import (
	"context"

	"github.com/google/wire"

	"github.com/titpetric/microservice/inject"
)

func New(ctx context.Context) (*Server, error) {
	wire.Build(
		inject.Inject,
		wire.Struct(new(Server), "*"),
	)
	return nil, nil
}
