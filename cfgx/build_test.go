package cfgx

import (
	"errors"
	"fmt"
	"testing"
)

type sampleConfig struct {
	Name  string `mapstructure:"name"`
	Count int    `mapstructure:"count"`
}

func TestBuildNoOptions(t *testing.T) {
	runTestCases(t, []testCase{
		{
			name: "value target",
			run: func(t *testing.T) {
				input := map[string]any{
					"name":  "alpha",
					"count": 3,
				}
				cfg, err := Build[sampleConfig](input)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg.Name != "alpha" || cfg.Count != 3 {
					t.Fatalf("unexpected result: %#v", cfg)
				}
			},
		},
		{
			name: "pointer target",
			run: func(t *testing.T) {
				input := map[string]any{
					"name":  "beta",
					"count": 9,
				}
				cfg, err := Build[*sampleConfig](input)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cfg == nil {
					t.Fatalf("expected non-nil pointer result")
				}
				if cfg.Name != "beta" || cfg.Count != 9 {
					t.Fatalf("unexpected result: %#v", cfg)
				}
			},
		},
	})
}

func TestBuildDefaultsAndPreprocessors(t *testing.T) {
	defaultOpt := func(b *builder[sampleConfig]) {
		b.defaults = func() (sampleConfig, error) {
			return sampleConfig{Name: "default-name", Count: 1}, nil
		}
	}
	preOpt := func(b *builder[sampleConfig]) {
		b.preprocessors = append(b.preprocessors, func(in any) (any, error) {
			data, _ := in.(map[string]any)
			if data == nil {
				data = map[string]any{}
			}
			data["count"] = 42
			return data, nil
		})
	}

	cfg, err := Build[sampleConfig](map[string]any{}, defaultOpt, preOpt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "default-name" || cfg.Count != 42 {
		t.Fatalf("unexpected cfg: %#v", cfg)
	}
}

func TestBuildPreprocessorError(t *testing.T) {
	preOpt := func(b *builder[sampleConfig]) {
		b.preprocessors = append(b.preprocessors, func(any) (any, error) {
			return nil, errors.New("boom")
		})
	}

	_, err := Build[sampleConfig](map[string]any{}, preOpt)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrPreprocess) {
		t.Fatalf("expected ErrPreprocess, got %v", err)
	}
	var stageErr *StageError
	if !errors.As(err, &stageErr) {
		t.Fatalf("expected StageError, got %T", err)
	}
	if stageErr.Meta["preprocessor_index"] != 0 {
		t.Fatalf("expected preprocessor_index metadata, got %+v", stageErr.Meta)
	}
}

func TestBuildDefaultsError(t *testing.T) {
	defaultOpt := func(b *builder[sampleConfig]) {
		b.defaults = func() (sampleConfig, error) {
			return sampleConfig{}, errors.New("boom")
		}
	}

	_, err := Build[sampleConfig](map[string]any{}, defaultOpt)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrDefaults) {
		t.Fatalf("expected ErrDefaults, got %v", err)
	}
}

func TestBuildDecodeError(t *testing.T) {
	_, err := Build[sampleConfig]([]string{"bad"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrDecode) {
		t.Fatalf("expected ErrDecode, got %v", err)
	}
}

func TestBuildValidatorError(t *testing.T) {
	validateOpt := func(b *builder[sampleConfig]) {
		b.validator = func(cfg *sampleConfig) error {
			if cfg.Count == 0 {
				return errors.New("count required")
			}
			return nil
		}
	}

	_, err := Build[sampleConfig](map[string]any{}, validateOpt)
	if err == nil {
		t.Fatal("expected validator error")
	}
	if !errors.Is(err, ErrValidate) {
		t.Fatalf("expected ErrValidate, got %v", err)
	}
}

func ExampleBuild_minimal() {
	type Config struct {
		Addr string `mapstructure:"addr"`
		Port int    `mapstructure:"port"`
	}

	cfg, err := Build[Config](map[string]any{
		"addr": "localhost",
		"port": 8080,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s:%d\n", cfg.Addr, cfg.Port)
	// Output: localhost:8080
}
