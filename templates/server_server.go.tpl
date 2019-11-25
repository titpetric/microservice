package ${SERVICE}

import (
	"context"

	"${MODULE}/rpc/${SERVICE}"
)

type Server struct {
}

func New(ctx context.Context) (*Server, error) {
	return &Server{}, nil
}

var _ ${SERVICE}.${SERVICE_CAMEL}Service = &Server{}
