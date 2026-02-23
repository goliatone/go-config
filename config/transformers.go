package config

import "strings"

type StringTransformer func(string) (string, error)

func TrimSpace(value string) (string, error) {
	return strings.TrimSpace(value), nil
}

func ToLower(value string) (string, error) {
	return strings.ToLower(value), nil
}

func ToUpper(value string) (string, error) {
	return strings.ToUpper(value), nil
}

func EnsureLeadingSlash(value string) (string, error) {
	if value == "" || strings.HasPrefix(value, "/") {
		return value, nil
	}
	return "/" + value, nil
}
