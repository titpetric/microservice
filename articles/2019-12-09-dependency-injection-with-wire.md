# Go: Dependency injection with Wire

Our microservices ultimately need at least a database connection in order to
query and store data in a MySQL database. We need to pass this object, possibly
from the `main()` function, to our service implementation.

## The Go way

I have learned through trial and error, that "Dependency Injection" is a term which
manages to evoke a strong emotional response in gophers. As I've said before, all
that the term represents in Go is "a way to pass stuff into our handlers", while it
means a lot of different things to people familiar with the concept from other
programming languages.

The most trivial way of dependency injection would be to declare function arguments
in our server constructor. For example, if we would need a `*sqlx.DB`, we would
declare a constructor like:

~~~go
func New(ctx context.Context, db *sqlx.DB) {
	...
}
~~~

It would be up to the caller to implement the correct argument list to fill the
required dependencies for the constructor. When dealing with a possible large scale
RPC scaffolding project, like we are, we are faced with a few pitfalls of this
approach.

The proto file definition for our service doesn't provide any way to set up a
list of dependencies which are required for our service. Obviously the service
block in the proto file only declared the interface for our RPCs, but it has
absolutely no standard way to reference Go structures (like `*sqlx.DB`).

The obvious second pitfall is that our code generators would become exponentially
more complex with each added service, and custom service dependencies. While we
can asume that we use a single database connection for a service, we can't really
know how unique the dependencies for any given service will be.

Ideally we need a way where we can add new dependencies, and not have to refactor
every service we have written until then to keep some idea of a consistent Go API.

## Runtime vs. compile time

In addition to the standard Go way to pass parameters to constructors, there are
two pattern to dependency injection in the wild. Each one of them has a set of
constraints, which basically boil down to it's effect on when an issue is detected.

