# Improving our database handling

As we start to develop real services, we found out that our database
handling needs some improvements. We are going to do a series of
improvements of the current state by adding some new features, and
restructuring some existing ones.

## Data source name - DSN

We aren't doing any kind of data source handling so far, but we need
to inject some options into the DSN, like the following:

- `parseTime=true` - needed for decoding date/datetime values into a `time.Time`, instead of `[]byte`,
- `loc=Local` - set the location for time.Time values, [see time.LoadLocation](https://golang.org/pkg/time/#LoadLocation),
- `collation=utf8mb4_general_ci` - set the default collation (utf8mb4 is a given these days if you want emojis)

Create `db/dsn.go`:

~~~go
package db

import "strings"

func cleanDSN(dsn string) string {
	dsn = addOptionToDSN(dsn, "?", "?")
	dsn = addOptionToDSN(dsn, "collation=", "&collation=utf8_general_ci")
	dsn = addOptionToDSN(dsn, "parseTime=", "&parseTime=true")
	dsn = addOptionToDSN(dsn, "loc=", "&loc=Local")
	dsn = strings.Replace(dsn, "?&", "?", 1)
	return dsn
}

func addOptionToDSN(dsn, match, option string) string {
	if !strings.Contains(dsn, match) {
		dsn += option
	}
	return dsn
}
~~~

The `cleanDSN` function will append default options to the dsn, if they aren't
already provided. If you will specify `?loc=UTC` to your microservice DSN environment,
that option will be used, instead of the refault `loc=Local`.

We would also like to output the DSN into the logs, without outputing the credentials.
We'll resort to a simple regular expression to mask the DSN username and password.
Create `db/dsn_mask.go`:

~~~go
package db

import "regexp"
)

var dsnMasker = regexp.MustCompile("(.)(?:.*)(.):(.)(?:.*)(.)@")

func maskDSN(dsn string) string {
	return dsnMasker.ReplaceAllString(dsn, "$1****$2:$3****$4@")
}
~~~

The regular expression takes care of masking any number of characters in the
username and password part of the DSN, outputting only the first and last characters
of each, with 4 asterisks in between.

## Multiple database connections

As we figured out, we want to run the migrations for our app from the app itself.
In order for migrations to work, they need extended privileges, which aren't usually
needed or wanted in a microservice.

