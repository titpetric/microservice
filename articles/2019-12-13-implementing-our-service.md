# Go: implementing a microservice

We should move towards writing actual code for our service. We are
writing a simple statistics collection service for tracking page views.
We are going to create a database schema for collecting page views
and improve things along the way.

## The database schema

To refresh, we will be implementing our Push() RPC call:

~~~protobuf
service StatsService {
	rpc Push(PushRequest) returns (PushResponse);
}

message PushRequest {
	string property = 1;
	uint32 section = 2;
	uint32 id = 3;
}

message PushResponse {}
~~~

I've created a database schema which serves my use case for logging
the request. In addition to the request data for the RPC, I added fields
`remote_ip`, and `stamp` to track the user making the page view and
the time the page view was logged. I also defined an `id` uint64 primary
key, which I'll fill out with a K-sortable [Sonyflake ID](https://github.com/sony/sonyflake).

| Name             | Type                | Key | Comment                             |
|------------------|---------------------|-----|-------------------------------------|
| id               | bigint(20) unsigned | PRI | Tracking ID                         |
| property         | varchar(32)         |     | Property name (human readable, a-z) |
| property_section | int(11) unsigned    |     | Property Section ID                 |
| property_id      | int(11) unsigned    |     | Property Item ID                    |
| remote_ip        | varchar(255)        |     | Remote IP from user making request  |
| stamp            | datetime            |     | Timestamp of request                |

As you can see, our docs generator is already paying for itself. This
is our migration, with the comments stripped out to cut down verbosity:

~~~sql
CREATE TABLE `incoming` (
 `id` bigint(20) unsigned
 `property` varchar(32)
 `property_section` int(11) unsigned
 `property_id` int(11) unsigned
 `remote_ip` varchar(255)
 `stamp` datetime
 PRIMARY KEY (`id`)
);

CREATE TABLE `incoming_proc` LIKE `incoming`;
~~~

We are automatically creating a copy of `incoming` into `incoming_proc`. This has to do
with processing caveats - renaming tables in mysql is an atomic operation, meaning we
can issue the following queries and replace the tables between each other:

~~~sql
RENAME TABLE incoming TO incoming_old, incoming_proc TO incoming, incoming_old TO incoming_proc;
~~~

The rename table statement would replace the tables between each other. This is to separate
our reads from our writes - one table is always getting written to, the other is getting read
and aggregated into other tables, grouped by various time intervals (hourly, daily, monthly,...).

In the previous version of this service, the table schema had an ID field which was an AUTO_INCREMENT.
In InnoDB, an AUTO_INCREMENT column triggers a special table-level lock, which blocks other transactions
so that they recieve consecutive primary key values. We aren't resorting to auto incrementing fields
so we already eliminated a possible source of performance issues.

After adding the migration, we run `make`, so all our assets are generated.

## Improving the microservice environment

We do a few improvements of our code generators in order to increase consistency. Since
wire generates `wire_gen.go`, we rename our `types_db.go` to `types_gen.go` so we can
be consistent. We also notice that the fields from our migration result in a few oddly
named struct fields:

- Id
- PropertyId
- RemoteIp

When we were writing our camelizer, we didn't consider that we want to stay compliant
with common Go naming styles, e.g. "JSON" instead of "Json". Let's quickly improve that
part of our code generator, by importing a package that handles common initialisms.

~~~go
package main

import (
	"github.com/serenize/snaker"
)

func camel(input string) string {
	return snaker.SnakeToCamel(input)
}
~~~