Runtime dependency injections, like [codegangsta/inject](https://github.com/codegangsta/inject)
resort to using reflection from the internal `reflect` package, to inspect objects or
functions written in the Go way above, and invoke functions that provide results for
individual parameters. As an example, injection would look like:

~~~go
injector := inject.New()
injector.Map(context.TODO()) // literal type
injector.Map(func() *sqlx.DB { return new(sqlx.DB) }) // object factory
injector.Invoke(server.New)
~~~

The idea is, that you declare an object which holds either concrete values or functions
that return values of a required type and then figure out with reflection which values
should be passed to `server.New`, hidden behind injector.Invoke() call.

Obviously this has a few issues like error handling (it's likely that a database
connection can produce a timeout error on connection, we can't handle it here and thus
we can only resort to a panic call), and ultimately a runtime error when a value or
construction for a particular type is not defined.

Runtime errors are an unwanted side effect when reflection is used in such a way. The
structure of the application doesn't have any kind of type hinting that would suggest
to the go compiler that a dependency isn't satisfied, and possibly dangerous errors
could bleed into production because of this.

Google has also realized this problem, and because of it has designed their own DI
framework, [google/wire](https://github.com/google/wire). The intent of the project
is to provide a compile-time dependency injection, which could error out in case
a dependency isn't declared or provided at build time.

## Wire and dependency providers

Wire relies on code generation and reflection to analyze the source code and produce
injection code that is equal to what you would have to write, in order to fill the
dependencies required by your service.

The concepts introduced by Wire are called:

- The Injector - the code that introduces dependencies to your object,
- The Provider - the code that provides an instance of a particular type

An injector is any function that returns a particular type, like in our case,
the database connection handle, `*sqlx.DB`, or in the case of our currently
only RPC client, a `stats.StatsService` interface. Each of these can be returned
with an error, or without one. If returned with an error, wire will generate
the supporting code for error checking and bubble up the error from the generated
Injector.

Let's create `db/connect.go` with our provider for a database connection:

~~~go
package db

import (
	"errors"
	"os"

	"github.com/jmoiron/sqlx"
)

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
~~~

In this case, the `db.Connect` function is a provider for our database connection.
As for our clients, we will resort to code generation again, to produce another
ProviderSet. Add the following to your Makefile under your `templates:` target (the
main one):

~~~Makefile
	@./templates/client_wire.go.sh
~~~

And then create `templates/client_wire.go.sh` with the following contents:

~~~bash
#!/bin/bash
cd $(dirname $(dirname $(readlink -f $0)))

## list all services
schemas=$(ls -d rpc/* | xargs -n1 basename)

function render_wire {
	echo "package client"
	echo
	echo "import ("
	echo -e "\t\"github.com/google/wire\""
	echo
	for schema in $schemas; do
		echo -e "\t\"${MODULE}/client/${schema}\""
	done
	echo ")"
	echo
	echo "var Inject = wire.NewSet("
	for schema in $schemas; do
		echo -e "\t${schema}.New,"
	done
	echo ")"
}

render_wire > client/wire.go
echo "~ client/wire.go"
~~~

For each service we have declared, a new provider is registered in wire.NewSet() and
made available over `client.Inject`. In our current case, this file is generated:

~~~go
package client

import (
	"github.com/google/wire"

	"github.com/titpetric/microservice/client/stats"
)

var Inject = wire.NewSet(
	stats.New,
)
~~~

As we would add a new service, the import for the service would be added, and a new
provider would be added to the `wire.NewSet` call. As such, new services immediately
become available to all existing services, all we need is to add the appropriate type
into the Server{} structure and it will get filled with the generated injector.

As these injectors are stand-alone, we can create a global `inject` package, which
binds them together in a single ProviderSet (`inject/inject.go`):

~~~go
package inject

import (
	"github.com/google/wire"

	"github.com/titpetric/microservice/client"
	"github.com/titpetric/microservice/db"
)

var Inject = wire.NewSet(db.Connect, client.Inject)
~~~

And from here on we can move towards generating the Injector with wire.

## Wire and dependency Injector

We have scaffolded all the requirements for wire, to be able to analyze the code and
generate the injector that will fill out dependencies for our services. First, we
need to install wire in our build/Dockerfile:

~~~diff
+# Install google wire for DI
+RUN go get -u github.com/google/wire/cmd/wire
~~~

Rebuild and push the dockerfile to your registry, and you can start using wire from Drone.
As wire will create our new service constructor, remove `New()` from `templates/server_server.go.tpl`
and add `*sqlx.DB` as a dependency by declaring it inside the `Server{}` struct:

~~~
package ${SERVICE}

import (
	"context"

	"github.com/jmoiron/sqlx"

	"${MODULE}/rpc/${SERVICE}"
)

type Server struct {
	db *sqlx.DB
}

var _ ${SERVICE}.${SERVICE_CAMEL}Service = &Server{}
~~~

This will ensure that there isn't a conflict between the old constructor, and the new
constructor created with Wire. We will now create an injector definition in our service,
under `server/stats/wire.go`, by using a template for our code generator (`templates/server_wire.go.tpl`):

~~~
//+build wireinject

package ${SERVICE}

import (
	"context"

	"github.com/google/wire"

	"${MODULE}/inject"
)

func New(ctx context.Context) (*Server, error) {
	wire.Build(
		inject.Inject,
		wire.Struct(new(Server), "*"),
	)
	return nil, nil
}
~~~

As you see from the template, the file begins with `//+build wireinject`, which
excludes this file from our build process. Wire uses this file to load and analyze
the source files that it references, to figure out which dependencies are defined.

Make sure that you invoke the following line in your Makefile `templates.%` target
to generate the required wire.go file for each existing service:

~~~Makefile
	@envsubst < templates/server_wire.go.tpl > server/$(SERVICE)/wire.go
~~~

After we build the docker image for our build environment, all we need to do is
run `wire ./...` on our code base. We will add this to our build steps in drone:

~~~yaml
- name: build
  image: titpetric/microservice-build
  pull: always
  commands:
    - make tidy
    - wire ./...
    - make build
~~~

Running `wire ./...` produces our `wire_gen.go` files:

~~~
# wire ./...
wire: github.com/titpetric/microservice/server/stats: wrote server/stats/wire_gen.go
~~~

And if we inspect the file itself, we can see it's written just as well if we would
write it by hand. Each of the required dependencies by our server is filled out, and
the others are omitted. This in our case means that we get our DB handle, but as
we aren't using any RPC clients they are omitted until we add them to the `Server` struct.

~~~go
// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package stats

import (
	"context"
	"github.com/titpetric/microservice/db"
)

// Injectors from wire.go:

func New(ctx context.Context) (*Server, error) {
	sqlxDB, err := db.Connect()
	if err != nil {
		return nil, err
	}
	server := &Server{
		db: sqlxDB,
	}
	return server, nil
}
~~~

The providers that return errors also have error handling in this function. As the errors
are preserved and returned, we can use them outside of our service constructor - in our case,
we issue a `log.Fatal` in main() and exit.

With wire set up, adding a new dependency into our service is trivial, we just need to add
a new field into the Server{} struct. In case we want to omit a field from being filled out
with wire, we can tag it with `wire:"-"`. We future proofed our service layer in such a way
that we can add new dependencies into the main `inject` package, and they will be picked up
by all services at the same time.

There are some caveats with wire, notably that you can't pass a `wire.ProviderSet` as the
argument for an Injector. This is because wire relies on parsing the AST of your packages
source files in order to generate provider invocations. In the sense of a public API which
could be imported from a third party package, wire has no way to know on the code generation
step, which providers are defined inside a ProviderSet.

There are probablly other notable exceptions where wire might fail. In the sense of something
you can pick up and randomly throw at an object, it's definitely not as robust as writing
individual server constructors by hand.

As a possibility, instead of a generic ProviderSet, we could have a concrete Providers interface,
that must satisfy all the fields declared on `Server{}`. The interface, as it is a concrete type,
can cross package boundaries and the individual provider implementations (or a singleton of them)
could be provided as an external package. Currently wire doesn't really take the concept into
it's logical conclusion, and is probablly too complex for a DI framework. But, as we don't really
have a better option and won't start to write our own, this is what we have and we can work with it.
