package main

type Table struct {
	Name    string `db:"TABLE_NAME"`
	Comment string `db:"TABLE_COMMENT"`

	Columns []*Column
}

var TableFields = []string{"TABLE_NAME", "TABLE_COMMENT"}

type Column struct {
	Name    string `db:"COLUMN_NAME"`
	Type    string `db:"COLUMN_TYPE"`
	Key     string `db:"COLUMN_KEY"`
	Comment string `db:"COLUMN_COMMENT"`

	// Holds the clean data type
	DataType string `db:"DATA_TYPE"`
}

var ColumnFields = []string{"COLUMN_NAME", "COLUMN_TYPE", "COLUMN_KEY", "COLUMN_COMMENT", "DATA_TYPE"}
