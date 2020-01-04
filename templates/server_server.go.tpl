package ${SERVICE}

import (
	"context"

	"github.com/jmoiron/sqlx"

	"${MODULE}/rpc/${SERVICE}"
)

// Server implements ${SERVICE}.${SERVICE_CAMEL}
type Server struct {
	db *sqlx.DB
}

// Shutdown is a cleanup hook after SIGTERM
func (*Server) Shutdown() {
}

var _ ${SERVICE}.${SERVICE_CAMEL}Service = &Server{}
