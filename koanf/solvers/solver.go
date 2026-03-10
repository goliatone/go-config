package solvers

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type ConfigSolver interface {
	Solve(config *koanf.Koanf) *koanf.Koanf
}

// ErrorReporter is an optional extension for solvers that can surface
// recoverable solver errors to container orchestration code.
type ErrorReporter interface {
	Err() error
}

func ToString(v any) string {
	return fmt.Sprint(v)
}

type delimiters struct {
	Start string
	End   string
}
