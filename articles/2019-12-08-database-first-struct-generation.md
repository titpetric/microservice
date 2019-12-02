# Go: Database first struct generation

As we set up our protobuf structures and data migrations, we will need to interface between them.
Unfortunately, protobuf structs don't have good support for [adding go tags](https://github.com/golang/protobuf/issues/52),
so we can't easily pin `db` tags to it, nor should we really. There's always going to be some mismatching
with what a RPC request/response is, and what's actually stored in the database.

SQL databases have pretty good support over their `information_schema` interface. We can list the
database tables in a schema, and individually get their fields, field types and even comments so
we have all the information about structures available. Since we're using Drone CI to test migrations,
we can use the same database state to source our Go structures.

## Database schema data structures and helpers

We care both about the tables themselves, as well as the column names, types, and any column comment
that we might include in the DB schema. We can get this informations from `information_schema`, from
the `tables` and `columns` tables respectively.

Create a `cmd/db-schema-cli/types.go` file:

~~~go
package main

type Table struct {
	Name    string `db:"TABLE_NAME"`
	Comment string `db:"TABLE_COMMENT"`

	Columns []*Column
}

func (*Table) Fields() []string {
	return []string{"TABLE_NAME", "TABLE_COMMENT"}
}

type Column struct {
	Name    string `db:"COLUMN_NAME"`
	Type    string `db:"COLUMN_TYPE"`
	Key     string `db:"COLUMN_KEY"`
	Comment string `db:"COLUMN_COMMENT"`

	// Holds the clean data type (`int` not `int(11) unsigned` ...)
	DataType string `db:"DATA_TYPE"`
}

func (*Column) Fields() []string {
	return []string{"COLUMN_NAME", "COLUMN_TYPE", "COLUMN_KEY", "COLUMN_COMMENT", "DATA_TYPE"}
}
~~~

And we can already anticipate that we will need a function to convert the SQL field names we
declare into camel case. We can do it without regular expressions, like this:

~~~go
func camel(input string) string {
	parts := strings.Split(input, "_", -1)
	for k, v := range parts {
		parts[k] = strings.ToUpper(v[0:1]) + v[1:]
	}
	return strings.Join(parts, "")
}
~~~

## Querying information schema

In our `cmd/db-schema-cli/main.go` file, we need to set up the database connection, query the
schema and produce the raw structures that we need to generate any kind of code output.

