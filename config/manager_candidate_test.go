package config

import (
	"context"
	"errors"
	"testing"

	"github.com/goliatone/go-config/cfgx"
	"github.com/goliatone/go-config/koanf/solvers"
)

type candidateConfig struct {
	Name        string `koanf:"name"`
	denied      bool
	validations int
}

func (c *candidateConfig) Validate() error {
	c.validations++
	if c.denied {
		return errors.New("CONFIDENTIAL")
	}
	return nil
}

func TestStagedCandidateValidationAndPublication(t *testing.T) {
	for _, terminalOnly := range []bool{false, true} {
		for _, denied := range []bool{false, true} {
			seed := &candidateConfig{Name: "prior", denied: denied}
			container := New(seed).WithConfigPath("").WithProvider(DefaultValuesProvider[*candidateConfig](map[string]any{"name": "new"}))
			if terminalOnly {
				container.WithFinalSolver(solvers.FinalSolverFunc(func(context.Context, map[string]any) error { return nil }))
			} else {
				container.WithOpaqueValues(ownedPolicy)
			}
			priorK := container.K
			err := container.Load(context.Background())
			if (err != nil) != denied {
				t.Fatalf("private-state validation mismatch: terminal=%v denied=%v err=%v", terminalOnly, denied, err)
			}
			if seed.Name != "prior" || seed.validations != 0 {
				t.Fatal("candidate validation mutated seed state")
			}
			if denied {
				if container.Raw() != seed || container.K != priorK {
					t.Fatal("failed candidate was published")
				}
			} else {
				current := container.Raw()
				if current == seed || current.Name != "new" || current.denied || current.validations != 1 {
					t.Fatal("publication did not retain the exact validated candidate")
				}
				if err := container.Load(context.Background()); err != nil {
					t.Fatal(err)
				}
				if current.Name != "new" || current.validations != 1 || current == container.Raw() {
					t.Fatal("reload modified a previously published generation")
				}
			}
		}
	}
}

type privateCandidateConfig struct {
	Name   string `koanf:"name"`
	denied *bool
}

func (c *privateCandidateConfig) Validate() error {
	if c.denied != nil && *c.denied {
		return errors.New("CONFIDENTIAL")
	}
	return nil
}
func TestStagedPrivateReferenceCloner(t *testing.T) {
	denied := false
	seed := &privateCandidateConfig{Name: "prior", denied: &denied}
	container := New(seed).WithConfigPath("").WithOpaqueValues(ownedPolicy)
	prior := container.K
	if err := container.Load(context.Background()); !errors.Is(err, cfgx.ErrDefaultsClonerRequired) {
		t.Fatalf("private references were not rejected safely: %v", err)
	}
	if container.Raw() != seed || container.K != prior {
		t.Fatal("rejected defaults published state")
	}
	container.WithDefaultsCloner(func(source *privateCandidateConfig) (*privateCandidateConfig, error) { return source, nil })
	if err := container.Load(context.Background()); !errors.Is(err, cfgx.ErrDefaultsClonerRequired) {
		t.Fatal("cloner reused the live root")
	}
	container.WithDefaultsCloner(func(source *privateCandidateConfig) (*privateCandidateConfig, error) {
		next := *source
		if source.denied != nil {
			flag := *source.denied
			next.denied = &flag
		}
		return &next, nil
	}).WithNormalizer(func(candidate *privateCandidateConfig) error {
		*candidate.denied = true
		candidate.Name = "new"
		return nil
	})
	if err := container.Load(context.Background()); err == nil {
		t.Fatal("isolated private validation bypassed")
	}
	if denied || seed.Name != "prior" || container.Raw() != seed || container.K != prior {
		t.Fatal("custom candidate clone mutated prior private state")
	}
}

func TestStagedOpaqueNormalizerDestination(t *testing.T) {
	seed := &normalizerOpaqueConfig{}
	container := New(seed).WithConfigPath("").WithOpaqueValues(ownedPolicy).
		WithNormalizer(func(c *normalizerOpaqueConfig) error { c.Value = &ownedValue{}; return nil })
	if err := container.Load(context.Background()); !errors.Is(err, cfgx.ErrOpaqueDestination) {
		t.Fatalf("normalizer bypassed destination checks: %v", err)
	}
	if container.Raw() != seed || seed.Value != nil {
		t.Fatal("unsupported normalizer candidate published")
	}
}

type normalizerOpaqueConfig struct{ Value any }

func (*normalizerOpaqueConfig) Validate() error { return nil }
