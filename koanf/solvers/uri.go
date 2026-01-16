package solvers

import (
	"encoding/base64"
	"io/fs"
	"os"
	"strings"

	"github.com/knadh/koanf/v2"
)

type uris struct {
	fs         fs.FS
	delimeters *delimiters
}

// NewURISolver will resolve variables
func NewURISolver(s, e string) ConfigSolver {
	return NewURISolverWithFS(s, e, os.DirFS("."))
}

func NewURISolverWithFS(s, e string, f fs.FS) ConfigSolver {
	return &uris{
		fs: f,
		delimeters: &delimiters{
			Start: s,
			End:   e,
		},
	}
}

// Solve will transform a configuration object
func (s uris) Solve(config *koanf.Koanf) *koanf.Koanf {
	c := config.All()

	for key, val := range c {
		v2, ok := val.(string)
		if !ok {
			continue
		}
		s.keypath(key, v2, config)
	}

	return config
}

func (s uris) keypath(key, val string, config *koanf.Koanf) {
	start := strings.Index(val, s.delimeters.Start)
	if start != 0 {
		return
	}

	end := strings.Index(val, s.delimeters.End)
	if end < start {
		return
	}

	start = start + len(s.delimeters.Start)

	protocol := val[start:end]

	end = end + len(s.delimeters.End)
	uri := val[end:]

	// TODO: need to implement a way to dynamically register solvers
	switch protocol {
	case "file":
		if content, err := SolveFileProtocol(s.fs, uri); err == nil {
			config.Set(key, content)
		}
	case "base64":
		if content, err := SolveBase64DecodeProtocol(s.fs, uri); err == nil {
			config.Set(key, content)
		}
	}
}

func SolveFileProtocol(f fs.FS, uri string) (string, error) {
	content := ""
	b, err := fs.ReadFile(f, uri)
	if err == nil {
		content = string(b)
		content = strings.TrimRight(content, "\n")
	}
	return content, err
}

func SolveBase64DecodeProtocol(f fs.FS, uri string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(uri)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
