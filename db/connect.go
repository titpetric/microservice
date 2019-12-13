package db

import (
	"errors"
	"os"

	"github.com/jmoiron/sqlx"
)

// Connect connects to a database and produces the handle for injection
func Connect() (*sqlx.DB, error) {
	dsn := os.Getenv("DB_DSN")
	driver := os.Getenv("DB_DRIVER")
	if dsn == "" {
		return nil, errors.New("DB_DSN not provided")
	}
	if driver == "" {
		driver = "mysql"
	}
	return sqlx.Connect(driver, dsn)
}
