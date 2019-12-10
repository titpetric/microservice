# Go: Generating database schema documentation

Since we are testing migrations and generating Go struct types from the database
schema, we also have all the available information to generate documentation snippets
for each schema. We will generate markdown formatted tables with the final
database schema, after all the migrations have been applied.

## Updating the Go generators

We need to adjust the generator code, since each table will need to be rendered in
an individual markdown file. We need to add a parameter to db-schema-cli, which we
will pass to specify an output folder.

~~~diff
--- a/cmd/db-schema-cli/main.go
+++ b/cmd/db-schema-cli/main.go
@@ -18,11 +18,13 @@ func main() {
                }
                Schema string
                Format string
+               Output string
        }
        flag.StringVar(&config.db.Driver, "db-driver", "mysql", "Database driver")
        flag.StringVar(&config.db.DSN, "db-dsn", "", "DSN for database connection")
        flag.StringVar(&config.Schema, "schema", "", "Schema name to print tables for")
        flag.StringVar(&config.Format, "format", "go", "Output formatting")
+       flag.StringVar(&config.Output, "output", "", "Output folder (mandatory)")
        flag.Parse()

+       if config.Output == "" {
+               log.Fatal("Missing -output parameter, please specify output folder")
+       }
~~~

We can now specifiy `-output [folder]` as an argument to db-schema-cli. Let's adjust
the signature for `renderGo` to take this parameter, so we will generate Go code
in the folder specified with the same parameter. We can also add `renderMarkdown`
at the same time:

~~~go
// Render go structs
if config.Format == "go" {
	if err := renderGo(config.Output, config.Schema, tables); err != nil {
		log.Fatal(err)
	}
}

// Render markdown tables
if config.Format == "markdown" {
	if err := renderMarkdown(config.Output, config.Schema, tables); err != nil {
		log.Fatal(err)
	}
}
~~~

Please create a `render-md.go` file, which contains the following:

~~~
package main

func renderMarkdown(basePath string, schema string, tables []*Table) error {
	return nil
}
~~~

The stub is here just so we can compile the source as we need to do some housekeeping
for the renderGo function. Since we want to write to a file from renderGo, we will
create a bytes buffer, and modify our various `fmt.Print*` calls into `fmt.Fprint*`,
which takes an `io.Writer` as an additional first parameter. The code in the renderGo
function becomes something like this:

~~~go
buf := bytes.NewBuffer([]byte{})

fmt.Fprintf(buf, "package %s\n", schema)
fmt.Fprintln(buf)
// ...
~~~

Now, we can get the generated source code by calling `buf.Bytes()`. We can also load the
`go/format` package, and push the source code through a formatter. At the tail end of the
renderGo function, add the following code which formats the source and saves `types_db.go`.

~~~go
filename := path.Join(basePath, "types_db.go")
contents := buf.Bytes()

formatted, err := format.Source(contents)
if err != nil {
	// fall back to unformatted source to inspect
	// the saved file for the error which occured
	formatted = contents
	log.Println("An error occured while formatting the go source: %s", err)
	log.Println("Saving the unformatted code")
}

fmt.Println(filename)

return ioutil.WriteFile(filename, formatted, 0644)
~~~

The Makefile command to invoke the go struct generator changes. Let's just explicitly
add `-format` and `-output` so we can generate the file in the specified folder.

~~~diff
- > server/$(SERVICE)/types_db.go
+ -format go -output server/$(SERVICE)
~~~

We can verify if types_db.go is still generated correctly. Let's run make and verify.
Somewhere in the output of `make`, the following line shows up:

~~~plaintext
[migrations:18] server/stats/types_db.go
~~~

This means that the `fmt.Println` was executed, and the file was written just after
that. If we open the file and verify, we will see that the generated struct is there,
so we didn't break anything.

## Implementing a markdown output renderer

We started with an empty stub, and now it's up to us to implement the renderer to Markdown.
For this, we need to set some simple requirements:

- we want to create the output dir
- each table needs to be in it's own file (`[table_name].md`),
- we will print the table name as the title,
- optionally we will print the table description if any,
- and produce a padded, human readable, markdown table with the sql table structure

