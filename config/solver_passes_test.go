package config

import (
	"context"
	"testing"
)

type solverPassConfig struct {
	Foo   string `koanf:"foo"`
	Value string `koanf:"value"`
}

func (c solverPassConfig) Validate() error { return nil }

func TestContainerSolvers_MaxPasses(t *testing.T) {
	cfg := &solverPassConfig{}
	defaultValues := map[string]any{
		"foo":   "bar",
		"value": `{{ "$" + "{foo}" }}`,
	}

	container := New(cfg).
		WithConfigPath("").
		WithProvider(DefaultValuesProvider[*solverPassConfig](defaultValues)).
		WithSolverPasses(1)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Value != "${foo}" {
		t.Fatalf("expected value to remain unresolved, got %q", cfg.Value)
	}
}

func TestContainerSolvers_MultiPassResolves(t *testing.T) {
	cfg := &solverPassConfig{}
	defaultValues := map[string]any{
		"foo":   "bar",
		"value": `{{ "$" + "{foo}" }}`,
	}

	container := New(cfg).
		WithConfigPath("").
		WithProvider(DefaultValuesProvider[*solverPassConfig](defaultValues)).
		WithSolverPasses(2)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Value != "bar" {
		t.Fatalf("expected value to resolve after second pass, got %q", cfg.Value)
	}
}
