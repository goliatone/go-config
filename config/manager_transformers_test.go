package config

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type transformerQueueConfig struct {
	Backend string `koanf:"backend"`
	Name    string `koanf:"name"`
}

type transformerNestedConfig struct {
	Value string `koanf:"value"`
}

type transformerTestConfig struct {
	Name    string                   `koanf:"name"`
	Alias   string                   `koanf:"alias"`
	Tags    []string                 `koanf:"tags"`
	PtrTags *[]string                `koanf:"ptr_tags"`
	Queue   transformerQueueConfig   `koanf:"queue"`
	Nested  *transformerNestedConfig `koanf:"nested"`

	validateCalls int
	validateErr   error
}

func (c *transformerTestConfig) Validate() error {
	c.validateCalls++
	if c.validateErr != nil {
		return c.validateErr
	}
	return nil
}

func newTransformerContainer(cfg *transformerTestConfig, defaults map[string]any) *Container[*transformerTestConfig] {
	return New(cfg).
		WithConfigPath("").
		WithProvider(DefaultValuesProvider[*transformerTestConfig](defaults))
}

func TestDefaultTrimSpaceTransformerAppliesToStringsAndSlices(t *testing.T) {
	ptrTags := []string{"  one  ", " two "}
	cfg := &transformerTestConfig{
		PtrTags: &ptrTags,
		Nested:  &transformerNestedConfig{Value: "  nested  "},
	}

	container := newTransformerContainer(cfg, map[string]any{
		"name": "  alice  ",
		"tags": []string{"  red ", " blue  "},
		"queue": map[string]any{
			"backend": " redis ",
		},
	}).
		WithBaseValidate(false)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.Name != "alice" {
		t.Fatalf("expected trimmed name, got %q", cfg.Name)
	}
	if !reflect.DeepEqual(cfg.Tags, []string{"red", "blue"}) {
		t.Fatalf("expected trimmed tags, got %#v", cfg.Tags)
	}
	if cfg.PtrTags == nil || !reflect.DeepEqual(*cfg.PtrTags, []string{"one", "two"}) {
		t.Fatalf("expected trimmed ptr tags, got %#v", cfg.PtrTags)
	}
	if cfg.Nested == nil || cfg.Nested.Value != "nested" {
		t.Fatalf("expected trimmed nested value, got %#v", cfg.Nested)
	}
	if cfg.Queue.Backend != "redis" {
		t.Fatalf("expected trimmed queue backend, got %q", cfg.Queue.Backend)
	}
}

func TestKeySpecificTransformerUsesExactPath(t *testing.T) {
	cfg := &transformerTestConfig{}
	container := newTransformerContainer(cfg, map[string]any{
		"queue": map[string]any{
			"backend": "REDIS",
			"name":    "jobs",
		},
	}).
		WithBaseValidate(false).
		WithDefaultTransformers(false).
		WithStringTransformerForKey("queue", ToLower).
		WithStringTransformerForKey("queue.backend", ToLower)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.Queue.Backend != "redis" {
		t.Fatalf("expected exact key match to transform backend, got %q", cfg.Queue.Backend)
	}
	if cfg.Queue.Name != "jobs" {
		t.Fatalf("expected queue.name unchanged, got %q", cfg.Queue.Name)
	}
}

func TestTransformerOrderingGlobalThenKeySpecific(t *testing.T) {
	cfg := &transformerTestConfig{}
	calls := []string{}

	container := newTransformerContainer(cfg, map[string]any{
		"name": "name-seed",
		"queue": map[string]any{
			"backend": "backend-seed",
		},
	}).
		WithBaseValidate(false).
		WithDefaultTransformers(false).
		WithStringTransformer(
			func(v string) (string, error) {
				calls = append(calls, "g1:"+v)
				return v + "-g1", nil
			},
			func(v string) (string, error) {
				calls = append(calls, "g2:"+v)
				return v + "-g2", nil
			},
		).
		WithStringTransformerForKey("queue.backend",
			func(v string) (string, error) {
				calls = append(calls, "k1:"+v)
				return v + "-k1", nil
			},
			func(v string) (string, error) {
				calls = append(calls, "k2:"+v)
				return v + "-k2", nil
			},
		)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.Name != "name-seed-g1-g2" {
		t.Fatalf("expected global transformers on name, got %q", cfg.Name)
	}
	if cfg.Queue.Backend != "backend-seed-g1-g2-k1-k2" {
		t.Fatalf("expected global then key-specific order, got %q", cfg.Queue.Backend)
	}

	wantSubsequence := []string{
		"g1:backend-seed",
		"g2:backend-seed-g1",
		"k1:backend-seed-g1-g2",
		"k2:backend-seed-g1-g2-k1",
	}
	searchIndex := 0
	for _, call := range calls {
		if searchIndex >= len(wantSubsequence) {
			break
		}
		if call == wantSubsequence[searchIndex] {
			searchIndex++
		}
	}
	if searchIndex != len(wantSubsequence) {
		t.Fatalf("missing expected queue.backend transformer order, got calls=%v", calls)
	}
}