We can start with the very simple one:

~~~go
// create output folder
if err := os.MkdirAll(basePath, 0755); err != nil {
	return err
}
~~~

With a call to MkdirAll we recursively create the output folder. Since renderGo also needs
to create a folder, we copy the snippet into renderGo as well. Now, we can move on to loop
over the tables in our schema, and individually render the markdown contents for them.

~~~go
// generate individual markdown files with schema
for _, table := range tables {
	filename := path.Join(basePath, table.Name+".md")

	contents := renderMarkdownTable(table)
	if err := ioutil.WriteFile(filename, contents, 0644); err != nil {
		return err
	}

	fmt.Println(filename)
}
return nil
~~~

This function will break out if an error occurs on ioutil.WriteFile. Possible errors
include you not having write permissions for the file, or out of disk space. As renderMarkdown
returns an error, we can pass this information back to main() and error out with a
relevant error message.

Now we can move to the individual table markdown output generator. Now, markdown can
read tables in the following format:

~~~markdown
|Name|Type|Key|Comment|
|--|--|--|--|
|1|2|3|4|
~~~

The obvious issue is that without correct padding, this isn't very human readable.
We want to generate output as close to this as possible:

~~~markdown
| Name | Type | Key | Comment |
|------|------|-----|---------|
| 1    | 2    | 3   | 4       |
~~~

Now, we immediately notice something - the heading of the table also counts as the
length of a column, and adds to the padding below. We need to set the initial padding
to the length of the column title:

~~~go
// calculate initial padding from table header
titles := []string{"Name", "Type", "Key", "Comment"}
padding := map[string]int{}
for _, v := range titles {
	padding[v] = len(v)
}
~~~

After this, we need to loop through each table column, and adjust the padding for
each markdown column depending on the data returned. To make the code a bit clearer,
we also create a local `max(a,b int) int` function.