And now we can use ID, PropertyID and RemoteIP respectively. The package
[serenize/snaker](https://github.com/serenize/snaker) lists common initialisms
which it sources from [golang/lint#lint.go](https://github.com/golang/lint/blob/fdd1cda4f05fd1fd86124f0ef9ce31a0b72c8448/lint.go#L770).
We can even add golint to our CI jobs? Let's do it. Add the following at the end of `docker/build/Dockerfile`:

~~~Dockerfile
# Install golint
RUN go get -u golang.org/x/lint/golint
~~~

Rebuild the image by running `make` and `make push` so it can be used with Drone, and then
let's add a Makefile target and Drone CI step to lint our code:

~~~Makefile
lint:
	golint -set_exit_code ./...
~~~

And our `.drone.yml`:

~~~diff
--- a/.drone.yml
+++ b/.drone.yml
@@ -29,6 +29,7 @@ steps:
   commands:
     - make tidy
     - wire ./...
+    - make lint
     - make build
~~~

I fixed the issues that showed up, mostly about having comments for exported fields, like:

~~~diff
--- a/templates/client_wire.go.sh
+++ b/templates/client_wire.go.sh
@@ -15,6 +15,7 @@ function render_wire {
        done
        echo ")"
        echo
+       echo "// Inject produces a wire.ProviderSet with our RPC clients"
        echo "var Inject = wire.NewSet("
        for schema in $schemas; do
                echo -e "\t${schema}.New,"
~~~

And after adding all the possible comments to our code generated code and a few stylistic
fixes around our `db` package, we are ready to implement our service up to spec.

## Implementing Push

Implementing our Push RPC with all the scaffolding which we have now is pretty trivial:

~~~go
package stats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/titpetric/microservice/rpc/stats"
)

// Push a record to the incoming log table
func (svc *Server) Push(ctx context.Context, r *stats.PushRequest) (*stats.PushResponse, error) {
	var err error
	row := Incoming{}

	row.ID, err = svc.sonyflake.NextID()
	if err != nil {
		return nil, err
	}

	row.Property = r.Property
	row.PropertySection = r.Section
	row.PropertyID = r.Id
	row.RemoteIP = "127.0.0.1"
	row.SetStamp(time.Now())

	fields := strings.Join(IncomingFields, ",")
	named := ":" + strings.Join(IncomingFields, ",:")

	query := fmt.Sprintf("insert into %s (%s) values (%s)", IncomingTable, fields, named)
	_, err = svc.db.NamedExecContext(ctx, query, row)
	return nil, err
}
~~~

This is an initial implementation. There are two notable things to take care of. Since the Stamp field
was mapped into a `*time.Time`, we can't assign the output of `time.New() time.Time` directly to it.
Since I don't particularly like wrapping a `timeNewPtr() *time.Time` in every service, I modified the
code generator to add a setter for this field type:

~~~go
...
	for _, table := range tables {
		fields := []string{}
		primary := []string{}
		setters := []string{}
...
			if columnType == "*time.Time" {
				setters = append(setters, []string{
					fmt.Sprintf("// Set%s sets %s which requires a *time.Time", columnName, columnName),
					fmt.Sprintf("func (s *%s) Set%s(t time.Time) { s.%s = &t }", tableName, columnName, columnName),
				}...)
			}
...
		for _, v := range setters {
			fmt.Fprintln(buf, v)
		}
		if len(setters) > 0 {
			fmt.Fprintln(buf)
		}

...
~~~

And the other thing is still a TODO item - I want to log the Remote IP of the request.
Now, since we're dealing with a Twirp implementation, we know that we have a `*http.Request`
as the entrypoint. This is the first real divergence between the implementation of a Twirp
or a gRPC service.

What we need to do to get the IP here is to wrap our twirp handler in our own http.Handler,
which gets the information about the IP from the `*http.Request`, and updates the context so
we can get the value for this field from the context passed to our handler.

1. http.Request provides a `Context() context.Context` function to get the request,
2. http.Request provides a `WithContext(context.Context) *Request` to build a new *Request

This should be pretty simple. Let's see if we can do it on the first try (no cheating!).
To help you out with some starting code, this is a wrapped handler:

~~~go
--- a/templates/cmd_main.go.tpl
+++ b/templates/cmd_main.go.tpl
@@ -15,6 +15,12 @@ import (
        server "${MODULE}/server/${SERVICE}"
 )
 
+func wrapWithIP(h http.Handler) http.Handler {
+       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
+               h.ServeHTTP(w, r)
+       })
+}
+
 func main() {
        ctx := context.TODO()
 
@@ -25,5 +31,5 @@ func main() {
 
        twirpHandler := ${SERVICE}.New${SERVICE_CAMEL}ServiceServer(srv, nil)
 
-       http.ListenAndServe(":3000", twirpHandler)
+       http.ListenAndServe(":3000", wrapWithIP(twirpHandler))
 }
~~~

Now stop reading and go write your wrapper!

I said no cheating? Read the IP from a HTTP Request{}, set it into a context,
and then come back when you have it working!

Ok, enough of that. Let's see what I did on the first try:

~~~
func wrapWithIP(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get IP address
		ip := func() string {
			headers := []string{
				http.CanonicalHeaderKey("X-Forwarded-For"),
				http.CanonicalHeaderKey("X-Real-IP"),
			}
			for _, header := range headers {
				if addr := r.Header.Get(header); addr != "" {
					return strings.SplitN(addr, ", ", 2)[0]
				}
			}
			return strings.SplitN(r.RemoteAddr, ":", 2)[0];
		}()

		ctx := r.Context()
		ctx = context.WithValue(ctx, "ip.address", ip)

		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
~~~

I implemented a wrapper that checks `X-Forwarded-For` and `X-Real-IP` headers, and returns
the first relevant IP listed in there. In case none of these headers are present, I take
`r.RemoteAddr`, and strip away the port number which is expected from reading the docs.

Let's modify our Push implementation to read from the context:

~~~go
	if remoteIP, ok := ctx.Value("ip.address").(string); ok {
		row.RemoteIP = remoteIP
	}
~~~

So, at worst - the Remote IP field will be empty. Hopefully I didn't do too bad of a job
above and I'm gonna see a real IP get logged into the database when I'll issue my first
request to the service.