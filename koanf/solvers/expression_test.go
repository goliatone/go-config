package solvers

import (
	"testing"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
)

func TestExpressionSolver_EvaluatesFullMatch(t *testing.T) {
	defaultValues := map[string]any{
		"app": map[string]any{
			"env":  "development",
			"name": "MyApp",
		},
		"debug":    `{{ app.env == "development" }}`,
		"label":    `{{ app.name + "-" + app.env }}`,
		"sum":      "{{ 1 + 2 }}",
		"embedded": "prefix {{ 1 + 1 }}",
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewExpressionSolver("{{", "}}")
	out := solver.Solve(k)

	assert.Equal(t, true, out.Get("debug"))
	assert.Equal(t, "MyApp-development", out.Get("label"))
	assert.EqualValues(t, 3, out.Get("sum"))
	assert.Equal(t, "prefix {{ 1 + 1 }}", out.Get("embedded"))
}

func TestExpressionSolver_OnEvalLeaveUnchanged(t *testing.T) {
	defaultValues := map[string]any{
		"bad": "{{ }}",
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewExpressionSolverWithEvaluator("{{", "}}", nil, OnEvalLeaveUnchanged())
	out := solver.Solve(k)

	assert.Equal(t, "{{ }}", out.Get("bad"))
}

func TestExpressionSolver_OnEvalRemove(t *testing.T) {
	defaultValues := map[string]any{
		"bad": "{{ }}",
		"ok":  "{{ 1 + 1 }}",
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewExpressionSolverWithEvaluator("{{", "}}", nil, OnEvalRemove())
	out := solver.Solve(k)

	assert.False(t, out.Exists("bad"))
	assert.EqualValues(t, 2, out.Get("ok"))
}

func TestExpressionSolver_OnEvalLogAndPanic(t *testing.T) {
	defaultValues := map[string]any{
		"bad": "{{ }}",
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewExpressionSolverWithEvaluator("{{", "}}", nil, OnEvalLogAndPanic(nil))

	assert.Panics(t, func() {
		solver.Solve(k)
	})
}
