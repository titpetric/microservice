package stats

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/titpetric/microservice/rpc/stats"
)

type Server struct {
	db *sqlx.DB
}

var _ stats.StatsService = &Server{}

func (svc *Server) Push(_ context.Context, _ *stats.PushRequest) (*stats.PushResponse, error) {
	panic("not implemented") // TODO: Implement
}
