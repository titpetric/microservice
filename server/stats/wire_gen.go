// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package stats

import (
	"context"
	"github.com/titpetric/microservice/db"
	"github.com/titpetric/microservice/inject"
)

// Injectors from wire.go:

func New(ctx context.Context) (*Server, error) {
	sqlxDB, err := db.Connect(ctx)
	if err != nil {
		return nil, err
	}
	sonyflake := inject.Sonyflake()
	server := &Server{
		db:        sqlxDB,
		sonyflake: sonyflake,
	}
	return server, nil
}
