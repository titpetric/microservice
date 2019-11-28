package main

import (
	"strings"
)

func Camel(input string) string {
	parts := strings.Split(input, "_")
	for k, v := range parts {
		parts[k] = strings.ToUpper(v[0:1]) + v[1:]
	}
	return strings.Join(parts, "")
}
