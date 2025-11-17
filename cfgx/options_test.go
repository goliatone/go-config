package cfgx

import (
	"errors"
	"reflect"
	"testing"

	"github.com/go-viper/mapstructure/v2"
)

func TestWithDefaults(t *testing.T) {
	cfg, err := Build[sampleConfig](nil, WithDefaults(sampleConfig{Name: "d"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "d" {
		t.Fatalf("expected defaults applied, got %#v", cfg)
	}
}

func TestWithDefaultFuncError(t *testing.T) {
	opt := WithDefaultFunc[sampleConfig](func() (sampleConfig, error) {
		return sampleConfig{}, errors.New("boom")
	})

	_, err := Build[sampleConfig](nil, opt)
	if err == nil || !errors.Is(err, ErrDefaults) {
		t.Fatalf("expected ErrDefaults, got %v", err)
	}
}

func TestWithPreprocessOrder(t *testing.T) {
	var order []int
	first := func(any) (any, error) {
		order = append(order, 1)
		return map[string]any{"name": "first"}, nil
	}
	second := func(any) (any, error) {
		order = append(order, 2)
		return map[string]any{"name": "second"}, nil
	}

	_, err := Build[sampleConfig](map[string]any{}, WithPreprocess[sampleConfig](first, second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("unexpected order: %#v", order)
	}
}

func TestWithDecoderMutator(t *testing.T) {
	b := newBuilder[sampleConfig](nil)
	WithDecoder[sampleConfig](func(conf *mapstructure.DecoderConfig) {
		conf.TagName = "foo"
		conf.ErrorUnused = true
	})(b)
	if b.decoderConfig.TagName != "foo" || !b.decoderConfig.ErrorUnused {
		t.Fatalf("decoder config not mutated: %#v", b.decoderConfig)
	}
}

func TestWithDecodeHooks(t *testing.T) {
	b := newBuilder[sampleConfig](nil)
	hook := func(reflect.Type, reflect.Type, any) (any, error) { return nil, nil }
	WithDecodeHooks[sampleConfig](hook)(b)
	if len(b.decodeHooks) != 1 {
		t.Fatalf("expected decode hook appended")
	}
}

func TestWithStrictKeys(t *testing.T) {
	b := newBuilder[sampleConfig](nil)
	WithStrictKeys[sampleConfig]()(b)
	if !b.decoderConfig.ErrorUnused || !b.decoderConfig.ZeroFields {
		t.Fatalf("strict keys not set")
	}
}

func TestWithWeakTyping(t *testing.T) {
	b := newBuilder[sampleConfig](nil)
	WithWeakTyping[sampleConfig](false)(b)
	if b.decoderConfig.WeaklyTypedInput {
		t.Fatalf("expected weak typing disabled")
	}
}

func TestWithTagName(t *testing.T) {
	b := newBuilder[sampleConfig](nil)
	WithTagName[sampleConfig]("custom")(b)
	if b.decoderConfig.TagName != "custom" {
		t.Fatalf("expected custom tag")
	}
}

func TestWithValidatorDuplicate(t *testing.T) {
	_, err := Build[sampleConfig](map[string]any{},
		WithValidator(func(*sampleConfig) error { return nil }),
		WithValidator(func(*sampleConfig) error { return nil }),
	)
	if err == nil || !errors.Is(err, ErrOption) {
		t.Fatalf("expected option error, got %v", err)
	}
}

func TestWithValidatorFunc(t *testing.T) {
	_, err := Build[sampleConfig](map[string]any{},
		WithValidatorFunc(func(cfg sampleConfig) error {
			if cfg.Name != "" {
				return errors.New("should be zero value")
			}
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithoutDefaultHooks(t *testing.T) {
	b := newBuilder[sampleConfig](nil)
	WithoutDefaultHooks[sampleConfig]()(b)
	if b.useHookSet {
		t.Fatalf("expected hook set disabled")
	}
	WithDefaultHooks[sampleConfig]()(b)
	if !b.useHookSet {
		t.Fatalf("expected hook set enabled")
	}
}

func TestWithPreprocessFunc(t *testing.T) {
	var called bool
	opt := WithPreprocessFunc[sampleConfig](func(any) (any, error) {
		called = true
		return map[string]any{}, nil
	})
	_, err := Build[sampleConfig](map[string]any{}, opt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected preprocessor to run")
	}
}

func TestWithDecoderConfig(t *testing.T) {
	conf := mapstructure.DecoderConfig{
		TagName:          "foo",
		ZeroFields:       true,
		WeaklyTypedInput: false,
	}
	b := newBuilder[sampleConfig](nil)
	WithDecoderConfig[sampleConfig](conf)(b)
	if b.decoderConfig.TagName != "foo" || !b.decoderConfig.ZeroFields || b.decoderConfig.WeaklyTypedInput {
		t.Fatalf("decoder config not applied: %#v", b.decoderConfig)
	}
}

func TestWithOptionError(t *testing.T) {
	_, err := Build[sampleConfig](map[string]any{}, WithOptionError[sampleConfig](errors.New("boom")))
	if err == nil || !errors.Is(err, ErrOption) {
		t.Fatalf("expected ErrOption, got %v", err)
	}
}
