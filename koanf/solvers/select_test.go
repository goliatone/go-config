package solvers

import (
	"errors"
	"testing"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
)

func TestSelectSolver_ResolvesTemplateSelector(t *testing.T) {
	defaultValues := map[string]any{
		"app": map[string]any{
			"env": "development",
		},
		"reminders": map[string]any{
			"$select":  "${app.env}",
			"$default": "production",
			"development": map[string]any{
				"max_reminders": 2,
			},
			"production": map[string]any{
				"max_reminders": 5,
			},
		},
	}

	k := koanf.New(".")
	_ = k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewSelectSolver("$select", "$default")
	out := solver.Solve(k)

	assert.Equal(t, 2, out.Get("reminders.max_reminders"))
	assert.False(t, out.Exists("reminders.$select"))
	assert.False(t, out.Exists("reminders.development"))
	assert.False(t, out.Exists("reminders.production"))
}

func TestSelectSolver_ResolvesPlainPathSelector(t *testing.T) {
	defaultValues := map[string]any{
		"app": map[string]any{
			"env": "production",
		},
		"reminders": map[string]any{
			"$select":  "app.env",
			"$default": "development",
			"development": map[string]any{
				"max_reminders": 2,
			},
			"production": map[string]any{
				"max_reminders": 5,
			},
		},
	}

	k := koanf.New(".")
	_ = k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewSelectSolver("$select", "$default")
	out := solver.Solve(k)

	assert.Equal(t, 5, out.Get("reminders.max_reminders"))
}

func TestSelectSolver_ResolvesLiteralSelector(t *testing.T) {
	defaultValues := map[string]any{
		"reminders": map[string]any{
			"$select":  "production",
			"$default": "development",
			"development": map[string]any{
				"max_reminders": 2,
			},
			"production": map[string]any{
				"max_reminders": 5,
			},
		},
	}

	k := koanf.New(".")
	_ = k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewSelectSolver("$select", "$default")
	out := solver.Solve(k)

	assert.Equal(t, 5, out.Get("reminders.max_reminders"))
}

func TestSelectSolver_UsesDefaultWhenBranchMissing(t *testing.T) {
	defaultValues := map[string]any{
		"app": map[string]any{
			"env": "staging",
		},
		"reminders": map[string]any{
			"$select":  "${app.env}",
			"$default": "production",
			"development": map[string]any{
				"max_reminders": 2,
			},
			"production": map[string]any{
				"max_reminders": 5,
			},
		},
	}

	k := koanf.New(".")
	_ = k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewSelectSolver("$select", "$default")
	out := solver.Solve(k)

	assert.Equal(t, 5, out.Get("reminders.max_reminders"))
}

func TestSelectSolver_ReturnsErrorWhenNoBranchAndNoDefault(t *testing.T) {
	defaultValues := map[string]any{
		"app": map[string]any{
			"env": "staging",
		},
		"reminders": map[string]any{
			"$select": "${app.env}",
			"development": map[string]any{
				"max_reminders": 2,
			},
		},
	}

	k := koanf.New(".")
	_ = k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewSelectSolver("$select", "$default")
	out := solver.Solve(k)
	assert.NotNil(t, out)

	reporter, ok := solver.(ErrorReporter)
	if !ok {
		t.Fatalf("expected select solver to implement ErrorReporter")
	}
	if reporter.Err() == nil {
		t.Fatalf("expected select solver error")
	}

	var selectErr *SelectResolutionError
	if !errors.As(reporter.Err(), &selectErr) {
		t.Fatalf("expected SelectResolutionError, got %v", reporter.Err())
	}
	assert.Equal(t, "reminders", selectErr.NodePath)
	assert.Equal(t, "app.env", selectErr.SelectPath)
	assert.Equal(t, "staging", selectErr.SelectValue)
	assert.Equal(t, "", selectErr.DefaultKey)
}

func TestSelectSolver_ResolvesNestedSelectsInOnePass(t *testing.T) {
	defaultValues := map[string]any{
		"app": map[string]any{
			"env": "production",
		},
		"region": "us",
		"reminders": map[string]any{
			"$select": "${app.env}",
			"development": map[string]any{
				"max_reminders": 2,
			},
			"production": map[string]any{
				"$select":  "${region}",
				"$default": "eu",
				"us": map[string]any{
					"max_reminders": 5,
				},
				"eu": map[string]any{
					"max_reminders": 4,
				},
			},
		},
	}

	k := koanf.New(".")
	_ = k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewSelectSolver("$select", "$default")
	out := solver.Solve(k)

	assert.Equal(t, 5, out.Get("reminders.max_reminders"))
	reporter := solver.(ErrorReporter)
	assert.NoError(t, reporter.Err())
}

func TestSelectSolver_NoOpWithoutSelect(t *testing.T) {
	defaultValues := map[string]any{
		"reminders": map[string]any{
			"max_reminders": 5,
		},
	}

	k := koanf.New(".")
	_ = k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewSelectSolver("$select", "$default")
	out := solver.Solve(k)

	assert.Equal(t, 5, out.Get("reminders.max_reminders"))
}
