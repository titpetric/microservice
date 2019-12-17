# Go: Instrumenting the Database client with Elastic APM

After we set up Elastic APM to log our request transaction, the following thing we wish to
instrument are the SQL queries going to our database endpoint. Elastic APM provides
instrumentation that wraps the database/sql driver, which produces an `*sql.DB`.

## Extending DB connection

We already planned to produce a `*sqlx.DB` for this eventuality with the
Connector field function in `db.ConnectionOptions`:

~~~go
// Connector is an optional parameter to produce our
// own *sql.DB, which is then wrapped in *sqlx.DB
Connector func(context.Context, Credentials) (*sql.DB, error)
~~~

We can now just modify the `Connect()` function in the DB package, to extend it with
an APM connector.

~~~go
// Connect connects to a database and produces the handle for injection
func Connect(ctx context.Context) (*sqlx.DB, error) {
	options := ConnectionOptions{
		Connector: func(ctx context.Context, credentials Credentials) (*sql.DB, error) {
			db, err := apmsql.Open(credentials.Driver, credentials.DSN)
			if err != nil {
				return nil, err
			}
			if err = db.PingContext(ctx); err != nil {
				db.Close()
				return nil, err
			}
			return db, nil
		},
	}
	options.Credentials.DSN = os.Getenv("DB_DSN")
	options.Credentials.Driver = os.Getenv("DB_DRIVER")
	return ConnectWithRetry(ctx, options)
}
~~~

There are about three notable parts to this change. First, by default we were using
`sqlx.Connect` to create the database handle and issue a Ping request and error out.
As this is an sqlx addon, we need to re-implement some functionality here by calling
Open, followed with PingContext.

What `apmsql` does under the scenes is to produce a `sql.Driver` interface, that wraps the
original driver for the drivers you're already familiar with. The Elastic APM Go Agent
provides the following packages to register popular SQL drivers:

- `go.elastic.co/apm/module/apmsql/pq` (github.com/lib/pq)
- `go.elastic.co/apm/module/apmsql/mysql` (github.com/go-sql-driver/mysql)
- `go.elastic.co/apm/module/apmsql/sqlite3` (github.com/mattn/go-sqlite3)

## Verifying it's working

Each SQL query issued will produce what is called a "Span". Like the transactions, the APM
client sends the query metadata and duration, and nests it under the main request transaction.
This way you can see particularly which queries have executed within a given request.

Let's rebuild our service with `make` and `make docker`, and run our development stack
with `docker-compose up -d`. Let's re-run some requests with curl, and navigate to the new
transactions in the APM interface.



Everything stays the same, now there is just the APM agent under the hood, sending query
metrics to Elastic APM as we wanted.