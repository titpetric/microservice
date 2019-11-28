package main

import (
	"flag"
	"log"

	_ "github.com/go-sql-driver/mysql"

	"github.com/jmoiron/sqlx"
	"github.com/titpetric/microservice/db"
)

func main() {
	var config struct {
		db struct {
			DSN    string
			Driver string
		}
		Real    bool
		Service string
	}
	flag.StringVar(&config.db.Driver, "db-driver", "mysql", "Database driver")
	flag.StringVar(&config.db.DSN, "db-dsn", "", "DSN for database connection")
	flag.StringVar(&config.Service, "service", "", "Service name for migrations")
	flag.BoolVar(&config.Real, "real", false, "false = print migrations, true = run migrations")
	flag.Parse()

	if config.Service == "" {
		log.Printf("Available migration services: %+v", db.List())
		log.Fatal()
	}

	switch config.Real {
	case true:
		if handle, err := sqlx.Connect(config.db.Driver, config.db.DSN); err != nil {
			log.Fatalf("Error connecting to database: %+v", err)
		} else {
			if err := db.Run("stats", handle); err != nil {
				log.Fatalf("An error occured: %+v", err)
			}
		}
	default:
		if err := db.Print("stats"); err != nil {
			log.Fatalf("An error occured: %+v", err)
		}
	}
}
