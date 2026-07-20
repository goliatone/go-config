package logger

import (
	"bytes"
	"strings"
	"testing"

	masker "github.com/goliatone/go-masker"
)

func TestDefaultLoggerMasksStructuredSecrets(t *testing.T) {
	var output bytes.Buffer
	log := newDefaultLogger("config", &output)

	log.Debug("configuration loaded", "config", map[string]any{
		"service": map[string]any{
			"api_key":      "api-key-sentinel",
			"access_token": "access-token-sentinel",
			"password":     "password-sentinel",
			"endpoint":     "http://127.0.0.1:8080",
		},
	})

	got := output.String()
	for _, secret := range []string{"api-key-sentinel", "access-token-sentinel", "password-sentinel"} {
		if strings.Contains(got, secret) {
			t.Fatalf("log output leaked %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, masker.RedactedValue) {
		t.Fatalf("log output does not contain redaction marker: %s", got)
	}
	if !strings.Contains(got, "http://127.0.0.1:8080") {
		t.Fatalf("log output omitted non-sensitive configuration: %s", got)
	}
}

func TestMaskSensitiveDoesNotMutateInput(t *testing.T) {
	input := map[string]any{
		"api_key": "api-key-sentinel",
		"name":    "example",
	}

	maskedAny, err := MaskSensitive(input)
	if err != nil {
		t.Fatalf("MaskSensitive: %v", err)
	}
	masked := maskedAny.(map[string]any)
	if masked["api_key"] != masker.RedactedValue {
		t.Fatalf("masked api_key = %v, want %q", masked["api_key"], masker.RedactedValue)
	}
	if input["api_key"] != "api-key-sentinel" {
		t.Fatalf("MaskSensitive mutated input: %#v", input)
	}
}