~~~go
max := func(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// calculate max length for columns for padding
for _, column := range table.Columns {
	padding["Name"] = max(padding["Name"], len(column.Name))
	padding["Type"] = max(padding["Type"], len(column.Type))
	padding["Key"] = max(padding["Key"], len(column.Key))
	padding["Comment"] = max(padding["Comment"], len(column.Comment))
}
~~~

Now, we should have the required length for each column in the `padding` variable.
Naively, we could resort to something like `strings.Repeat` and length checks to
calculate the padding required based on the cell contents length, but we can resort
to the formatting features of `fmt` print functions, namely the width value for any type.

~~~
%f     default width, default precision
%9f    width 9, default precision
%.2f   default width, precision 2
%9.2f  width 9, precision 2
%9.f   width 9, precision 0
~~~

With our particular case, we need to print `%10s` to pad a string to the length of 10.
By default, the string is padded on the left, but there's a flag for that as well. We can
print `%-10s`, and the field will be left justified, like we want.

~~~go
// use fmt.Sprintf to add padding to columns, left align columns
format := strings.Repeat("| %%-%ds ", len(padding)) + "|\n"

// %%-%ds becomes %-10s, which right-pads string to len=10
paddings := []interface{}{
	padding["Name"],
	padding["Type"],
	padding["Key"],
	padding["Comment"],
}
format = fmt.Sprintf(format, paddings...)
~~~

Ultimately, we end up with a formatting string which we can pass to fmt.Sprintf and can be
used for each row of the table. The string may be something like `| %-10s | %-5s | %-3s | %-50s |`.
Now, the print functions in fmt take `...interface{}` as the parameters, so unfortunately, we can't
just use `titles...` since the types don't match. We need to coerce the type into `[]interface{}`,
just like we did with the paddings above.

~~~go
// create initial buffer with table name
buf := bytes.NewBufferString(fmt.Sprintf("# %s\n\n", table.Name))

// and comment
if table.Comment != "" {
	buf.WriteString(fmt.Sprintf("%s\n\n", table.Comment))
}

// write header row strings to the buffer
row := []interface{}{"Name", "Type", "Key", "Comment"}
buf.WriteString(fmt.Sprintf(format, row...))
~~~

After the header row, we need to output the divider. All the padding from the values
should just be replaced with dash characters, all we need is to pass a row with empty columns.

~~~go
// table header/body delimiter
row = []interface{}{"", "", "", ""}
buf.WriteString(strings.Replace(fmt.Sprintf(format, row...), " ", "-", -1))
~~~

And now we are left with printing the columns:

~~~go
// table body
for _, column := range table.Columns {
	row := []interface{}{column.Name, column.Type, column.Key, column.Comment}
	buf.WriteString(fmt.Sprintf(format, row...))
}

// return byte slice for writing to file
return buf.Bytes()
~~~

Edit the Makefile to add the db-schema-cli renderer with the following parameters:

- `-format markdown` - sets the renderer,
- `-output docs/schema/$(SERVICE)` - sets the output path

At the end of the `migrate.%` target, copy the following line:

~~~Makefile
	./build/db-schema-cli-linux-amd64 -schema $(SERVICE) -db-dsn "root:$(MYSQL_ROOT_PASSWORD)@tcp(mysql-test:3306)/$(SERVICE)" -format markdown -output docs/schema/$(SERVICE)
~~~

When running make we can verify that the migrations for our services are written.

~~~plaintext
...
[migrations:17] ./build/db-schema-cli-linux-amd64 -schema stats -db-dsn "root:default@tcp(mysql-test:3306)/stats" -format go -output server/stats
[migrations:18] server/stats/types_db.go
[migrations:19] ./build/db-schema-cli-linux-amd64 -schema stats -db-dsn "root:default@tcp(mysql-test:3306)/stats" -format markdown -output docs/schema/stats
[migrations:20] docs/schema/stats/migrations.md
[migrations:21] OK.
...
~~~

And we can check the output to verify that it's padded correctly:

~~~markdown
# migrations

| Name            | Type         | Key | Comment                        |
|-----------------|--------------|-----|--------------------------------|
| project         | varchar(16)  | PRI | Microservice or project name   |
| filename        | varchar(255) | PRI | yyyy-mm-dd-HHMMSS.sql          |
| statement_index | int(11)      |     | Statement number from SQL file |
| status          | text         |     | ok or full error message       |
~~~

## A caveat emptor about database migrations

We have now a system in place that:

- tests SQL migrations with Drone CI integration tests,
- generates structs based on the SQL tables in our schema,
- generates documentation snippets for SQL tables

By approaching our service development in this way, we have made possible to create
SQL schema and migrations with database specific tooling like [MySQL workbench](https://www.mysql.com/products/workbench/),
while at the same time keeping a single source of truth for the data structures in use.

The SQL schema can be updated with migrations, and the Drone CI steps will ensure that the
Go structures are updated as well. If some field is renamed it will be a breaking change
and you will need to rename the Go field names in use by hand. Since we're not using
the database structures for JSON encoding, we can at least be sure, that any breaking
changes in regards to field naming will not trickle out to your end users.

The most significant warning about running migrations separately from the service is that
breaking changes in the service need to be deployed in a coordinated way. The SQL migration
will break the deployed service and there will be some time before a new version of the
service may be running. This is an inheren problematic of distributed systems.

If this is a particular pain point for you, and it should be, you should perform the database
migrations in a non breaking way. For example, renaming a field safely might mean the following:

1. Migration: Add a new field,
2. Service: Write both to new and old fields,
3. Migration: `UPDATE table set new_field=old_field;`,
4. Service: Stop writing/referencing old fields,
5. Migration: DROP the old field from the table

The process isn't easy to automate but illustrates why in production service deployments,
it's good to have a DBA for assistance. Coupling the service and migrations together brings
other problems, like having long running migration steps and concurrency issues since your
service might be deployed over many replicas and hosts.

Controlled upgrades are usually a task that's performed by humans, as much as we would like to
automate this there can be various technical circumstances which limit you as to when you can
perform schema migrations or a redeploy of your services, especially in distributed systems.