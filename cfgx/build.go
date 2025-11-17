package cfgx

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/go-viper/mapstructure/v2"
	"github.com/mitchellh/copystructure"
)

const (
	stageDefaults   = "defaults"
	stagePreprocess = "preprocess"
	stageDecode     = "decode"
	stageValidate   = "validate"
)

var (
	// ErrDefaults wraps failures when generating or cloning default config instances.
	ErrDefaults = errors.New("cfgx: defaults stage failed")
	// ErrPreprocess wraps failures while executing preprocessors before decoding.
	ErrPreprocess = errors.New("cfgx: preprocess stage failed")
	// ErrDecode wraps mapstructure decode failures.
	ErrDecode = errors.New("cfgx: decode stage failed")
	// ErrValidate wraps validator-reported errors.
	ErrValidate = errors.New("cfgx: validate stage failed")
	// ErrOption indicates a misconfigured builder option (e.g., duplicate validator).
	ErrOption = errors.New("cfgx: option configuration failed")
)

// StageError describes a failure in a specific build stage along with contextual metadata.
type StageError struct {
	Stage string
	Base  error
	Err   error
	Meta  map[string]any
}

// Error implements the error interface.
func (e *StageError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", e.Stage, e.Err)
}

// Unwrap allows errors.Is/As to inspect the underlying error.
func (e *StageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Is reports whether the target matches either the stage sentinel or wrapped error.
func (e *StageError) Is(target error) bool {
	if e == nil {
		return target == nil
	}
	if errors.Is(e.Base, target) {
		return true
	}
	return errors.Is(e.Err, target)
}

func stageError(stage string, base, err error, meta map[string]any) error {
	if err == nil {
		return nil
	}
	return &StageError{
		Stage: stage,
		Base:  base,
		Err:   err,
		Meta:  meta,
	}
}

// builder holds Build state and user-supplied options.
type builder[T any] struct {
	input         any
	defaults      func() (T, error)
	preprocessors []Preprocessor
	decodeHooks   []mapstructure.DecodeHookFunc
	decoderConfig mapstructure.DecoderConfig
	validator     Validator[T]
	useHookSet    bool
	optionErr     error
}

func newBuilder[T any](input any) *builder[T] {
	return &builder[T]{
		input: input,
		decoderConfig: mapstructure.DecoderConfig{
			TagName:          "mapstructure",
			WeaklyTypedInput: true,
		},
		useHookSet: true,
	}
}

// Build orchestrates defaults, preprocessors, decode hooks, mapstructure decode, and validation.
// When any stage fails, the returned error wraps one of the ErrDefaults/ErrPreprocess/ErrDecode/ErrValidate
// sentinels so callers can branch via errors.Is while still accessing StageError metadata via errors.As.
func Build[T any](input any, opts ...Option[T]) (T, error) {
	b := newBuilder[T](input)
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(b)
	}
	if b.optionErr != nil {
		var zero T
		return zero, b.optionErr
	}
	return b.build()
}

func (b *builder[T]) setOptionError(format string, args ...any) {
	if b.optionErr != nil {
		return
	}
	err := fmt.Errorf(format, args...)
	b.optionErr = fmt.Errorf("%w: %w", ErrOption, err)
}

func (b *builder[T]) build() (T, error) {
	result, err := b.applyDefaults()
	if err != nil {
		var zero T
		return zero, err
	}

	currentInput, err := b.applyPreprocessors(b.input)
	if err != nil {
		var zero T
		return zero, err
	}

	if currentInput == nil {
		currentInput = b.input
	}

	if err := b.decode(currentInput, &result); err != nil {
		var zero T
		return zero, err
	}

	if err := b.runValidator(&result); err != nil {
		var zero T
		return zero, err
	}

	return result, nil
}

func (b *builder[T]) applyDefaults() (T, error) {
	if b.defaults == nil {
		var zero T
		return zero, nil
	}
	val, err := b.defaults()
	if err != nil {
		var zero T
		return zero, stageError(stageDefaults, ErrDefaults, err, nil)
	}
	cloned, err := cloneValue(val)
	if err != nil {
		var zero T
		return zero, stageError(stageDefaults, ErrDefaults, err, map[string]any{
			"reason": "clone",
		})
	}
	return cloned, nil
}

func (b *builder[T]) applyPreprocessors(input any) (any, error) {
	current := input
	for idx, pre := range b.preprocessors {
		if pre == nil {
			continue
		}
		next, err := pre(current)
		if err != nil {
			return nil, stageError(stagePreprocess, ErrPreprocess, err, map[string]any{
				"preprocessor_index": idx,
			})
		}
		current = next
	}
	return current, nil
}

func (b *builder[T]) decode(input any, result *T) error {
	target := prepareDecodeTarget(result)

	config := b.decoderConfig
	config.Result = target
	config.DecodeHook = b.composeDecodeHooks()
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return stageError(stageDecode, ErrDecode, err, map[string]any{"reason": "decoder_config"})
	}
	if err := decoder.Decode(input); err != nil {
		return stageError(stageDecode, ErrDecode, err, nil)
	}
	return nil
}

func (b *builder[T]) composeDecodeHooks() mapstructure.DecodeHookFunc {
	hooks := make([]mapstructure.DecodeHookFunc, 0, len(b.decodeHooks)+3)
	if b.useHookSet {
		hooks = append(hooks, DefaultDecodeHooks()...)
	}
	hooks = append(hooks, b.decodeHooks...)
	if len(hooks) == 0 {
		return nil
	}
	if len(hooks) == 1 {
		return hooks[0]
	}
	return mapstructure.ComposeDecodeHookFunc(hooks...)
}

func prepareDecodeTarget[T any](result *T) any {
	val := reflect.ValueOf(result).Elem()
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		return val.Interface()
	}
	return val.Addr().Interface()
}

func (b *builder[T]) runValidator(result *T) error {
	if b.validator == nil {
		return nil
	}
	if err := b.validator(result); err != nil {
		return stageError(stageValidate, ErrValidate, err, nil)
	}
	return nil
}

func cloneValue[T any](value T) (T, error) {
	var zero T
	cloned, err := copystructure.Copy(value)
	if err != nil {
		return zero, err
	}
	casted, ok := cloned.(T)
	if !ok {
		return zero, fmt.Errorf("cfgx: failed to cast cloned value %T to target type", cloned)
	}
	return casted, nil
}
