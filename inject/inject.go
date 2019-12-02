package inject

import (
	"github.com/google/wire"

	"github.com/titpetric/microservice/client"
	"github.com/titpetric/microservice/db"
)

var Inject = wire.NewSet(db.Connect, client.Inject)
