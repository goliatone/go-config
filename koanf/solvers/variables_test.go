package solvers

import (
	"testing"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
)

func TestKSolver_Variables(t *testing.T) {
	notMatching := "${nothing}"
	defaultValues := map[string]any{
		"server": map[string]any{
			"base_url": "${base_url}",
		},
		"version":  "0.23.45",
		"base_url": "http://localhost:3333",
		"context": map[string]any{
			"version": "${version}",
		},
		"not_matching": notMatching,
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewVariablesSolver("${", "}")
	out := solver.Solve(k)

	assert.Equal(
		t,
		out.Get("server.base_url"),
		out.Get("base_url"),
	)

	assert.Equal(
		t,
		out.Get("context.version"),
		out.Get("version"),
	)

	assert.Equal(
		t,
		notMatching,
		out.Get("not_matching"),
	)
}

func TestKSolver_Variables_custom_delimeters(t *testing.T) {
	notMatching := "@/nothing/"
	defaultValues := map[string]any{
		"server": map[string]any{
			"base_url": "@/base_url/",
		},
		"version":  "0.23.45",
		"base_url": "http://localhost:3333",
		"context": map[string]any{
			"version": "@/version/",
		},
		"not_matching": notMatching,
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewVariablesSolver("@/", "/")
	out := solver.Solve(k)

	assert.Equal(
		t,
		out.Get("base_url"),
		out.Get("server.base_url"),
	)

	assert.Equal(
		t,
		out.Get("version"),
		out.Get("context.version"),
	)

	assert.Equal(
		t,
		notMatching,
		out.Get("not_matching"),
	)
}

func TestKSolver_Variables_custom_delimeters2(t *testing.T) {
	notMatching := "{{nothing}}"
	defaultValues := map[string]any{
		"server": map[string]any{
			"base_url": "{{base_url}}",
		},
		"version":  "0.23.45",
		"base_url": "http://localhost:3333",
		"context": map[string]any{
			"version": "{{version}}",
		},
		"not_matching": notMatching,
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewVariablesSolver("{{", "}}")
	out := solver.Solve(k)

	assert.Equal(
		t,
		out.Get("base_url"),
		out.Get("server.base_url"),
	)

	assert.Equal(
		t,
		out.Get("version"),
		out.Get("context.version"),
	)

	assert.Equal(
		t,
		notMatching,
		out.Get("not_matching"),
	)
}

func TestKSolver_Variables_non_matching(t *testing.T) {
	notMatching := "${nothing}"
	defaultValues := map[string]any{
		"server": map[string]any{
			"base_url": "${base_url}",
		},
		"version":  "0.23.45",
		"base_url": "http://localhost:3333",
		"context": map[string]any{
			"version": "${version}",
		},
		"not_matching": notMatching,
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewVariablesSolver("${", "}")
	out := solver.Solve(k)

	assert.Equal(
		t,
		out.Get("base_url"),
		out.Get("server.base_url"),
	)

	assert.Equal(
		t,
		out.Get("version"),
		out.Get("context.version"),
	)

	assert.Equal(
		t,
		notMatching,
		out.Get("not_matching"),
	)
}

func TestKSolver_Variables_embedded(t *testing.T) {
	defaultValues := map[string]any{
		"host": "localhost",
		"lang": "en",
		"server": map[string]any{
			"endpoint": "http://${host}/api/v0/${lang}",
		},
	}

	k := koanf.New(".")
	k.Load(confmap.Provider(defaultValues, "."), nil)

	solver := NewVariablesSolver("${", "}")
	out := solver.Solve(k)

	assert.Equal(t, "http://localhost/api/v0/en", out.Get("server.endpoint"))
}
