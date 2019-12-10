package main

import (
	"flag"
	"log"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/jmoiron/sqlx"
)

func main() {
	var config struct {
		db struct {
			DSN    string
			Driver string
		}
		Schema string
		Format string
		Output string
	}
	flag.StringVar(&config.db.Driver, "db-driver", "mysql", "Database driver")
	flag.StringVar(&config.db.DSN, "db-dsn", "", "DSN for database connection")
	flag.StringVar(&config.Schema, "schema", "", "Schema name to print tables for")
	flag.StringVar(&config.Format, "format", "go", "Output formatting")
	flag.StringVar(&config.Output, "output", "", "Output folder (mandatory)")
	flag.Parse()

	if config.Output == "" {
		log.Fatal("Missing -output parameter, please specify output folder")
	}

	handle, err := sqlx.Connect(config.db.Driver, config.db.DSN)
	if err != nil {
		log.Fatalf("Error connecting to database: %+v", err)
	}

	// List tables in schema
	tables := []*Table{}
	fields := strings.Join(TableFields, ", ")
	err = handle.Select(&tables, "select "+fields+" from information_schema.tables where table_schema=? order by table_name asc", config.Schema)
	if err != nil {
		log.Println("Error listing database tables")
		log.Fatal(err)
	}

	// List columns in tables
	for _, table := range tables {
		fields := strings.Join(ColumnFields, ", ")
		err := handle.Select(&table.Columns, "select "+fields+" from information_schema.columns where table_schema=? and table_name=? order by ordinal_position asc", config.Schema, table.Name)
		if err != nil {
			log.Println("Error listing database columns for table:", table.Name)
			log.Fatal(err)
		}
	}

	// Render go structs
	if config.Format == "go" {
		if err := renderGo(config.Output, config.Schema, tables); err != nil {
			log.Fatal(err)
		}
	}

	// Render markdown tables
	if config.Format == "markdown" {
		if err := renderMarkdown(config.Output, config.Schema, tables); err != nil {
			log.Fatal(err)
		}
	}
}
