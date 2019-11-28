package main

import (
	"strings"
)

func camel(input string) string {
	parts := strings.Split(input, "_")
	for k, v := range parts {
		parts[k] = strings.ToUpper(v[0:1]) + v[1:]
	}
	return strings.Join(parts, "")
}

func contains(set []string, value string) bool {
	for _, v := range set {
		if v == value {
			return true
		}
	}
	return false
}
