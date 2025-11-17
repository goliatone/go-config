package cfgx

import (
	"errors"
	"testing"
)

type funcStruct struct {
	Name  string
	Count int
	Nest  struct {
		Value any
	}
}

func TestPreprocessEvalFuncs_Map(t *testing.T) {
	input := map[string]any{
		"name": func() string { return "dynamic" },
		"count": func() (int, error) {
			return 42, nil
		},
		"nested": map[string]any{
			"value": func() int { return 7 },
		},
	}

	result, err := PreprocessEvalFuncs()(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := result.(map[string]any)
	if output["name"] != "dynamic" || output["count"] != 42 {
		t.Fatalf("unexpected output: %#v", output)
	}
	nested := output["nested"].(map[string]any)
	if nested["value"] != 7 {
		t.Fatalf("expected nested value 7, got %#v", nested["value"])
	}
}

func TestPreprocessEvalFuncs_Struct(t *testing.T) {
	input := struct {
		Name  func() string `mapstructure:"name"`
		Count func() int    `mapstructure:"count"`
	}{
		Name:  func() string { return "struct" },
		Count: func() int { return 10 },
	}
	result, err := PreprocessEvalFuncs()(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := result.(map[string]any)
	if output["name"] != "struct" || output["count"] != 10 {
		t.Fatalf("unexpected output: %#v", output)
	}
}

func TestPreprocessEvalFuncs_Error(t *testing.T) {
	input := map[string]any{
		"value": func() (int, error) {
			return 0, errors.New("boom")
		},
	}
	_, err := PreprocessEvalFuncs()(input)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPreprocessEvalFuncs_Panic(t *testing.T) {
	input := map[string]any{
		"value": func() any {
			panic("nope")
		},
	}
	_, err := PreprocessEvalFuncs()(input)
	if err == nil {
		t.Fatal("expected panic error")
	}
}

func TestPreprocessMerge(t *testing.T) {
	base := map[string]any{"name": "base", "nested": map[string]any{"count": 1}}
	overlay := struct {
		Count int `mapstructure:"count"`
	}{Count: 99}

	result, err := PreprocessMerge(map[string]any{"count": 2}, overlay)(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := result.(map[string]any)
	if output["name"] != "base" || output["count"] != 99 {
		t.Fatalf("unexpected output: %#v", output)
	}
	nested := output["nested"].(map[string]any)
	if nested["count"] != 1 {
		t.Fatalf("nested map should be preserved, got %#v", nested)
	}
}

func TestPreprocessMergeError(t *testing.T) {
	base := map[string]any{}
	_, err := PreprocessMerge(map[int]any{1: "bad"})(base)
	if err == nil {
		t.Fatal("expected error for non-string key")
	}
}

func TestWithMerge(t *testing.T) {
	type Config struct {
		Name  string `mapstructure:"name"`
		Count int    `mapstructure:"count"`
	}
	cfg, err := Build[Config](map[string]any{"name": "base"}, WithMerge[Config](map[string]any{"count": 5}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "base" || cfg.Count != 5 {
		t.Fatalf("unexpected cfg: %#v", cfg)
	}
}

func TestWithPreprocessEvalFuncs(t *testing.T) {
	type Config struct {
		Name string `mapstructure:"name"`
	}
	input := struct {
		Name func() string `mapstructure:"name"`
	}{
		Name: func() string { return "dynamic" },
	}
	cfg, err := Build[Config](input, WithPreprocessEvalFuncs[Config]())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "dynamic" {
		t.Fatalf("expected dynamic, got %s", cfg.Name)
	}
}
