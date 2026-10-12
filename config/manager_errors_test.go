package config

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	gerrors "github.com/goliatone/go-errors"
	"github.com/knadh/koanf/v2"
)

type diagnosticProvider struct{ validation, load error }

func (diagnosticProvider) Type() ProviderType                         { return ProviderType("CONFIDENTIAL") }
func (diagnosticProvider) Priority() int                              { return 0 }
func (p diagnosticProvider) Validate() error                          { return p.validation }
func (p diagnosticProvider) Load(context.Context, *koanf.Koanf) error { return p.load }

func TestProtectedProviderDiagnostics(t *testing.T) {
	for _, phase := range []struct{ stage, code string }{
		{"factory", "PROVIDER_CREATION_FAILED"}, {"validate", "INVALID_PROVIDER_TYPE"}, {"load", "CONFIG_LOAD_FAILED"},
	} {
		for _, canceled := range []bool{false, true} {
			t.Run(phase.stage+map[bool]string{false: "", true: "_canceled"}[canceled], func(t *testing.T) {
				cause := gerrors.New("CONFIDENTIAL", gerrors.CategoryOperation).WithMetadata(map[string]any{"reference": "CONFIDENTIAL"})
				if canceled {
					cause.Source = context.Canceled
				}
				seed := &terminalConfig{Value: "prior"}
				container := New(seed).WithConfigPath("").WithOpaqueValues(ownedPolicy).
					WithProvider(func(*Container[*terminalConfig]) (Provider, error) {
						if phase.stage == "factory" {
							return nil, cause
						}
						if phase.stage == "validate" {
							return diagnosticProvider{validation: cause}, nil
						}
						return diagnosticProvider{load: cause}, nil
					})
				prior := container.K
				err := container.Load(context.Background())
				var structured *gerrors.Error
				if !errors.As(err, &structured) || structured.TextCode != phase.code {
					t.Fatalf("wrong diagnostic classification: %v", err)
				}
				encoded, marshalErr := json.Marshal(err)
				if marshalErr != nil || strings.Contains(err.Error(), "CONFIDENTIAL") || strings.Contains(string(encoded), "CONFIDENTIAL") || errors.Is(err, cause) {
					t.Fatal("provider data retained by protected diagnostics")
				}
				if errors.Is(err, context.Canceled) != canceled {
					t.Fatal("cancellation classification lost")
				}
				if container.K != prior || seed.Value != "prior" || container.Raw() != seed {
					t.Fatal("provider failure published state")
				}
			})
		}
	}
}

func TestLegacyProviderDiagnostics(t *testing.T) {
	cause := errors.New("legacy diagnostic")
	err := New(&terminalConfig{}).WithConfigPath("").WithProvider(func(*Container[*terminalConfig]) (Provider, error) { return diagnosticProvider{load: cause}, nil }).Load(context.Background())
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "legacy diagnostic") {
		t.Fatal("legacy provider diagnostics changed")
	}
}