~~~go
func main() {
	var config struct {
		db struct {
			DSN    string
			Driver string
		}
		Schema string
		Format string
	}
	flag.StringVar(&config.db.Driver, "db-driver", "mysql", "Database driver")
	flag.StringVar(&config.db.DSN, "db-dsn", "", "DSN for database connection")
	flag.StringVar(&config.Schema, "schema", "", "Schema name to print tables for")
	flag.StringVar(&config.Format, "format", "go", "Output formatting")
	flag.Parse()

	handle, err := sqlx.Connect(config.db.Driver, config.db.DSN)
	if err != nil {
		log.Fatalf("Error connecting to database: %+v", err)
	}
~~~

We set up configuration flags similarly to `db-migrate` cli program, taking the database driver,
connection DSN, the schema we want to inspect, and finally an output format. By default we will
generate Go code, but other output formats (and possibly new options) will come in handy for
generating the database documentation.

Next, we read the table information, and fill out the column information for each table.

~~~go
// List tables in schema
tables := []*Table{}
fields := strings.Join((*Table)(nil).Fields(), ", ")
err = handle.Select(&tables, "select "+fields+" from information_schema.tables where table_schema=? order by table_name asc", config.Schema)
if err != nil {
	log.Println("Error listing database tables")
	log.Fatal(err)
}

// List columns in tables
for _, table := range tables {
	fields := strings.Join((*Column)(nil).Fields(), ", ")
	err := handle.Select(&table.Columns, "select "+fields+" from information_schema.columns where table_schema=? and table_name=? order by ordinal_position asc", config.Schema, table.Name)
	if err != nil {
		log.Println("Error listing database columns for table:", table.Name)
		log.Fatal(err)
	}
}
~~~

All we need now is some bridging code to invoke our Go code rendering:

~~~go
// Render go structs
if config.Format == "go" {
	if err := renderGo(config.Schema, tables); err != nil {
		log.Fatal(err)
	}
}
~~~

We are passing the schema name in order to set the generated package name, as well as all the table information.

## Type conversions

When we're talking about database columns, we have two type columns to consider: `data_type`, which holds
the raw type, and `column_type`, which is the type augmented with it's restrictions, like the length, or
if a numeric type is a signed/unsigned number. The latter is important to differentiate `int64` and `uint64`.

We can split the types into three distinct requirements:

- numeric types which can be signed/unsigned (combined type),
- simple types which have direct translations into Go,
- special types that need a package import (`datetime` -> `*time.Time`)

The mappings for each type should be simple:

~~~go
var numericTypes map[string]string = map[string]string{
	"tinyint":  "int8",
	"smallint": "int16",
	// `mediumint` - this one would, technically, be int24 (3 bytes), but
	"mediumint": "int32",
	"int":       "int32",
	"bigint":    "int64",
}

func isNumeric(column *Column) (string, bool) {
	val, ok := numericTypes[column.DataType]
	return val, ok
}
~~~

All the numeric types have the characteristic, that an unsigned value just prepends `u` in front of
the signed mapped type. This way an unsigned `int32` will become an `uint32`.

~~~go
var simpleTypes map[string]string = map[string]string{
	"char":       "string",
	"varchar":    "string",
	"text":       "string",
	"longtext":   "string",
	"mediumtext": "string",
	"tinytext":   "string",
	"longblob":   "[]byte",
	"blob":       "[]byte",
	"varbinary":  "[]byte",
	// `float` and `double` are here since they don't have unsigned modifiers
	"float":  "float32",
	"double": "float64",
	// `decimal` - double stored as string, \o/
	"decimal": "string",
}

func isSimple(column *Column) (string, bool) {
	val, ok := simpleTypes[column.DataType]
	return val, ok
}
~~~

Simple types, like numeric types, return a 1-1 type mappping. As these values are not signed/unsigned,
the returned mapped type will stay as-is.

~~~go
type specialType struct {
	Import string
	Type   string
}

var specialTypes map[string]specialType = map[string]specialType{
	"date":      specialType{"time", "*time.Time"},
	"datetime":  specialType{"time", "*time.Time"},
	"time":      specialType{"time", "*time.Time"},
	"timestamp": specialType{"time", "*time.Time"},
	// `enum` and `set` aren't implemented
	// `year` isn't implemented
}

func isSpecial(column *Column) (specialType, bool) {
	val, ok := specialTypes[column.DataType]
	return val, ok
}
~~~

The special types are a bit more complex in the sense that they also provide an import. We cover the
built in date/time fields, but we do have other fields that are currently omitted. MySQL has a `JSON`
field type, for which we'd have to import `github.com/jmoiron/sqlx/types`, and use `types.JSONText` in
order to have it usable. There are also other types there, like `GzippedText`, and an implementation of
a `BIT(1)` type that scans the value into a `bool` field. We won't be implementing these for our use
case, but the example just goes to show, how quickly the requirements can grow.

Since a schema can have many types that import a package, we need to keep these packages unique:

~~~go
func contains(set []string, value string) bool {
	for _, v := range set {
		if v == value {
			return true
		}
	}
	return false
}
~~~

The `contains()` helper helps us out by having a way to check if an import has already been added.
Which leaves us with just the actual function to generate the relevant Go code:

~~~go
func renderGo(schema string, tables []*Table) error {
	imports := []string{}

	resolveType := func(column *Column) (string, error) {
		if val, ok := isSimple(column); ok {
			return val, nil
		}
		if val, ok := isNumeric(column); ok {
			isUnsigned := strings.Contains(strings.ToLower(column.Type), "unsigned")
			if isUnsigned {
				return "u" + val, nil
			}
			return val, nil
		}
		if val, ok := isSpecial(column); ok {
			if !contains(imports, val.Import) {
				imports = append(imports, val.Import)
			}
			return val.Type, nil
		}
		return "", errors.Errorf("Unsupported SQL type: %s", column.DataType)
	}

	// Loop through tables/columns, return type error if any
	// This also builds the `imports` slice for codegen lower
	for _, table := range tables {
		for _, column := range table.Columns {
			if _, err := resolveType(column); err != nil {
				return err
			}
		}
	}
~~~

Before generating any code, we pass through all columns and call `resolveType`, to declare all imports,
and to error out if any of the used types on the table can't be resolved to a Go type. We want to do
that as soon as possible.

~~~go
	fmt.Printf("package %s\n", schema)
	fmt.Println()

	// Print collected imports
	if len(imports) > 0 {
		fmt.Println("import (")
		for _, val := range imports {
			fmt.Printf("\t\"%s\"\n", val)
		}
		fmt.Println(")")
		fmt.Println()
	}
~~~

Printing the declared imports is trivial. If we aren't relying on any special types, the imports lines
will be omitted fully.

~~~go
	for _, table := range tables {
		fields := []string{}
		primary := []string{}
		if table.Comment != "" {
			fmt.Println("//", table.Comment)
		}
		fmt.Printf("type %s struct {\n", camel(table.Name))
		for idx, column := range table.Columns {
			fields = append(fields, column.Name)
			if column.Key == "PRI" {
				primary = append(primary, column.Name)
			}

			if column.Comment != "" {
				if idx > 0 {
					fmt.Println()
				}
				fmt.Printf("	// %s\n", column.Comment)
			}
			columnType, _ := resolveType(column)
			fmt.Printf("	%s %s `db:\"%s\"`\n", camel(column.Name), columnType, column.Name)
		}
		fmt.Println("}")
		fmt.Println()
		fmt.Printf("func (*%s) Fields() []string {\n", camel(table.Name))
		if len(fields) > 0 {
			fmt.Printf("\treturn []string{\"%s\"}\n", strings.Join(fields, "\", \""))
		} else {
			fmt.Printf("\treturn []string{}\n")
		}
		fmt.Println("}")
		fmt.Println()
		fmt.Printf("func (*%s) PrimaryFields() []string {\n", camel(table.Name))
		if len(primary) > 0 {
			fmt.Printf("\treturn []string{\"%s\"}\n", strings.Join(primary, "\", \""))
		} else {
			fmt.Printf("\treturn []string{}\n")
		}
		fmt.Println("}")
	}
	return nil
~~~

Finally, for each table we collect the table fields, the primary key fields, and print out the relevant Go
structures and functions to support them. A big part of generating the `Fields` and `PrimaryFields` functions
is to avoid reflection. A migration to add a new field could break the service, if a `select * from [table]...`
style SQL query would be used. A newly added field would result in an error executing the query. By selecting
fields explicitly, we can avoid this scenario.

In fact, there's little use to keep `Fields` and `PrimaryFields` as functions on the structure, so we'll
immediately modify this to generate a public, service-prefixed variable.

~~~go
fmt.Printf("var %sFields []string = ", camel(table.Name))
if len(fields) > 0 {
	fmt.Printf("[]string{\"%s\"}", strings.Join(fields, "\", \""))
} else {
	fmt.Printf("[]string{}")
}
fmt.Println()
fmt.Printf("var %sPrimaryFields []string = ", camel(table.Name))
if len(primary) > 0 {
	fmt.Printf("[]string{\"%s\"}", strings.Join(primary, "\", \""))
} else {
	fmt.Printf("[]string{}")
}
fmt.Println()
~~~

All that's left to do at this point is to run and verify the generated code.

~~~go
package stats

type Migrations struct {
	// Microservice or project name
	Project string `db:"project"`

	// yyyy-mm-dd-HHMMSS.sql
	Filename string `db:"filename"`

	// Statement number from SQL file
	StatementIndex int32 `db:"statement_index"`

	// ok or full error message
	Status string `db:"status"`
}

var MigrationsFields []string = []string{"project", "filename", "statement_index", "status"}
var MigrationsPrimaryFields []string = []string{"project", "filename"}
~~~

That's exactly what we wanted. We generate the full Go structure with all required metadata to support
our Go development workflows. The comments are generated from the database schema, so we have this information
available at any time - if a DBA is managing the database schema in their favorite editor, or if a developer
is reading the Go struct definitions or internal godoc pages. Documentation accessibility is built into our
process.

All that's left to do now is to include our generator into the Makefile, under the `migrate.%:` target:

~~~
./build/db-schema-cli-linux-amd64 -schema $(SERVICE) -db-dsn "root:$(MYSQL_ROOT_PASSWORD)@tcp(mysql-test:3306)/$(SERVICE)" > server/$(SERVICE)/types_db.go
~~~

Now, we have some specific requirements to consider, which we are solving by generating `types_db.go` under
individual services which we are making. Let's consider how having a global types package would be problematic:

- It's likely we would have naming conflicts for tables (example: `log` table existing in multiple services),
- Prefixing the structures with the service name would introduce stutter (`stats.stats_log` becomes `StatsStatsLog`)
- We really only need the database structures in our service implementation. At worst, we need to convert them to PB.

In some casses, the possible mapping between DB and PB structures could be 1-1, but it's likely we're going to
have different types for PB (javascript client's can't really handle uint64 types so they have to be strings),
and we will always have data which is available in the database, which must never be exposed publicly (JSON?).

In fact, I have half of mind to add `json:"-"` tags to all the fields and see how far that brings me. It really
isn't a bad idea, so...

~~~
--- a/cmd/db-schema-cli/render-go.go
+++ b/cmd/db-schema-cli/render-go.go
@@ -128,7 +128,7 @@ func renderGo(schema string, tables []*Table) error {
                                fmt.Printf("    // %s\n", column.Comment)
                        }
                        columnType, _ := resolveType(column)
-                       fmt.Printf("    %s %s `db:\"%s\"`\n", camel(column.Name), columnType, column.Name)
+                       fmt.Printf("    %s %s `db:\"%s\" json:\"-\"`\n", camel(column.Name), columnType, column.Name)
                }
                fmt.Println("}")
                fmt.Println()
~~~

I think this is what people are referring to when they say you have to future-proof your code. I'm not counting
on the fact that the structures defined here will never be exported, we might have to come back and patch our
code generation in this case, but it's way better to start with paranoid defaults, and then open up your system.
It's way harder to disable something, than to enable it.

## Moving forward

As we didn't create a real migration that would create any tables yet, the only table that's dumped as Go code is
the Migrations table. Our next steps will be to create some real migrations for our table, and see if we can
already define some coner cases where we're going to need to go outside of the current implementation.

As we said, it's our goal to actively generate the documentation from SQL as our data source, which also means
we're going to be generating some markdown output from the database tables we'll be working with.
