package stats

import (
	"github.com/jmoiron/sqlx"
	"github.com/sony/sonyflake"

	"github.com/titpetric/microservice/rpc/stats"
)

// Server implements stats.StatsService
type Server struct {
	db *sqlx.DB

	sonyflake *sonyflake.Sonyflake
	flusher   *Flusher
}

// Shutdown is a cleanup hook after SIGTERM
func (s *Server) Shutdown() {
	<-s.flusher.Done()
}

var _ stats.StatsService = &Server{}