func TestTransformerErrorsSurfaceInValidationReport(t *testing.T) {
	cfg := &transformerTestConfig{}
	container := newTransformerContainer(cfg, map[string]any{
		"queue": map[string]any{
			"backend": "redis",
		},
	}).
		WithBaseValidate(false).
		WithDefaultTransformers(false).
		WithFailFast(true).
		WithStringTransformerForKey("queue.backend", func(v string) (string, error) {
			return v, fmt.Errorf("backend transform failed")
		})

	err := container.Load(context.Background())
	if err == nil {
		t.Fatalf("expected transformer error")
	}

	var report *ValidationReport
	if !errors.As(err, &report) {
		t.Fatalf("expected ValidationReport, got %T", err)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(report.Issues))
	}
	issue := report.Issues[0]
	if issue.Stage != "transform" {
		t.Fatalf("expected transform stage, got %q", issue.Stage)
	}
	if issue.Path != "queue.backend" {
		t.Fatalf("expected queue.backend path, got %q", issue.Path)
	}
	if issue.Code != "transformer_error" {
		t.Fatalf("expected transformer_error code, got %q", issue.Code)
	}
}

func TestTransformerPanicUsesPanicCode(t *testing.T) {
	cfg := &transformerTestConfig{}
	container := newTransformerContainer(cfg, map[string]any{
		"name": "alice",
	}).
		WithBaseValidate(false).
		WithDefaultTransformers(false).
		WithFailFast(true).
		WithStringTransformer(func(v string) (string, error) {
			panic("boom")
		})

	err := container.Load(context.Background())
	if err == nil {
		t.Fatalf("expected transformer panic error")
	}

	var report *ValidationReport
	if !errors.As(err, &report) {
		t.Fatalf("expected ValidationReport, got %T", err)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(report.Issues))
	}
	if report.Issues[0].Code != "transformer_panic" {
		t.Fatalf("expected transformer_panic code, got %q", report.Issues[0].Code)
	}
	if report.Issues[0].Stage != "transform" {
		t.Fatalf("expected transform stage, got %q", report.Issues[0].Stage)
	}
}

func TestTransformerFailFastAndAggregateAcrossStages(t *testing.T) {
	t.Run("fail_fast_stops_after_transform_issue", func(t *testing.T) {
		cfg := &transformerTestConfig{}
		normalizerCalls := 0
		validatorCalls := 0

		container := newTransformerContainer(cfg, map[string]any{
			"queue": map[string]any{
				"backend": "redis",
			},
		}).
			WithBaseValidate(false).
			WithDefaultTransformers(false).
			WithFailFast(true).
			WithStringTransformerForKey("queue.backend", func(v string) (string, error) {
				return v, fmt.Errorf("transform failed")
			}).
			WithNormalizer(func(c *transformerTestConfig) error {
				normalizerCalls++
				return fmt.Errorf("normalizer failed")
			}).
			WithValidator(func(c *transformerTestConfig) error {
				validatorCalls++
				return fmt.Errorf("validator failed")
			})

		err := container.Load(context.Background())
		if err == nil {
			t.Fatalf("expected validation error")
		}

		var report *ValidationReport
		if !errors.As(err, &report) {
			t.Fatalf("expected ValidationReport, got %T", err)
		}
		if len(report.Issues) != 1 || report.Issues[0].Stage != "transform" {
			t.Fatalf("expected single transform issue, got %#v", report.Issues)
		}
		if normalizerCalls != 0 || validatorCalls != 0 {
			t.Fatalf("expected downstream stages skipped, normalizer=%d validator=%d", normalizerCalls, validatorCalls)
		}
	})

	t.Run("aggregate_collects_transform_normalize_validate", func(t *testing.T) {
		cfg := &transformerTestConfig{}
		container := newTransformerContainer(cfg, map[string]any{
			"queue": map[string]any{
				"backend": "redis",
			},
		}).
			WithBaseValidate(false).
			WithDefaultTransformers(false).
			WithFailFast(false).
			WithStringTransformerForKey("queue.backend", func(v string) (string, error) {
				return v, fmt.Errorf("transform failed")
			}).
			WithNormalizer(func(c *transformerTestConfig) error {
				return fmt.Errorf("normalizer failed")
			}).
			WithValidator(func(c *transformerTestConfig) error {
				return fmt.Errorf("validator failed")
			})

		err := container.Load(context.Background())
		if err == nil {
			t.Fatalf("expected validation error")
		}

		var report *ValidationReport
		if !errors.As(err, &report) {
			t.Fatalf("expected ValidationReport, got %T", err)
		}
		if len(report.Issues) != 3 {
			t.Fatalf("expected 3 issues, got %d", len(report.Issues))
		}
		stages := []string{report.Issues[0].Stage, report.Issues[1].Stage, report.Issues[2].Stage}
		if !reflect.DeepEqual(stages, []string{"transform", "normalize", "validate"}) {
			t.Fatalf("unexpected stages: got=%v", stages)
		}
	})
}

