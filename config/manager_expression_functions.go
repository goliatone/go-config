package config

import (
	"strings"

	"github.com/goliatone/go-config/koanf/solvers"
	opts "github.com/goliatone/go-options"
)

// ExpressionFunction is a custom function callable from expression templates,
// for example {{ githash(7) }}.
type ExpressionFunction func(args ...any) (any, error)

// WithExpressionFunction registers or replaces an expression function by name.
func (c *Container[C]) WithExpressionFunction(name string, fn ExpressionFunction) *Container[C] {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || fn == nil {
		return c
	}
	if c.expressionFunctions == nil {
		c.expressionFunctions = map[string]ExpressionFunction{}
	}
	c.expressionFunctions[name] = fn
	return c
}

func (c *Container[C]) effectiveSolvers() []solvers.ConfigSolver {
	if len(c.solvers) == 0 {
		if len(c.expressionFunctions) == 0 {
			return nil
		}
		return []solvers.ConfigSolver{
			solvers.NewExpressionSolverWithEvaluator("{{", "}}", c.expressionEvaluator(), nil),
		}
	}

	eval := c.expressionEvaluator()
	if eval == nil {
		return c.solvers
	}

	out := make([]solvers.ConfigSolver, 0, len(c.solvers)+1)
	replaced := false

	for _, solver := range c.solvers {
		updated, ok := solvers.ReplaceExpressionSolverEvaluator(solver, eval)
		if ok {
			replaced = true
			out = append(out, updated)
			continue
		}
		out = append(out, solver)
	}

	if !replaced {
		out = append(out, solvers.NewExpressionSolverWithEvaluator("{{", "}}", eval, nil))
	}

	return out
}

func (c *Container[C]) expressionEvaluator() opts.Evaluator {
	if len(c.expressionFunctions) == 0 {
		return nil
	}

	registry := opts.NewFunctionRegistry()
	for name, fn := range c.expressionFunctions {
		if fn == nil || strings.TrimSpace(name) == "" {
			continue
		}
		_ = registry.Register(name, opts.Function(fn))
	}

	return opts.NewExprEvaluator(opts.ExprWithFunctionRegistry(registry))
}
