# Go: Listing database migrations

As we prepared the database migration files and embedded them into the db package,
we are now left to implement the details required to process these migrations. Let's
start with extending the FS type, to actually provide functionality for reading them.

~~~go
// ReadFile returns decoded file contents from FS
func (fs FS) ReadFile(filename string) ([]byte, error) {
	if val, ok := fs[filename]; ok {
		return base64.StdEncoding.DecodeString(val)
	}
	return nil, os.ErrNotExist
}
~~~

Reading a file from our FS type is super simple. All we need to do is check that the file
we are trying to read actually exists, and base64 decode it. In order to be somewhat compliant
to the standard practice for reading files, we return an `os.ErrNotExist` error in case
the file we're trying to read isn't there.

Let's also add a migrations helper, that will list the migrations inside the filesystem,
so that we can just loop over them and run them one by one. It only lists actual migration
files (`*.up.sql`), and doesn't return any other files embedded (`migrations.sql` for one).
The files are sorted into their expected execution order.

~~~go
// Migrations returns list of SQL files to execute
func (fs FS) Migrations() []string {
	result := []string{}
	for filename, contents := range fs {
		// skip empty files
		if contents == "" {
			continue
		}
		if matched, _ := filepath.Match("*.up.sql", filename); matched {
			result = append(result, filename)
		}
	}
	sort.Strings(result)
	return result
}
~~~

We also need a structure for our migration schema (`migration.sql`), and this is it:

~~~go
package db

type (
	migration struct {
		Project        string `db:"project"`
		Filename       string `db:"filename"`
		StatementIndex int    `db:"statement_index"`
		Status         string `db:"status"`
	}
)

func (migration) Fields() []string {
	return []string{"project", "filename", "statement_index", "status"}
}
~~~

The fields are as follows:

- `project` denotes the migration group / service (e.g. `stats`),
- `filename` is a migration from that group (`2019-11-26-092610-description-here.up.sql`),
- `statement_index` - the sequential statement index from a migration,
- `status` - this will either be "ok", or the error produced from a failing migration

To exmplain `statement_index`: each migration can be several SQL queries. A common case is
to import the complete database schema inside one migration, where a migration will be several
`CREATE TABLE` statements in a single file. We need to split this file with a delimiter `;`,
which should be at the end of the line. For this we can use the `;$` regular expression to
split the migration into statements, remove empty statements, and return a list of them.
The list key is the statement index.

The Fields() function there is to help us issue queries against the database. In order
to issue a full `INSERT` (or `REPLACE`,...), we need to list the database table fields.
The other way to do it would be to use reflection, but that's overkill. This function
helps us to remove some of the field stutter which applies to data insertion/update.

We can produce a helper function, that takes the result of `FS.ReadFile`:

~~~go
func statements(contents []byte, err error) ([]string, error) {
	result := []string{}
	if err != nil {
		return result, err
	}
	stmts := regexp.MustCompilePOSIX(";$").Split(string(contents), -1)
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			result = append(result, stmt)
		}
	}
	return result, nil
}
~~~

This way, we can produce the statements from a migration file in the following way:

~~~go
stmts, err := statements(migrations["stats"].ReadFile("migrations.sql"))
~~~

The result from ReadFile is expanded into the function parameters here. The error
checking here is a bit reduced, on account that the error is just propagated forward.

## The migration API

So now that we have all the required structures and functions to read migrations and
split them into statements, we need to consider the migration API. I would like to
provide the following functionality from this package:

- `List() []string` - should list the service names for the embedded migrations
- `Print(project string) error` - output the migration queries with log.Println,
- `Run(project string, db *sqlx.DB) error` - pass a database handle and project name to run migrations

This gives us the ability to write a cli tool, that can print or run specific migrations.
Let's start with the low hanging fruit: List():

~~~go
func List() []string {
	result := []string{}
	for k, _ := range migrations {
		result = append(result, k)
	}
	return result
}
~~~

The function will run only once, and isn't performance intensive, but for a small exercise,
let's rewrite that into an efficiently allocated function. I do this so often that it becomes
second nature, just to avoid inefficient allocations with append().

~~~go
func List() []string {
	result := make([]string, len(migrations))
	i := 0
	for k, _ := range migrations {
		result[i] = k
	}
	return result
}
~~~

Which leaves us with Print and Run functions. Let's start with Print, since it doesn't require us
to set up a database just yet. The goal is to get the project filesystem and list the statements
for the contained migrations. Let's first implement `cmd/db-migrate-cli/main.go`:

~~~go
package main

import (
	"log"

	"github.com/titpetric/microservice/db"
)

func main() {
	log.Printf("Migration projects: %+v", db.List())
	log.Println("Migration statements for stats")
	if err := db.Print("stats"); err != nil {
		log.Printf("An error occured: %+v", err)
	}
}
~~~

We will extend this file over our development, but for now we are only interested in verifying
that the API which we will create is functional. We could make unit tests, but this is just as
good as we need this tool in any case. We cannot verify validity of the data just yet, the only
thing we can do is to verify that the data is there, and can be read.

~~~go
package db

import (
	"fmt"
	"log"

	"github.com/pkg/errors"
)

func Print(project string) error {
	fs, ok := migrations[project]
	if !ok {
		return errors.Errorf("Migrations for '%s' don't exist", project)
	}

	printQuery := func(idx int, query string) error {
		log.Println()
		log.Println("-- Statement index:", idx)
		log.Println(query)
		log.Println()
		return nil
	}

	migrate := func(filename string) error {
		log.Println("Printing migrations from", filename)
		if stmts, err := statements(fs.ReadFile(filename)); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Error reading migration: %s", filename))
		} else {
			for idx, stmt := range stmts {
				if err := printQuery(idx, stmt); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// print main migration
	if err := migrate("migrations.sql"); err != nil {
		return err
	}

	// print service migrations
	for _, filename := range fs.Migrations() {
		if err := migrate(filename); err != nil {
			return err
		}
	}
	return nil
}
~~~

The structure here is a bit shorter than what we expect from `Run`, as we don't need to log
the statement indices in the migration struct. We print the initial migrations.sql, as well
as every other migration contained in the migration index for the project.

After running `make` and building all our binaries, we can run the `db-migrate-cli` cli to
verify that the migration statements are being printed.

~~~
# ./build/db-migrate-cli-linux-amd64
2019/11/26 11:20:00 Migration projects: [stats]
2019/11/26 11:20:00 Migration statements for stats
2019/11/26 11:20:00 Printing migrations from migrations.sql
2019/11/26 11:20:00
2019/11/26 11:20:00 -- Statement index: 0
2019/11/26 11:20:00 CREATE TABLE IF NOT EXISTS `migrations` (
 `project` varchar(16) NOT NULL COMMENT 'Microservice or project name',
 `filename` varchar(255) NOT NULL COMMENT 'yyyy-mm-dd-HHMMSS.sql',
 `statement_index` int(11) NOT NULL COMMENT 'Statement number from SQL file',
 `status` text NOT NULL COMMENT 'ok or full error message',
 PRIMARY KEY (`project`,`filename`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8
2019/11/26 11:20:00
2019/11/26 11:20:00 Printing migrations from 2019-11-26-092610-description-here.up.sql
2019/11/26 11:20:00
2019/11/26 11:20:00 -- Statement index: 0
2019/11/26 11:20:00 -- Hello world
2019/11/26 11:20:00
~~~

We now verified that:

- we can list our db schema migration projects (`stats`),
- we have our `migration.sql` file embedded,
- we have some migrations contained, notably `-- Hello world` :)

We can now move onto implementing execution of the migration against a real database.
