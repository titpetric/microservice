package client

import (
	"github.com/google/wire"

	"github.com/titpetric/microservice/client/stats"
)

var Inject = wire.NewSet(
	stats.New,
)
