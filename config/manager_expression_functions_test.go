package config

import (
	"context"
	"fmt"
	"testing"

	"github.com/goliatone/go-config/koanf/solvers"
)

type expressionFunctionsConfig struct {
	App struct {
		Hash string `koanf:"hash"`
	} `koanf:"app"`
}

func (c *expressionFunctionsConfig) Validate() error { return nil }

func TestContainerWithExpressionFunction_ResolvesTemplate(t *testing.T) {
	cfg := &expressionFunctionsConfig{}
	container := New(cfg).
		WithConfigPath("").
		WithProvider(DefaultValuesProvider[*expressionFunctionsConfig](map[string]any{
			"app": map[string]any{
				"hash": "{{ githash(7) }}",
			},
		})).
		WithExpressionFunction("githash", func(args ...any) (any, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("githash expects one argument")
			}
			return "f9d293c", nil
		})

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.App.Hash != "f9d293c" {
		t.Fatalf("expected app hash to resolve, got %q", cfg.App.Hash)
	}
}

func TestContainerWithExpressionFunction_AppendsExpressionSolverWhenMissing(t *testing.T) {
	cfg := &expressionFunctionsConfig{}
	container := New(cfg).
		WithConfigPath("").
		WithProvider(DefaultValuesProvider[*expressionFunctionsConfig](map[string]any{
			"app": map[string]any{
				"hash": "{{ githash(7) }}",
			},
		})).
		WithSolvers(solvers.NewVariablesSolver("${", "}")).
		WithExpressionFunction("githash", func(args ...any) (any, error) {
			return "f9d293c", nil
		})

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.App.Hash != "f9d293c" {
		t.Fatalf("expected app hash to resolve with fallback expression solver, got %q", cfg.App.Hash)
	}
}

func TestContainerWithExpressionFunction_SecondRegistrationReplacesFirst(t *testing.T) {
	cfg := &expressionFunctionsConfig{}
	container := New(cfg).
		WithConfigPath("").
		WithProvider(DefaultValuesProvider[*expressionFunctionsConfig](map[string]any{
			"app": map[string]any{
				"hash": "{{ githash(7) }}",
			},
		})).
		WithExpressionFunction("githash", func(args ...any) (any, error) {
			return "old-hash", nil
		}).
		WithExpressionFunction("githash", func(args ...any) (any, error) {
			return "f9d293c", nil
		})

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.App.Hash != "f9d293c" {
		t.Fatalf("expected latest function registration to win, got %q", cfg.App.Hash)
	}
}
