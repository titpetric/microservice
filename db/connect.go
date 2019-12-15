package db

import (
	"context"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// Connect connects to a database and produces the handle for injection
func Connect(ctx context.Context) (*sqlx.DB, error) {
	options := ConnectionOptions{}
	options.Credentials.DSN = os.Getenv("DB_DSN")
	options.Credentials.DriverName = os.Getenv("DB_DRIVER")
	return ConnectWithOptions(ctx, options)
}

// ConnectWithOptions connect to host based on ConnectionOptions{}
func ConnectWithOptions(ctx context.Context, options ConnectionOptions) (*sqlx.DB, error) {
	credentials := options.Credentials
	if credentials.DSN == "" {
		return nil, errors.New("DSN not provided")
	}
	if credentials.DriverName == "" {
		credentials.DriverName = "mysql"
	}
	credentials.DSN = cleanDSN(credentials.DSN)
	if options.Connector != nil {
		handle, err := options.Connector(ctx, credentials)
		if err == nil {
			return sqlx.NewDb(handle, credentials.DriverName), nil
		}
		return nil, errors.WithStack(err)
	}
	return sqlx.ConnectContext(ctx, credentials.DriverName, credentials.DSN)
}
