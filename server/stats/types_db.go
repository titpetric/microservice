package stats

type Migrations struct {
	// Microservice or project name
	Project string `db:"project" json:"-"`

	// yyyy-mm-dd-HHMMSS.sql
	Filename string `db:"filename" json:"-"`

	// Statement number from SQL file
	StatementIndex int32 `db:"statement_index" json:"-"`

	// ok or full error message
	Status string `db:"status" json:"-"`
}

var MigrationsFields []string = []string{"project", "filename", "statement_index", "status"}
var MigrationsPrimaryFields []string = []string{"project", "filename"}
