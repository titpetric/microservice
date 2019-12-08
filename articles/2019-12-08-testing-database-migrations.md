# Drone CI: Testing database migrations

Since we can now list our database migrations, the next logical step is testing
them out on a real database. For that we still need to implement our Run function.
Based on what we have in Print(), we only need to update the migrate() function.

~~~go
execQuery := func(idx int, query string, useLog bool) error {
	if useLog {
		log.Println()
		log.Println("-- Statement index:", idx)
		log.Println(query)
		log.Println()
	}
	if _, err := db.Exec(query); err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}
~~~

Our `printQuery` function adds database execution over the same output that we already know.
We skip the printing of the query if useLog = false, namely for the `migrations.sql` file.
We still need to track the statemend index in the `migration{}` struct, so we need to update
what we have in `migrate()`:

~~~go
migrate := func(filename string) error {
	log.Println("Running migrations from", filename)

	status := migration{
		Project:  project,
		Filename: filename,
	}

	// we can't log the main migrations table
	useLog := (filename != "migrations.sql")
	if useLog {
		if err := db.Get(&status, "select * from migrations where project=? and filename=?", status.Project, status.Filename); err != nil && err != sql.ErrNoRows {
			return err
		}
		if status.Status == "ok" {
			log.Println("Migrations already applied, skipping")
			return nil
		}
	}
~~~

In this section we set the initial migration structure, and fetch the status of it from the database.
If the migration status is "ok", this means the migration was already fully applied, and we can skip it.

~~~go
up := func() error {
	stmts, err := statements(fs.ReadFile(filename))
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Error reading migration: %s", filename))
	}

	for idx, stmt := range stmts {
		// skip stmt if it has already been applied
		if idx >= status.StatementIndex {
			status.StatementIndex = idx
			if err := execQuery(idx, stmt, useLog); err != nil {
				status.Status = err.Error()
				return err
			}
		}
	}
	status.Status = "ok"
	return nil
}
~~~

Just like before, we loop over the statements, and appply them if they haven't been applied yet.
In case a migration failed from some external cause (like the database going offline), we also
support resuming the migrations by checking the statement index. If the statement index is equal
or higher than the logged statement index, then the migration statement will be executed.

We probablly don't need this code, but, it also depends if you will keep a persistent database
for testing the migrations, where it's more likely that you might end up with a broken state,
especially while testing migrations for development purposes.

And finally:

~~~go
	err := up()
	if useLog {
		// log the migration status into the database
		set := func(fields []string) string {
			sql := make([]string, len(fields))
			for k, v := range fields {
				sql[k] = v + "=:" + v
			}
			return strings.Join(sql, ", ")
		}
		if _, err := db.NamedExec("replace into migrations set "+set(status.Fields()), status); err != nil {
			log.Println("Updating migration status failed:", err)
		}
	}
	return err
}
~~~

Here we update the migration status into the database, regardless if it produced an error or not.
The error or success is written in the `status.Status` field, and we update the complete record.

## Configuring Drone CI for database integration tests

It should be really simple to run the database integration tests from Drone. Let's configure
a database service, and a special makefile target to run the migrations.

~~~yaml
- name: migrations
  image: percona/percona-server:8.0.17
  user: root
  pull: always
  commands:
    - yum -q -y install make
    - "bash -c 'while : ; do  sleep 1 ; $(cat < /dev/null > /dev/tcp/mysql-test/3306) && break ; done' 2>/dev/null"
    - make migrate

services:
- name: mysql-test
  pull: always
  image: percona/percona-server:8.0.17
  ports:
    - 3306
  environment:
    MYSQL_ROOT_PASSWORD: default
~~~

Here we slightly hacked together support for running the migrations by:

1. installing `make` into the migration step, using the provided database container (all you need is a mysql client)
2. a bash-fu check using `/dev/tcp` to wait until our database service came online before running migrations
3. the database service itself, with a configured "default" mysql root password

And the Makefile target should look like this:

~~~Makefile
migrate: $(shell ls -d db/schema/*/migrations.sql | xargs -n1 dirname | sed -e 's/db.schema./migrate./')
	@echo OK.

migrate.%: export SERVICE = $*
migrate.%: export MYSQL_ROOT_PASSWORD = default
migrate.%:
	mysql -h mysql-test -u root -p$(MYSQL_ROOT_PASSWORD) -e "CREATE DATABASE $(SERVICE);"
	./build/db-migrate-cli-linux-amd64 -service $(SERVICE) -db-dsn "root:$(MYSQL_ROOT_PASSWORD)@tcp(mysql-test:3306)/$(SERVICE)" -real=true
	./build/db-migrate-cli-linux-amd64 -service $(SERVICE) -db-dsn "root:$(MYSQL_ROOT_PASSWORD)@tcp(mysql-test:3306)/$(SERVICE)" -real=true
~~~

The makefile target produces the database for our service, and then runs our db-migration program.
We still need to configure some flags for the program, but as you see, we pass the service name, the database
connection DSN, and the flag `-real=true` to actually execute the migrations on the database.

We run the migrations twice, so we make sure that our migration status is logged correctly as well.
All the migrations in the second run must be skipped.

Let's add support for this into `cmd/db-migrate-cli/main.go`:

~~~go
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
			if err := db.Run(service.Driver, handle); err != nil {
				log.Fatalf("An error occured: %+v", err)
			}
		}
	default:
		if err := db.Print(service.Driver); err != nil {
			log.Fatalf("An error occured: %+v", err)
		}
	}
}
~~~

Here we do a couple of things:

- import the mysql database driver
- add configuration options with the standard library flag package,
- print services if no service is passed
- print migrations if real=false,
- connect to database and run migrations if real=true

All that's left to see is if our migrations execute. Let's take a look at the output of our migration target:

~~~plaintext
[migrations:0] + yum -q -y install make
[migrations:1] + bash -c 'while : ; do  sleep 1 ; $(cat < /dev/null > /dev/tcp/mysql-test/3306) && break ; done' 2>/dev/null
[migrations:2] + make migrate
[migrations:3] mysql -h mysql-test -u root -pdefault -e "CREATE DATABASE stats;"
[migrations:4] mysql: [Warning] Using a password on the command line interface can be insecure.
[migrations:5] ./build/db-migrate-cli-linux-amd64 -service stats -db-dsn "root:default@tcp(mysql-test:3306)/stats" -real=true
[migrations:6] 2019/11/26 11:22:06 Running migrations from migrations.sql
[migrations:7] 2019/11/26 11:22:06 Running migrations from 2019-11-26-092610-description-here.up.sql
[migrations:8] 2019/11/26 11:22:06
[migrations:9] 2019/11/26 11:22:06 -- Statement index: 0
[migrations:10] 2019/11/26 11:22:06 -- Hello world
[migrations:11] 2019/11/26 11:22:06
[migrations:12] ./build/db-migrate-cli-linux-amd64 -service stats -db-dsn "root:default@tcp(mysql-test:3306)/stats" -real=true
[migrations:13] 2019/11/26 11:22:06 Running migrations from migrations.sql
[migrations:14] 2019/11/26 11:22:06 Running migrations from 2019-11-26-092610-description-here.up.sql
[migrations:15] 2019/11/26 11:22:06 Migrations already applied, skipping
[migrations:16] OK.
~~~

Great success - the migrations are executed, and the second migration run doesn't end up with an error.
We can see that the migration is skipped, as it has already been applied to the database.