A microservice usually needs permissions to select, insert, update and delete records,
while a migration needs permissions to create, alter, index and possibly drop tables.
The [full list of grants](https://dev.mysql.com/doc/refman/8.0/en/grant.html#grant-privileges)
is quite long, and adding them all to the account your microservice might be using is
a security issue.

To sum it up - we will need to provide two DSN credentials, one for migrations and
one for the microservice. By doing this we are enabling a security barrier between the
service, and between the migrations which have elevated permissions.

To enable multiple database configurations, we will add option types (`db/types.go`):

~~~go
package db

import (
	"context"
	"time"

	"database/sql"
)

type (
	// Credentials contains DSN and Driver
	Credentials struct {
		DSN    string
		Driver string
	}

	// ConnectionOptions include common connection options
	ConnectionOptions struct {
		Credentials Credentials

		// Connector is an optional parameter to produce our
		// own *sql.DB, which is then wrapped in *sqlx.DB
		Connector func(context.Context, Credentials) (*sql.DB, error)

		Retries        int
		RetryDelay     time.Duration
		ConnectTimeout time.Duration
	}
)
~~~

So, to connect to any database, a client needs to provide the ConnectionOptions{},
with an optional Connector that produces an *sql.DB. The connector is optional and
will be used to instrument our sql client without having to modify or wrap the existing
functions, but by replacing the driver in use. We'll come back to this later.

## Updating database connections

With the introduced ConnectionOptions struct, we have options to configure
connection retry. The Retries gives us a maximum connection try count, RetryDelay
gives us a delay duration after particular connection attempt, while the ConnectTimeout
gives us a global cancellation timeout.

Let's modify our main Connect function to use the new structures, and add a ConnectWithOptions
call that produces the same result as Connect:

~~~go
// Connect connects to a database and produces the handle for injection
func Connect(ctx context.Context) (*sqlx.DB, error) {
	options := ConnectionOptions{}
	options.Credentials.DSN = os.Getenv("DB_DSN")
	options.Credentials.Driver = os.Getenv("DB_DRIVER")
	return ConnectWithOptions(ctx, options)
}

// ConnectWithOptions connect to host based on ConnectionOptions{}
func ConnectWithOptions(ctx context.Context, options ConnectionOptions) (*sqlx.DB, error) {
	credentials := options.Credentials
	if credentials.DSN == "" {
		return nil, errors.New("DSN not provided")
	}
	if credentials.Driver == "" {
		credentials.Driver = "mysql"
	}
	credentials.DSN = cleanDSN(credentials.DSN)
	if options.Connector != nil {
		handle, err := options.Connector(ctx, credentials)
		if err == nil {
			return sqlx.NewDb(handle, credentials.Driver), nil
		}
		return nil, errors.WithStack(err)
	}
	return sqlx.ConnectContext(ctx, credentials.Driver, credentials.DSN)
}
~~~

Notable changes:

- we now take a `context.Context`, so we can support cancellation (CTRL+C) from main(),
- DSN validation was moved into ConnectWithOptions()
- We implemented the connector code to wrap a *sql.DB into *sqlx.DB

Re-runing wire picks up our new `Connect` signature:

~~~diff
--- a/server/stats/wire_gen.go
+++ b/server/stats/wire_gen.go
@@ -14,7 +14,7 @@ import (
 // Injectors from wire.go:
 
 func New(ctx context.Context) (*Server, error) {
-       sqlxDB, err := db.Connect()
+       sqlxDB, err := db.Connect(ctx)
        if err != nil {
                return nil, err
        }
~~~

We can now move to implement connection retrying.

## Database connection retry

Let's create a `db/connector.go` file with our retry logic:

~~~go
package db

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// ConnectWithRetry uses retry options set in ConnectionOptions{}
func ConnectWithRetry(ctx context.Context, options ConnectionOptions) (db *sqlx.DB, err error) {
	dsn := maskDSN(options.Credentials.DSN)

	// by default, retry for 5 minutes, 5 seconds between retries
	if options.Retries == 0 && options.ConnectTimeout.Seconds() == 0 {
		options.ConnectTimeout = 5 * time.Minute
		options.RetryDelay = 5 * time.Second
	}

	connErrCh := make(chan error, 1)
	defer close(connErrCh)

	log.Println("connecting to database", dsn)

	go func() {
		try := 0
		for {
			try++
			if options.Retries > 0 && options.Retries <= try {
				err = errors.Errorf("could not connect, dsn=%s, tries=%d", dsn, try)
				break
			}

			db, err = ConnectWithOptions(ctx, options)
			if err != nil {
				log.Printf("can't connect, dsn=%s, err=%s, try=%d", dsn, err, try)

				select {
				case <-ctx.Done():
					break
				case <-time.After(options.RetryDelay):
					continue
				}
			}
			break
		}
		connErrCh <- err
	}()

	select {
	case err = <-connErrCh:
		break
	case <-time.After(options.ConnectTimeout):
		return nil, errors.Errorf("db connect timed out, dsn=%s", dsn)
	case <-ctx.Done():
		return nil, errors.Errorf("db connection cancelled, dsn=%s", dsn)
	}

	return
}
~~~

I tried to be brief here, but let's go over the notable parts:

- a goroutine is created to loop and retry connecting to the database,
- an error channel is created to recieve the final error from the goroutine,
- we are using named returns to keep the SLOC down somewhat,
- a final select statement listens for:
  - context cancellation (e.g. signal from outside)
  - overall connect timeout option as an event,
  - the exit from the gorutine (connErrCh)

Now, I'm pretty sure there's an error in there. Let me ask you a question:
What happens to the goroutine if the global ConnectTimeout is reached?

## Migration improvements

As a helper/utility, we want to run migrations from our service, if we
provide an argument like `-migrations true` (by default this would be false).
We also want to use the connection retry for the migrations, as well as our
service which will run behind it.

Let's improve `cmd/db-migrate-cli/main.go` first:

~~~diff
-               db struct {
-                       DSN    string
-                       Driver string
-               }
+               db      db.ConnectionOptions
                real    bool
                service string
        }
-       flag.StringVar(&config.db.Driver, "db-driver", "mysql", "Database driver")
-       flag.StringVar(&config.db.DSN, "db-dsn", "", "DSN for database connection")
+       flag.StringVar(&config.db.Credentials.Driver, "db-driver", "mysql", "Database driver")
+       flag.StringVar(&config.db.Credentials.DSN, "db-dsn", "", "DSN for database connection")
~~~

We replace the already existing structure with DSN and Driver name with the excepted
db.ConnectionOptions. We can now pass this to a `ConnectWithRetry` method along with a context:

~~~go
	ctx := context.Background()

	switch config.real {
	case true:
		handle, err := db.ConnectWithRetry(ctx, config.db)
		if err != nil {
			log.Fatalf("Error connecting to database: %+v", err)
		}
		if err := db.Run(config.service, handle); err != nil {
			log.Fatalf("An error occured: %+v", err)
		}
~~~

There's no changes to the API apart from that the connection retry is now built in.

To make this easier to test, we provision a specific `migrations` user and database with
the definition for the database service in drone.yml:

~~~yaml
services:
- name: mysql-test
  pull: always
  image: percona/percona-server:8.0.17
  ports:
    - 3306
  environment:
    MYSQL_ROOT_PASSWORD: default
    MYSQL_USER: migrations
    MYSQL_PASSWORD: migrations
    MYSQL_DATABASE: migrations
~~~

After running the migrations for a given service, we need to empty this database.
We don't want to error out if two services decide to have the same table name like `log`
or something generic. Let's add a `-drop` option to the `db-schema-cli`. At the same
time we also add `service`, since currently we only have `schema` and our code generation
would produce `package migration`, which would be incorrect.

1. to the `config` variable, add `service string` and `drop bool`,
2. invoke `flag.StringVar` and `flag.BoolVar` respectively,
3. when checking `config.output == ""`, add ` && !config.drop`,
4. after listing all the tables in the database, add the following snippet:

~~~go
// Drop all tables in schema
if config.drop {
	for _, table := range tables {
		query := "DROP TABLE " + table.Name
		log.Println(query)
		if _, err := handle.Exec(query); err != nil {
			log.Fatal(err)
		}
	}
	return
}
~~~

And finally, update renderGo and renderMarkdown parameter config.schema to config.service:

~~~diff
-               if err := renderGo(config.output, config.schema, tables); err != nil {
+               if err := renderGo(config.output, config.service, tables); err != nil {
...
-               if err := renderMarkdown(config.output, config.schema, tables); err != nil {
+               if err := renderMarkdown(config.output, config.service, tables); err != nil {
~~~

After cleaning up our Makefile and drone.yml file, we have the following:

~~~Makefile
migrate.%: export SERVICE = $*
migrate.%: DSN = "migrations:migrations@tcp(mysql-test:3306)/migrations"
migrate.%:
	./build/db-migrate-cli-linux-amd64 -service $(SERVICE) -db-dsn $(DSN) -real=true
	./build/db-migrate-cli-linux-amd64 -service $(SERVICE) -db-dsn $(DSN) -real=true
	@find -name types_gen.go -delete
	./build/db-schema-cli-linux-amd64 -service $(SERVICE) -schema migrations -db-dsn $(DSN) -format go -output server/$(SERVICE)
	./build/db-schema-cli-linux-amd64 -service $(SERVICE) -schema migrations -db-dsn $(DSN) -format markdown -output docs/schema/$(SERVICE)
	./build/db-schema-cli-linux-amd64 -schema migrations -db-dsn $(DSN) -drop=true
~~~

And .drone.yml respectively:

~~~yaml
steps:
- name: codegen
  image: titpetric/microservice-build
  pull: always
  commands:
    - make rpc
    - make templates
    - make build-cli
    - make migrate

- name: build
  image: titpetric/microservice-build
  pull: always
  commands:
    - make tidy
    - wire ./...
    - make lint
    - make build
~~~

The changes have allowed us to remove the following hack in the drone.yml config:

~~~yaml
    - "bash -c 'while : ; do  sleep 1 ; $(cat < /dev/null > /dev/tcp/mysql-test/3306) && break ; done' 2>/dev/null"
~~~

In fact, we removed the complete drone `migration` step in the cleanup. Since the schema is
already provisioned with the database service, we eliminated the need to highjack the
percona image for the database client, or installing it in our build image.

All we need to do now is to verify everything works as expected by running `make`:

~~~plaintext
...
[codegen:27] + make migrate
[codegen:28] ./build/db-migrate-cli-linux-amd64 -service stats -db-dsn "migrations:migrations@tcp(mysql-test:3306)/migrations" -real=true
[codegen:29] 2019/12/15 13:04:00 connecting to database m****s:m****s@tcp(mysql-test:3306)/migrations
[codegen:30] 2019/12/15 13:04:00 can't connect, dsn=m****s:m****s@tcp(mysql-test:3306)/migrations, err=dial tcp 192.168.96.2:3306: connect: connection refused, try=1
[codegen:31] 2019/12/15 13:04:05 can't connect, dsn=m****s:m****s@tcp(mysql-test:3306)/migrations, err=dial tcp 192.168.96.2:3306: connect: connection refused, try=2
[codegen:32] 2019/12/15 13:04:10 can't connect, dsn=m****s:m****s@tcp(mysql-test:3306)/migrations, err=dial tcp 192.168.96.2:3306: connect: connection refused, try=3
[codegen:33] 2019/12/15 13:04:15 Running migrations from migrations.sql
...
~~~

And finally, we need to add an option to our service main.go files that will enable us
to run the migrations on service startup, simplifying our `docker-compose.yml`.
Let's also take care of context cancellation via signals, using the small
but effective [SentimensRG/sigctx](https://github.com/sentimensRG/sigctx) package:

~~~diff
--- a/templates/cmd_main.go.tpl
+++ b/templates/cmd_main.go.tpl
@@ -4,20 +4,41 @@ package main
 // generator and template: templates/cmd_main.go.tpl
 
 import (
+       "flag"
        "log"
-       "context"
 
        "net/http"
 
        _ "github.com/go-sql-driver/mysql"
+       "github.com/SentimensRG/sigctx"
 
+       "${MODULE}/db"
        "${MODULE}/internal"
        "${MODULE}/rpc/${SERVICE}"
        server "${MODULE}/server/${SERVICE}"
 )
 
 func main() {
-       ctx := context.TODO()
+       var config struct {
+               migrate bool
+               migrateDB db.ConnectionOptions
+       }
+       flag.StringVar(&config.migrateDB.Credentials.Driver, "migrate-db-driver", "mysql", "Migrations: Database driver")
+       flag.StringVar(&config.migrateDB.Credentials.DSN, "migrate-db-dsn", "", "Migrations: DSN for database connection")
+       flag.BoolVar(&config.migrate, "migrate", false, "Run migrations?")
+       flag.Parse()
+
+       ctx := sigctx.New()
+
+       if config.migrate {
+               handle, err := db.ConnectWithRetry(ctx, config.migrateDB)
+               if err != nil {
+                       log.Fatalf("Error connecting to database: %+v", err)
+               }
+               if err := db.Run("${SERVICE}", handle); err != nil {
+                       log.Fatalf("An error occured: %+v", err)
+               }
+       }
~~~

We also add a bit of output before we start listening for requests:

~~~go
	log.Println("Starting service on port :3000")
	http.ListenAndServe(":3000", internal.WrapWithIP(twirpHandler))
~~~

## Re-testing the bundled migrations

We can now simplify our `docker-compose.yml` config to run migrations directly from our
service. Automatic migrations aren't something you want to run in production, but it
simplifies our testing/development a lot, since the database contents are usually thrown away.

~~~yaml
version: '3.4'

services:
  stats:
    image: titpetric/service-stats
    restart: always
    environment:
      DB_DSN: "stats:stats@tcp(db:3306)/stats"
    command: [
      "-migrate-db-dsn=stats:stats@tcp(db:3306)/stats",
      "-migrate"
    ]

  db:
    image: percona/percona-server:8.0.17
    environment:
      MYSQL_ALLOW_EMPTY_PASSWORD: "true"
      MYSQL_USER: "stats"
      MYSQL_DATABASE: "stats"
      MYSQL_PASSWORD: "stats"
    restart: always
~~~

Don't forget to run `make && make docker`, and after you can run `docker-compose up` and
watch the service getting set up without manual migration steps.

