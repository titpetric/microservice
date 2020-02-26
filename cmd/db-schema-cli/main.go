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
		service string
		schema  string
		format  string
		output  string
		drop    bool
	}
	flag.StringVar(&config.db.Driver, "db-driver", "mysql", "Database driver")
	flag.StringVar(&config.db.DSN, "db-dsn", "", "DSN for database connection")
	flag.StringVar(&config.schema, "schema", "", "Schema name to print tables for")
	flag.StringVar(&config.service, "service", "", "Service name to generate tables in")
	flag.StringVar(&config.format, "format", "go", "Output formatting")
	flag.StringVar(&config.output, "output", "", "Output folder (mandatory)")
	flag.BoolVar(&config.drop, "drop", false, "Drop tables in schema")
	flag.Parse()

	if config.output == "" && !config.drop {
		log.Fatal("Missing -output parameter, please specify output folder")
	}

	handle, err := sqlx.Connect(config.db.Driver, config.db.DSN)
	if err != nil {
		log.Fatalf("Error connecting to database: %+v", err)
	}

	// List tables in schema
	tables := []*Table{}
	fields := strings.Join(TableFields, ", ")
	err = handle.Select(&tables, "select "+fields+" from information_schema.tables where table_schema=? order by table_name asc", config.schema)
	if err != nil {
		log.Println("Error listing database tables")
		log.Fatal(err)
	}

	// Drop all tables in schema
	if config.drop {
		for _, table := range tables {
			query := "DROP TABLE `" + table.Name + "`"
			log.Println(query)
			if _, err := handle.Exec(query); err != nil {
				log.Fatal(err)
			}
		}
		return
	}

	// List columns in tables
	for _, table := range tables {
		fields := strings.Join(ColumnFields, ", ")
		err := handle.Select(&table.Columns, "select "+fields+" from information_schema.columns where table_schema=? and table_name=? order by ordinal_position asc", config.schema, table.Name)
		if err != nil {
			log.Println("Error listing database columns for table:", table.Name)
			log.Fatal(err)
		}
	}

	// Render go structs
	if config.format == "go" {
		if err := renderGo(config.output, config.service, tables); err != nil {
			log.Fatal(err)
		}
	}

	// Render markdown tables
	if config.format == "markdown" {
		if err := renderMarkdown(config.output, config.service, tables); err != nil {
			log.Fatal(err)
		}
	}
}
