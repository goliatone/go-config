package cfgx

import (
	"fmt"

	"github.com/go-viper/mapstructure/v2"
)

// Option allows callers to tweak builder behavior before decoding a config struct.
type Option[T any] func(*builder[T])

// Validator represents the validation hook invoked after decoding completes.
type Validator[T any] func(*T) error

// WithDefaults seeds the builder with a default config value that will be cloned
// before decoding. Later calls override earlier defaults.
func WithDefaults[T any](value T) Option[T] {
	return func(b *builder[T]) {
		b.defaults = func() (T, error) {
			return value, nil
		}
	}
}

// WithDefaultFunc allows defaults to be generated lazily. The provided function
// should return a fully configured instance ready for decoding overlays.
func WithDefaultFunc[T any](fn func() (T, error)) Option[T] {
	return func(b *builder[T]) {
		b.defaults = fn
	}
}

// WithPreprocess registers one or more preprocessors to run sequentially before decode.
func WithPreprocess[T any](pre ...Preprocessor) Option[T] {
	return func(b *builder[T]) {
		b.preprocessors = append(b.preprocessors, pre...)
	}
}

// WithDecoder lets callers mutate the underlying mapstructure DecoderConfig.
func WithDecoder[T any](fn func(*mapstructure.DecoderConfig)) Option[T] {
	return func(b *builder[T]) {
		if fn == nil {
			return
		}
		fn(&b.decoderConfig)
	}
}

// WithDecodeHooks appends custom decode hooks onto the builder.
func WithDecodeHooks[T any](hooks ...mapstructure.DecodeHookFunc) Option[T] {
	return func(b *builder[T]) {
		for _, hook := range hooks {
			if hook == nil {
				continue
			}
			b.decodeHooks = append(b.decodeHooks, hook)
		}
	}
}

// WithStrictKeys enables mapstructure's strict unused-key detection and zero-field enforcement.
func WithStrictKeys[T any]() Option[T] {
	return func(b *builder[T]) {
		b.decoderConfig.ErrorUnused = true
		b.decoderConfig.ZeroFields = true
	}
}

// WithWeakTyping toggles WeaklyTypedInput behavior.
func WithWeakTyping[T any](enabled bool) Option[T] {
	return func(b *builder[T]) {
		b.decoderConfig.WeaklyTypedInput = enabled
	}
}

// WithTagName overrides the struct tag key mapstructure uses while decoding.
func WithTagName[T any](tag string) Option[T] {
	return func(b *builder[T]) {
		if tag == "" {
			return
		}
		b.decoderConfig.TagName = tag
	}
}

// WithValidator registers a validator function invoked after decoding. Only one validator is allowed.
func WithValidator[T any](validator Validator[T]) Option[T] {
	return func(b *builder[T]) {
		if validator == nil {
			return
		}
		if b.validator != nil {
			b.setOptionError("validator already registered")
			return
		}
		b.validator = validator
	}
}

// WithValidatorFunc adapts a value-based validator into the pointer-based contract.
func WithValidatorFunc[T any](validator func(T) error) Option[T] {
	if validator == nil {
		return func(*builder[T]) {}
	}
	return WithValidator(func(cfg *T) error {
		if cfg == nil {
			var zero T
			return validator(zero)
		}
		return validator(*cfg)
	})
}

// WithoutDefaultHooks disables automatic inclusion of default decode hooks.
func WithoutDefaultHooks[T any]() Option[T] {
	return func(b *builder[T]) {
		b.useHookSet = false
	}
}

// WithDefaultHooks forces default hooks back on (useful when another option disabled them earlier).
func WithDefaultHooks[T any]() Option[T] {
	return func(b *builder[T]) {
		b.useHookSet = true
	}
}

// WithPreprocessFunc is a convenience for registering inline preprocessors.
func WithPreprocessFunc[T any](fn func(any) (any, error)) Option[T] {
	if fn == nil {
		return func(*builder[T]) {}
	}
	return WithPreprocess[T](Preprocessor(fn))
}

// WithDecoderConfig copies the supplied DecoderConfig as the baseline for the builder.
func WithDecoderConfig[T any](conf mapstructure.DecoderConfig) Option[T] {
	return func(b *builder[T]) {
		b.decoderConfig = conf
	}
}

// WithOptionError allows external helpers to surface option misconfiguration errors.
func WithOptionError[T any](err error) Option[T] {
	return func(b *builder[T]) {
		if err == nil {
			return
		}
		if b.optionErr == nil {
			b.optionErr = fmt.Errorf("%w: %w", ErrOption, err)
		}
	}
}

// WithPreprocessEvalFuncs appends the default EvalFunc preprocessor to the builder.
func WithPreprocessEvalFuncs[T any]() Option[T] {
	return WithPreprocess[T](PreprocessEvalFuncs())
}

// WithMerge merges the provided sources into the input map before decoding.
func WithMerge[T any](sources ...any) Option[T] {
	return WithPreprocess[T](PreprocessMerge(sources...))
}
