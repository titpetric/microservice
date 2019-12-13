package main

import (
	"github.com/serenize/snaker"
)

func camel(input string) string {
	return snaker.SnakeToCamel(input)
}

func contains(set []string, value string) bool {
	for _, v := range set {
		if v == value {
			return true
		}
	}
	return false
}
