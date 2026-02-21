package config

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/goliatone/go-config/cfgx"
)

type managerPipelineConfig struct {
	Name  string `koanf:"name"`
	Alias string `koanf:"alias"`

	validateCalls int
	validateErr   error
}

func (c *managerPipelineConfig) Validate() error {
	c.validateCalls++
	if c.validateErr != nil {
		return c.validateErr
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

func newManagerPipelineContainer(cfg *managerPipelineConfig, defaults map[string]any) *Container[*managerPipelineConfig] {
	return New(cfg).
		WithConfigPath("").
		WithProvider(DefaultValuesProvider[*managerPipelineConfig](defaults))
}

func TestValidationPipelineOrder(t *testing.T) {
	cfg := &managerPipelineConfig{}
	order := []string{}

	container := newManagerPipelineContainer(cfg, map[string]any{
		"name":  "  alice  ",
		"alias": "  alice  ",
	}).
		WithBaseValidate(false).
		WithNormalizer(
			func(c *managerPipelineConfig) error {
				order = append(order, "n1")
				c.Name = strings.TrimSpace(c.Name)
				return nil
			},
			func(c *managerPipelineConfig) error {
				order = append(order, "n2")
				c.Name = strings.ToUpper(c.Name)
				c.Alias = strings.TrimSpace(c.Alias)
				return nil
			},
		).
		WithValidator(
			func(c *managerPipelineConfig) error {
				order = append(order, "v1")
				if c.Name != "ALICE" {
					return fmt.Errorf("expected ALICE, got %q", c.Name)
				}
				return nil
			},
			func(c *managerPipelineConfig) error {
				order = append(order, "v2")
				if c.Alias != "alice" {
					return fmt.Errorf("expected alias alice, got %q", c.Alias)
				}
				return nil
			},
		)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	expectedOrder := []string{"n1", "n2", "v1", "v2"}
	if !reflect.DeepEqual(order, expectedOrder) {
		t.Fatalf("unexpected execution order: got=%v want=%v", order, expectedOrder)
	}
}

func TestBaseValidateToggle(t *testing.T) {
	t.Run("enabled_calls_validate_once", func(t *testing.T) {
		cfg := &managerPipelineConfig{}
		container := newManagerPipelineContainer(cfg, map[string]any{"name": "alice"}).
			WithValidator(func(c *managerPipelineConfig) error { return nil })

		if err := container.Load(context.Background()); err != nil {
			t.Fatalf("load failed: %v", err)
		}
		if cfg.validateCalls != 1 {
			t.Fatalf("expected 1 Validate call, got %d", cfg.validateCalls)
		}
	})

	t.Run("disabled_skips_validate", func(t *testing.T) {
		cfg := &managerPipelineConfig{}
		container := newManagerPipelineContainer(cfg, map[string]any{"name": "alice"}).
			WithBaseValidate(false).
			WithValidator(func(c *managerPipelineConfig) error { return nil })

		if err := container.Load(context.Background()); err != nil {
			t.Fatalf("load failed: %v", err)
		}
		if cfg.validateCalls != 0 {
			t.Fatalf("expected 0 Validate calls, got %d", cfg.validateCalls)
		}
	})
}

func TestValidationNoneBypassesPipeline(t *testing.T) {
	cfg := &managerPipelineConfig{validateErr: errors.New("validate should not run")}
	normalizerCalls := 0
	validatorCalls := 0

	container := newManagerPipelineContainer(cfg, map[string]any{"name": "alice"}).
		WithValidationMode(ValidationNone).
		WithNormalizer(func(c *managerPipelineConfig) error {
			normalizerCalls++
			return fmt.Errorf("normalizer should not run")
		}).
		WithValidator(func(c *managerPipelineConfig) error {
			validatorCalls++
			return fmt.Errorf("validator should not run")
		})

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("expected validation to be bypassed, got: %v", err)
	}
	if normalizerCalls != 0 || validatorCalls != 0 {
		t.Fatalf("expected hooks to be bypassed, normalizers=%d validators=%d", normalizerCalls, validatorCalls)
	}
	if cfg.validateCalls != 0 {
		t.Fatalf("expected Validate not to run, got %d calls", cfg.validateCalls)
	}
}

func TestValidationReportFailFastAndAggregate(t *testing.T) {
	t.Run("fail_fast_reports_first_issue", func(t *testing.T) {
		cfg := &managerPipelineConfig{}
		container := newManagerPipelineContainer(cfg, map[string]any{"name": "alice"}).
			WithBaseValidate(false).
			WithFailFast(true).
			WithValidator(
				func(c *managerPipelineConfig) error { return fmt.Errorf("validator-a failed") },
				func(c *managerPipelineConfig) error { return fmt.Errorf("validator-b failed") },
			)

		err := container.Load(context.Background())
		if err == nil {
			t.Fatalf("expected validation error")
		}

		var report *ValidationReport
		if !errors.As(err, &report) {
			t.Fatalf("expected ValidationReport via errors.As, got %T", err)
		}
		if len(report.Issues) != 1 {
			t.Fatalf("expected 1 issue in fail-fast mode, got %d", len(report.Issues))
		}
		if report.Issues[0].Stage != "validate" {
			t.Fatalf("expected validate stage, got %q", report.Issues[0].Stage)
		}
	})

	t.Run("aggregate_reports_all_issues", func(t *testing.T) {
		cfg := &managerPipelineConfig{}
		container := newManagerPipelineContainer(cfg, map[string]any{"name": "alice"}).
			WithBaseValidate(false).
			WithFailFast(false).
			WithNormalizer(func(c *managerPipelineConfig) error { return fmt.Errorf("normalizer-a failed") }).
			WithValidator(
				func(c *managerPipelineConfig) error { return fmt.Errorf("validator-a failed") },
				func(c *managerPipelineConfig) error { return fmt.Errorf("validator-b failed") },
			)

		err := container.Load(context.Background())
		if err == nil {
			t.Fatalf("expected validation error")
		}

		var report *ValidationReport
		if !errors.As(err, &report) {
			t.Fatalf("expected ValidationReport via errors.As, got %T", err)
		}
		if len(report.Issues) != 3 {
			t.Fatalf("expected 3 aggregated issues, got %d", len(report.Issues))
		}

		stages := []string{report.Issues[0].Stage, report.Issues[1].Stage, report.Issues[2].Stage}
		expectedStages := []string{"normalize", "validate", "validate"}
		if !reflect.DeepEqual(stages, expectedStages) {
			t.Fatalf("unexpected stage sequence: got=%v want=%v", stages, expectedStages)
		}
	})
}

type strictDecodePipelineConfig struct {
	Name string `koanf:"name"`
}

func (c *strictDecodePipelineConfig) Validate() error { return nil }

func TestStrictDecodeOptionAndStageErrorExposure(t *testing.T) {
	t.Run("strict_decode_disabled_allows_unknown_keys", func(t *testing.T) {
		cfg := &strictDecodePipelineConfig{}
		container := New(cfg).
			WithConfigPath("").
			WithProvider(DefaultValuesProvider[*strictDecodePipelineConfig](map[string]any{
				"name":    "alpha",
				"unknown": "value",
			}))

		if err := container.Load(context.Background()); err != nil {
			t.Fatalf("expected decode to ignore unknown keys, got: %v", err)
		}
	})

	t.Run("strict_decode_enabled_surfaces_cfgx_stage_error", func(t *testing.T) {
		cfg := &strictDecodePipelineConfig{}
		container := New(cfg).
			WithConfigPath("").
			WithStrictDecode(true).
			WithProvider(DefaultValuesProvider[*strictDecodePipelineConfig](map[string]any{
				"name":    "alpha",
				"unknown": "value",
			}))

		err := container.Load(context.Background())
		if err == nil {
			t.Fatalf("expected strict decode error")
		}

		var stageErr *cfgx.StageError
		if !errors.As(err, &stageErr) {
			t.Fatalf("expected cfgx.StageError via errors.As, got %T", err)
		}
		if stageErr.Stage != "decode" {
			t.Fatalf("expected decode stage, got %q", stageErr.Stage)
		}
	})
}

func TestValidationLegacyCompatibilityAndPrecedence(t *testing.T) {
	t.Run("legacy_false_disables_validation", func(t *testing.T) {
		cfg := &managerPipelineConfig{validateErr: errors.New("validation should be disabled")}
		container := newManagerPipelineContainer(cfg, map[string]any{"name": "alice"}).
			WithValidation(false)

		if err := container.Load(context.Background()); err != nil {
			t.Fatalf("expected validation disabled, got: %v", err)
		}
		if cfg.validateCalls != 0 {
			t.Fatalf("expected 0 Validate calls, got %d", cfg.validateCalls)
		}
	})

	t.Run("last_call_wins_mode_after_legacy", func(t *testing.T) {
		cfg := &managerPipelineConfig{validateErr: errors.New("expected validation error")}
		container := newManagerPipelineContainer(cfg, map[string]any{"name": "alice"}).
			WithValidation(false).
			WithValidationMode(ValidationSemantic)

		err := container.Load(context.Background())
		if err == nil {
			t.Fatalf("expected validation error")
		}
		if cfg.validateCalls != 1 {
			t.Fatalf("expected 1 Validate call, got %d", cfg.validateCalls)
		}
	})

	t.Run("last_call_wins_legacy_after_mode", func(t *testing.T) {
		cfg := &managerPipelineConfig{validateErr: errors.New("validation should be disabled")}
		container := newManagerPipelineContainer(cfg, map[string]any{"name": "alice"}).
			WithValidationMode(ValidationSemantic).
			WithValidation(false)

		if err := container.Load(context.Background()); err != nil {
			t.Fatalf("expected validation disabled by last call, got: %v", err)
		}
		if cfg.validateCalls != 0 {
			t.Fatalf("expected 0 Validate calls, got %d", cfg.validateCalls)
		}
	})
}

func TestLoadPreservesUnexportedState(t *testing.T) {
	cfg := &managerPipelineConfig{validateErr: errors.New("preserve internal state")}
	container := newManagerPipelineContainer(cfg, map[string]any{"name": "alice"}).
		WithValidationMode(ValidationNone)

	if err := container.Load(context.Background()); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if cfg.validateErr == nil {
		t.Fatalf("expected unexported state to be preserved across load")
	}
}

func TestValidationReportExposedForBaseValidateFailure(t *testing.T) {
	cfg := &managerPipelineConfig{validateErr: errors.New("base validate failed")}
	container := newManagerPipelineContainer(cfg, map[string]any{"name": "alice"}).
		WithFailFast(true)

	err := container.Load(context.Background())
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var report *ValidationReport
	if !errors.As(err, &report) {
		t.Fatalf("expected ValidationReport via errors.As, got %T", err)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(report.Issues))
	}
	if report.Issues[0].Stage != "validate" {
		t.Fatalf("expected validate stage, got %q", report.Issues[0].Stage)
	}
}
