package inject

import (
	"github.com/google/wire"

	"github.com/titpetric/microservice/client"
	"github.com/titpetric/microservice/db"
)

// Inject is the main ProviderSet for wire
var Inject = wire.NewSet(
	db.Connect,
	Sonyflake,
	client.Inject,
)
