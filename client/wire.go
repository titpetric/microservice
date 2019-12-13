package client

import (
	"github.com/google/wire"

	"github.com/titpetric/microservice/client/stats"
)

// Inject produces a wire.ProviderSet with our RPC clients
var Inject = wire.NewSet(
	stats.New,
)
