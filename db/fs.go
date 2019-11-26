package db

import (
	"os"
	"sort"

	"encoding/base64"
	"path/filepath"
)

type FS map[string]string

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

// ReadFile returns decoded file contents from FS
func (fs FS) ReadFile(filename string) ([]byte, error) {
	if val, ok := fs[filename]; ok {
		return base64.StdEncoding.DecodeString(val)
	}
	return nil, os.ErrNotExist
}