func TestValidationNoneRunsTransformersButSkipsSemanticStages(t *testing.T) {
	cfg := &transformerTestConfig{validateErr: errors.New("validate should not run")}
	normalizerCalls := 0
	validatorCalls := 0

	container := newTransformerContainer(cfg, map[string]any{
		"queue": map[string]any{
			"backend": " REDIS ",
		},
	}).
		WithValidationMode(ValidationNone).
		WithDefaultTransformers(false).
		WithStringTransformer(TrimSpace).
		WithStringTransformerForKey("queue.backend", ToLower).
		WithNormalizer(func(c *transformerTestConfig) error {
			normalizerCalls++
			return fmt.Errorf("normalizer should not run")
		}).
		WithValidator(func(c *transformerTestConfig) error {
			validatorCalls++
			return fmt.Errorf("validator should not run")
		})

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("expected ValidationNone to skip semantic stages, got: %v", err)
	}
	if cfg.Queue.Backend != "redis" {
		t.Fatalf("expected transformers to run in ValidationNone mode, got %q", cfg.Queue.Backend)
	}
	if normalizerCalls != 0 || validatorCalls != 0 {
		t.Fatalf("expected semantic hooks skipped, normalizer=%d validator=%d", normalizerCalls, validatorCalls)
	}
	if cfg.validateCalls != 0 {
		t.Fatalf("expected base Validate skipped, got %d", cfg.validateCalls)
	}
}

type TransformerEmbeddedFlat struct {
	Flat string `koanf:"flat"`
}

type TransformerEmbeddedTagged struct {
	Scoped string `koanf:"scoped"`
}

type transformerPathConfig struct {
	TransformerEmbeddedFlat
	TransformerEmbeddedTagged `koanf:"embed"`
	Ignored                   string `koanf:"-"`
	Fallback                  string
}

func (c *transformerPathConfig) Validate() error { return nil }

func TestTransformersResolveKoanfPaths(t *testing.T) {
	cfg := &transformerPathConfig{
		TransformerEmbeddedFlat: TransformerEmbeddedFlat{Flat: "flat"},
		TransformerEmbeddedTagged: TransformerEmbeddedTagged{
			Scoped: "scoped",
		},
		Ignored:  "ignored",
		Fallback: "fallback",
	}

	container := New(cfg).
		WithConfigPath("").
		WithDefaultTransformers(false).
		WithBaseValidate(false).
		WithStringTransformerForKey("flat", ToUpper).
		WithStringTransformerForKey("embed.scoped", ToUpper).
		WithStringTransformerForKey("Fallback", ToUpper).
		WithStringTransformerForKey("Ignored", ToUpper)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.Flat != "FLAT" {
		t.Fatalf("expected flattened embedded path transform, got %q", cfg.Flat)
	}
	if cfg.Scoped != "SCOPED" {
		t.Fatalf("expected explicitly tagged embedded path transform, got %q", cfg.Scoped)
	}
	if cfg.Fallback != "FALLBACK" {
		t.Fatalf("expected fallback field-name path transform, got %q", cfg.Fallback)
	}
	if cfg.Ignored != "ignored" {
		t.Fatalf("expected koanf:\"-\" field ignored, got %q", cfg.Ignored)
	}
}
