package db

import (
	"regexp"
	"strings"
)

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
