# Bash: Embedding files into Go

There's always that chance that your Go app/service will need to bundle some
files in order to fulfill some requirement. A common case of that is bundling
SQL migrations into an app. SQL migrations are a set of SQL quereies that need
to run to set up a clean database, or to change database schema over the course
of the applications lifetime.

## Planning database migrations

Database migrations can be a tricky thing. With common systems, people tend to
write database migrations that go both ways - up, or down (undo). Since there's
always a possibility of data loss, we will only plan migrations that go one way.

Let's implement the following filesystem schema:

~~~plaintext
/db - the main database migration package (generated .go files too),
/db/schema/ - a collection of service migrations
/db/schema/stats/ - migrations for `stats` schema, *.up.sql files
/db/schema/.../ - other service migrations...
~~~

The individual migrations should be consistently named, for example, a
database migration might be stored in the file `2019-11-18-141536-create-message.up.sql`.
Particularly, we only consider `*.up.sql` to be a migration, and the prefix
with the full date and time serves as a sorting key, so we know in which
order the migrations should execute in the database.

In addition to the actual migrations, we need to track the migrations as
they have been applied. A migration that was already applied shouldn't run again
as it will produce errors, or worse. For that we need a migration table, which
should be part of every service schema.

~~~sql
CREATE TABLE IF NOT EXISTS `migrations` (
 `project` varchar(16) NOT NULL COMMENT 'Microservice or project name',
 `filename` varchar(255) NOT NULL COMMENT 'yyyy-mm-dd-HHMMSS.sql',
 `statement_index` int(11) NOT NULL COMMENT 'Statement number from SQL file',
 `status` text NOT NULL COMMENT 'ok or full error message',
 PRIMARY KEY (`project`,`filename`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
~~~

We want to embed these files into our app, and at the same time, keep them
grouped by service, so only individual migrations for a service can be executed.
For this purpose, we create a filesystem wrapper in `db/fs.go`:

~~~go
type FS map[string]string
~~~

Well, that was anti-climactic. If you expected some over-engineering here, it really
isn't the point - we're going for simplicity here. The filesystem takes the filename
as the key, and the file contents as the value. It's very easy to generate this filesystem,
as effectively each filesystem is only a few lines of code:

~~~go
package db;

var Stats FS = FS{
	"2019-11-18-141536-create-message.up.sql": "...",
	"...": "...",
}
~~~

In fact, the filesystems are so simple, that we can again resort to simple code generation
to implement them. We don't need any kind of compression, and the only issue we need to
solve, is properly quoting or escaping the file contents, so we can know that the generated
code won't produce errors.

## Poor mans file embedding

Ah, as we analyze how we're going to generate the filesystem assets, we also realize that
the poor mans templating from the previous section isn't really appropriate anymore. We need
to have a way to loop over the list of migration files, in order to generate individual filesystems.

We still might not want the complexity of a Go application to embed these files, so let's
resort to using bash to generate the files. The requirements are simple - for each migration
for a schema generate a filesystem, and generate an index of `map[string]FS`, with a key/value
for each service.

Since the files contain special characters like newslines and back-ticks and aren't nicely
embeddable in Go as is, we will resort to base64 encoding for the file contents. For that
we can use the shell `base64` command, which supports encoding and decoding.

~~~bash
#!/bin/bash
cd $(dirname $(dirname $(readlink -f $0)))

## encode file contents in base64
function base64_encode {
	cat $1 | base64 -w 0
}

## generate a service FS
function render_service_schema {
	local schema=$(basename $1)
	echo "package db;"
	echo
	echo "var $schema FS = FS{"
	local files=$(find $1 -name '*.sql' | sort)
	for file in $files; do
		echo "\"$(basename $file)\": \"$(base64_encode $file)\","
	done
	echo "}"
}

## list all service FS into `migrations` global
function render_schema {
	echo "package db;"
	echo
	echo "var migrations map[string]FS = map[string]FS{"
	for schema in $schemas; do
		local package=$(basename $schema)
		echo "\"${package}\": ${package},"
	done
	echo "}"
}

## list all service migrations (db/schema/stats, ...)
schemas=$(ls db/schema/*/migrations.sql | xargs -n1 dirname)
for schema in $schemas; do
	# db/schema/stats -> schema/stats
	schema_relative=${schema/db\//}
	# schema/stats -> db/schema_stats.go
	output="db/${schema_relative/\//_}.go"

	render_service_schema $schema > $output
done

render_schema > db/schema.go
~~~

All that's left to do here is just to run `go fmt` on the resulting go files. As that is
already handled in our Drone CI steps, we have now succesfully prepared the required SQL
migrations into the `db` package so we can use it from here.

As we have now embedded all the database migrations into go code, we can move on towards
running these migrations on a real database as part of our CI testing suite.

A> It's worth noting that just days before publishing this chapter, a proposal for embedding landed on golang/go,
A> by [@bradfitz](https://twitter.com/bradfitz). It seems if all goes well with the planning
A> here, and the proposal isn't rejected outright for usability concerns, that some time
A> in the future the Go toolchain might handle embedding files in a portable and secure way. Take a read here:
A> [proposal: cmd/go: support embedding static assets (files) in binaries #35950](https://github.com/golang/go/issues/35950)