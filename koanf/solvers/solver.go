package solvers

import (
	"context"
	"fmt"

	"github.com/knadh/koanf/v2"
)

// FinalSolver runs once on private selected input, before typed decoding.
type FinalSolver interface {
	SolveContext(context.Context, map[string]any) error
}

type FinalSolverFunc func(context.Context, map[string]any) error

func (f FinalSolverFunc) SolveContext(ctx context.Context, values map[string]any) error {
	return f(ctx, values)
}

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
