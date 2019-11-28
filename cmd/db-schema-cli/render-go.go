package main

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

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
	}
	return nil
}
