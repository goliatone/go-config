package cfgx

import (
	"errors"
	"reflect"
	"strings"

	"github.com/go-viper/mapstructure/v2"
)

// OpaquePolicy identifies caller-owned pointer types that must not be inspected.
type OpaquePolicy func(reflect.Type) bool

var ErrOpaqueDestination = errors.New("cfgx: unsupported opaque destination")

func WithOpaqueValues[T any](policy OpaquePolicy) Option[T] {
	return func(b *builder[T]) { b.opaquePolicy = policy }
}

func opaqueDecodeHook(policy OpaquePolicy, ordinary mapstructure.DecodeHookFunc, config mapstructure.DecoderConfig) mapstructure.DecodeHookFunc {
	return mapstructure.DecodeHookFuncValue(func(from, to reflect.Value) (any, error) {
		if policy(from.Type()) {
			if from.Kind() != reflect.Pointer || !to.CanSet() || !from.Type().AssignableTo(to.Type()) {
				return nil, ErrOpaqueDestination
			}
			if to.Kind() != reflect.Pointer && (to.Kind() != reflect.Interface || to.Type().NumMethod() == 0) {
				return nil, ErrOpaqueDestination
			}
			to.Set(from)
			// Returning nil bypasses mapstructure's pointer allocation and nested decode.
			return nil, nil
		}
		if policy(to.Type()) {
			return nil, ErrOpaqueDestination
		}
		clearOpaqueOverrides(from, to, policy, config)
		if ordinary == nil {
			return from.Interface(), nil
		}
		return mapstructure.DecodeHookExec(ordinary, from, to)
	})
}

// SafeBuildError retains only known stage and ownership sentinels.
func SafeBuildError(err error) error {
	if err == nil {
		return nil
	}
	var stage *StageError
	if errors.As(err, &stage) {
		var base error
		switch stage.Stage {
		case stageDefaults:
			base = ErrDefaults
		case stageDecode:
			base = ErrDecode
		case stageValidate:
			base = ErrValidate
		case stagePreprocess:
			base = ErrPreprocess
		default:
			return ErrOption
		}
		cause := base
		if errors.Is(err, ErrDefaultsClonerRequired) {
			cause = errors.Join(cause, ErrDefaultsClonerRequired)
		}
		if errors.Is(err, ErrOpaqueDestination) {
			cause = errors.Join(cause, ErrOpaqueDestination)
		}
		return stageError(stage.Stage, base, cause, nil)
	}
	return ErrOption
}

func clearOpaqueOverrides(from, to reflect.Value, policy OpaquePolicy, config mapstructure.DecoderConfig) {
	if from.Kind() != reflect.Map || to.Kind() != reflect.Struct {
		return
	}
	match := config.MatchName
	if match == nil {
		match = strings.EqualFold
	}
	for i := 0; i < to.NumField(); i++ {
		field, slot := to.Type().Field(i), to.Field(i)
		if field.PkgPath != "" || !slot.CanSet() {
			continue
		}
		tags := strings.Split(field.Tag.Get(config.TagName), ",")
		name := tags[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		squash := config.Squash && field.Anonymous
		for _, tag := range tags[1:] {
			if tag == "squash" || (config.SquashTagOption != "" && tag == config.SquashTagOption) {
				squash = true
			}
		}
		if squash && !policy(slot.Type()) {
			nested := slot
			if nested.Kind() == reflect.Pointer && !nested.IsNil() {
				nested = nested.Elem()
			}
			clearOpaqueOverrides(from, nested, policy, config)
			continue
		}
		if slot.Kind() != reflect.Pointer || !policy(slot.Type()) {
			continue
		}
		for _, key := range from.MapKeys() {
			raw := key
			if raw.Kind() == reflect.Interface {
				raw = raw.Elem()
			}
			if raw.Kind() == reflect.String && match(raw.String(), name) {
				// mapstructure dereferences populated struct pointers before invoking hooks.
				slot.Set(reflect.Zero(slot.Type()))
				break
			}
		}
	}
}
