package config

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/goliatone/go-config/koanf/solvers"
)

type ownedValue struct {
	Payload   string
	destroyed bool
}

func (v *ownedValue) String() string               { return "sensitive-data" }
func (v *ownedValue) MarshalJSON() ([]byte, error) { return nil, errors.New("sensitive-data") }
func (v *ownedValue) Borrow() string               { return v.Payload }

type ownedReader interface{ Borrow() string }

func ownedPolicy(t reflect.Type) bool { return t == reflect.TypeOf((*ownedValue)(nil)) }

type opaqueConfig struct {
	Token  *ownedValue `koanf:"token"`
	Reader ownedReader `koanf:"reader"`
	Other  string      `koanf:"other"`
	Labels []string    `koanf:"labels"`
}

func (c *opaqueConfig) Validate() error {
	if c.Other == "reject" {
		return errors.New("sensitive-data")
	}
	return nil
}

func TestOpaqueLoadIdentityAndFailureIsolation(t *testing.T) {
	old := &ownedValue{Payload: "  old  "}
	cfg := &opaqueConfig{Token: old, Reader: old, Labels: []string{"prior"}}
	mode := "success"
	var acquired *ownedValue
	candidate := &ownedValue{Payload: "  new  "}
	container := New(cfg).WithConfigPath("").WithOpaqueValues(ownedPolicy).
		WithProvider(DefaultValuesProvider[*opaqueConfig](map[string]any{"other": "  ordinary  "})).
		WithFinalSolver(solvers.FinalSolverFunc(func(ctx context.Context, input map[string]any) error {
			acquired = candidate
			input["token"], input["reader"] = candidate, candidate
			if mode == "decode" {
				input["other"] = candidate
			}
			if mode == "validate" {
				input["other"] = "reject"
			}
			if mode == "terminal" {
				return errors.New("sensitive-data")
			}
			return nil
		})).WithStringTransformer(func(s string) (string, error) {
		if mode == "transform" {
			return s, errors.New("sensitive-data")
		}
		return s, nil
	}).WithNormalizer(func(c *opaqueConfig) error {
		c.Labels[0] = "candidate-" + mode
		if mode == "normalize" {
			return errors.New("sensitive-data")
		}
		return nil
	})
	// This is the adapter's per-load cleanup boundary.
	load := func() error {
		acquired = nil
		err := container.Load(context.Background())
		if err != nil && acquired != nil {
			acquired.destroyed = true
		}
		return err
	}
	seed := cfg
	if err := load(); err != nil {
		t.Fatal(err)
	}
	cfg = container.Raw()
	if cfg == seed || seed.Token != old || seed.Labels[0] != "prior" {
		t.Fatal("successful load modified the prior generation")
	}
	if cfg.Token != candidate || cfg.Reader != candidate || candidate.Payload != "  new  " || cfg.Other != "ordinary" || old.destroyed {
		t.Fatal("opaque identity, transformer exclusion, or ownership mismatch")
	}
	published, priorK := cfg.Token, container.K
	priorLabel := cfg.Labels[0]
	for _, failure := range []string{"decode", "validate", "terminal", "normalize", "transform"} {
		mode = failure
		candidate = &ownedValue{Payload: "  failing  "}
		err := load()
		if err == nil {
			t.Fatalf("expected failed candidate for %s", mode)
		}
		data, _ := json.Marshal(err)
		if strings.Contains(err.Error(), "sensitive-data") || strings.Contains(string(data), "sensitive-data") {
			t.Fatal("unsafe protected error")
		}
		if cfg.Token != published || cfg.Reader != published || cfg.Other != "ordinary" || cfg.Labels[0] != priorLabel || container.K != priorK || published.destroyed || old.destroyed || !candidate.destroyed {
			t.Fatal("failure changed published state or cleanup ownership")
		}
	}
}

func TestOpaqueDefaultsReloadWithoutCopy(t *testing.T) {
	value := &ownedValue{Payload: "  retained  "}
	cfg := &opaqueConfig{Token: value, Reader: value, Labels: []string{"old"}}
	container := New(cfg).WithConfigPath("").WithOpaqueValues(ownedPolicy)
	if err := container.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	seed := cfg
	cfg = container.Raw()
	if cfg == seed || seed.Token != value || seed.Labels[0] != "old" {
		t.Fatal("default publication changed the seed generation")
	}
	if cfg.Token != value || cfg.Reader != value || value.Payload != "  retained  " {
		t.Fatal("default identity lost")
	}
	k := container.K
	container.WithValidator(func(c *opaqueConfig) error { c.Labels[0] = "changed"; return errors.New("sensitive-data") })
	err := container.Load(context.Background())
	if err == nil || cfg.Token != value || cfg.Labels[0] != "old" || container.K != k || strings.Contains(err.Error(), "sensitive-data") {
		t.Fatal("default failure mutated prior state")
	}
}
