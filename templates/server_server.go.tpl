package ${SERVICE}

import (
	"context"

	"github.com/jmoiron/sqlx"

	"${MODULE}/rpc/${SERVICE}"
)

type Server struct {
	db *sqlx.DB
}

var _ ${SERVICE}.${SERVICE_CAMEL}Service = &Server{}
