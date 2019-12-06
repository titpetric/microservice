# Go: Instrumenting the Database client with Elastic APM

After we set up our request transaction, the following thing we wish to instrument are
the SQL queries going to our database endpoint. Elastic APM provides instrumentation that wraps
the database/sql driver, which produces an `*sql.DB`.

Each SQL query issued will produce what is called a "Span". Like the transactions, the APM
client sends the query metadata and duration, and nests it under the main request transaction.
This way you can see particularly which queries have executed within a given request.

The Elastic APM Go Agent provides the following packages to register popular SQL drivers:

- `module/apmsql/pq` (github.com/lib/pq)
- `module/apmsql/mysql` (github.com/go-sql-driver/mysql)
- `module/apmsql/sqlite3` (github.com/mattn/go-sqlite3)

Setting it up for our use case, we only need to produce the `*sql.DB` and wrap it into
an `*sqlx.DB` so we can use the sqlx query extensions like named parameter support.

## The connector

We already have a dependency defined in `db/connect.go`, which produces the required database
object. We will remove the already included driver for the database from `main.go`, and include
the augmented one from APM in the connect.go dependency. First we need to check the sqlx
documentation to figure out how we can wrap the *sql.DB object.

~~~go
// NewDb returns a new sqlx DB wrapper for a pre-existing *sql.DB.  The
// driverName of the original database is required for named query support.
func NewDb(db *sql.DB, driverName string) *DB {
	return &DB{DB: db, driverName: driverName, Mapper: mapper()}
}
~~~

Ok, that's pretty simple! All we need to keep is the connection object and the driver name,
and we have both of those readily available. Let's fix the Connect function to wrap it.

~~~go
-func Connect() (*sqlx.DB, error) {
+func Connect(ctx context.Context) (*sqlx.DB, error) {
        dsn := os.Getenv("DB_DSN")
        driver := os.Getenv("DB_DRIVER")
        if dsn == "" {
@@ -16,5 +19,17 @@ func Connect() (*sqlx.DB, error) {
        if driver == "" {
                driver = "mysql"
        }
-       return sqlx.Connect(driver, dsn)
+
+       db, err := apmsql.Open(driver, dsn)
+       if err != nil {
+               return nil, err
+       }
+
+       err = db.PingContext(ctx)
+       if err != nil {
+               db.Close()
+               return nil, err
+       }
+
+       return sqlx.NewDb(db, driver), nil
 }
~~~

There are about three notable parts to this change. First, by default we were using
`sqlx.Connect` to create the database handle and issue a Ping request and error out.
As this is an sqlx addon, we need to re-implement some functionality here by calling
Open, followed with Ping.

As Ping has a Context aware implementation, we changed the function signature to
receive a context to work with. Looking at the `wire_gen.go` files, we can see that
wire has re-wired our servers constructors to pass context to db.Connect:

~~~diff
 func New(ctx context.Context) (*Server, error) {
-       sqlxDB, err := db.Connect()
+       sqlxDB, err := db.Connect(ctx)
        if err != nil {
~~~

And ultimately, we just wrap the connection with sqlx.NewDb, producing an instrumented
database connection handle to use in our service. We didn't need to change anything
in our service, as wire took care of the only breaking change resulting from the change
of the connect function signature. Everything stays the same, now there is just the APM
agent under the hood, sending metrics to elastic APM as required.