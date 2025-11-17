package cfgx_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-config/cfgx"
	"github.com/goliatone/go-config/config"
)

func init() {
	cfgx.RegisterOptionalBoolType(config.NewOptionalBoolUnset())
}

func TestOptionalBoolHookPointer(t *testing.T) {
	type Config struct {
		Flag *config.OptionalBool `mapstructure:"flag"`
	}

	cfg, err := cfgx.Build[Config](map[string]any{"flag": "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Flag == nil || !cfg.Flag.IsSet() || !cfg.Flag.Value() {
		t.Fatalf("expected optional bool set to true, got %#v", cfg.Flag)
	}
}

func TestOptionalBoolHookValue(t *testing.T) {
	type Config struct {
		Flag config.OptionalBool `mapstructure:"flag"`
	}
	cfg, err := cfgx.Build[Config](map[string]any{"flag": map[string]any{"value": "false", "set": true}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Flag.IsSet() || cfg.Flag.Value() {
		t.Fatalf("expected optional bool false, got %#v", cfg.Flag)
	}
}

func TestDurationHook(t *testing.T) {
	type Config struct {
		Timeout time.Duration `mapstructure:"timeout"`
	}

	cfg, err := cfgx.Build[Config](map[string]any{"timeout": "3s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timeout != 3*time.Second {
		t.Fatalf("expected 3s, got %v", cfg.Timeout)
	}

	_, err = cfgx.Build[Config](map[string]any{"timeout": "3s"}, cfgx.WithoutDefaultHooks[Config]())
	if err == nil {
		t.Fatal("expected decode error without duration hook")
	}
}

type upperString string

func (u *upperString) UnmarshalText(text []byte) error {
	*u = upperString(strings.ToUpper(string(text)))
	return nil
}

func (u upperString) MarshalText() ([]byte, error) {
	return []byte(string(u)), nil
}

func TestTextUnmarshalerHook(t *testing.T) {
	type Config struct {
		Value upperString `mapstructure:"value"`
	}

	cfg, err := cfgx.Build[Config](map[string]any{"value": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Value != upperString("HELLO") {
		t.Fatalf("expected HELLO, got %s", cfg.Value)
	}
}

func ExampleRegisterOptionalBoolType() {
	cfgx.RegisterOptionalBoolType(config.NewOptionalBoolUnset())
	type Config struct {
		Flag *config.OptionalBool `mapstructure:"flag"`
	}
	cfg, err := cfgx.Build[Config](map[string]any{"flag": true})
	if err != nil {
		panic(err)
	}
	fmt.Println(cfg.Flag.Value())
	// Output: true
}
