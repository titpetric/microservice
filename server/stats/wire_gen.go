// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package stats

import (
	"context"
	"github.com/titpetric/microservice/db"
)

// Injectors from wire.go:

func New(ctx context.Context) (*Server, error) {
	sqlxDB, err := db.Connect()
	if err != nil {
		return nil, err
	}
	server := &Server{
		db: sqlxDB,
	}
	return server, nil
}