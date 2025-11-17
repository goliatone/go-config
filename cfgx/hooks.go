package cfgx

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/go-viper/mapstructure/v2"
)

// OptionalBool exposes the behavior required by cfgx to manipulate optional boolean values.
// Types that satisfy this interface (e.g., config.OptionalBool) can be registered via
// RegisterOptionalBoolType so OptionalBoolHook can operate without importing the concrete type.
type OptionalBool interface {
	Set(bool)
	Unset()
	IsSet() bool
	Value() bool
}

type optionalBoolRegistration struct {
	mu          sync.RWMutex
	valueType   reflect.Type
	pointerType reflect.Type
}

var optBool optionalBoolRegistration

// RegisterOptionalBoolType informs cfgx about an OptionalBool implementation. The provided sample
// should be a pointer to the concrete type so cfgx can instantiate new instances as needed.
func RegisterOptionalBoolType(sample OptionalBool) {
	if sample == nil {
		panic("cfgx: nil sample provided to RegisterOptionalBoolType")
	}
	ptrType := reflect.TypeOf(sample)
	if ptrType.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("cfgx: RegisterOptionalBoolType expects pointer type, got %s", ptrType))
	}
	optBool.mu.Lock()
	defer optBool.mu.Unlock()
	optBool.pointerType = ptrType
	optBool.valueType = ptrType.Elem()
}

func (r *optionalBoolRegistration) registered() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.pointerType != nil && r.valueType != nil
}

func (r *optionalBoolRegistration) pointerTypeOf() reflect.Type {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.pointerType
}

func (r *optionalBoolRegistration) valueTypeOf() reflect.Type {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.valueType
}

func (r *optionalBoolRegistration) newPointer() OptionalBool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.pointerType == nil {
		return nil
	}
	ptr := reflect.New(r.valueType)
	ob, _ := ptr.Interface().(OptionalBool)
	return ob
}

// DefaultDecodeHooks returns the standard hook set (optional bool, duration, text unmarshaler).
func DefaultDecodeHooks() []mapstructure.DecodeHookFunc {
	return []mapstructure.DecodeHookFunc{
		OptionalBoolHook(),
		DurationHook(),
		TextUnmarshalerHook(),
	}
}

// OptionalBoolHook normalises data destined for registered OptionalBool fields while preserving
// set/unset semantics.
func OptionalBoolHook() mapstructure.DecodeHookFunc {
	return func(from reflect.Type, to reflect.Type, data any) (any, error) {
		if !optBool.registered() || !isOptionalBoolTarget(to) {
			return data, nil
		}

		if data == nil {
			if to == optBool.pointerTypeOf() {
				return optionalBoolNilPointer(), nil
			}
			return optionalBoolZeroValue(), nil
		}

		switch v := data.(type) {
		case OptionalBool:
			return optionalBoolFromPointer(v, to), nil
		case bool:
			return optionalBoolFromBool(v, to), nil
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed == "" || strings.EqualFold(trimmed, "null") {
				if to == optBool.pointerTypeOf() {
					return optionalBoolNilPointer(), nil
				}
				return optionalBoolZeroValue(), nil
			}
			parsed, err := parseBoolString(trimmed)
			if err != nil {
				return nil, err
			}
			return optionalBoolFromBool(parsed, to), nil
		case map[string]any:
			return optionalBoolFromMap(v, to)
		default:
			if reflect.TypeOf(data) == optBool.valueTypeOf() {
				ptr := optBool.newPointer()
				if ptr == nil {
					return data, nil
				}
				val := reflect.ValueOf(data)
				reflect.ValueOf(ptr).Elem().Set(val)
				return optionalBoolFromPointer(ptr, to), nil
			}
			return data, nil
		}
	}
}

func optionalBoolFromPointer(ptr OptionalBool, target reflect.Type) any {
	if ptr == nil {
		if target == optBool.pointerTypeOf() {
			return optionalBoolNilPointer()
		}
		return optionalBoolZeroValue()
	}
	cloned := cloneOptionalBool(ptr)
	if target == optBool.pointerTypeOf() {
		return cloned
	}
	return reflect.ValueOf(cloned).Elem().Interface()
}

func optionalBoolFromBool(value bool, target reflect.Type) any {
	ptr := optBool.newPointer()
	if ptr == nil {
		return nil
	}
	ptr.Set(value)
	if target == optBool.pointerTypeOf() {
		return ptr
	}
	return reflect.ValueOf(ptr).Elem().Interface()
}

func optionalBoolFromMap(data map[string]any, target reflect.Type) (any, error) {
	ptr := optBool.newPointer()
	if ptr == nil {
		return data, nil
	}
	if handled, err := mergeOptionalBoolFromMap(ptr, data); err != nil {
		return nil, err
	} else if !handled {
		return data, nil
	}
	if target == optBool.pointerTypeOf() {
		return ptr, nil
	}
	return reflect.ValueOf(ptr).Elem().Interface(), nil
}

func isOptionalBoolTarget(t reflect.Type) bool {
	if t == nil {
		return false
	}
	return t == optBool.pointerTypeOf() || t == optBool.valueTypeOf()
}

func optionalBoolNilPointer() any {
	return reflect.Zero(optBool.pointerTypeOf()).Interface()
}

func optionalBoolZeroValue() any {
	ptr := optBool.newPointer()
	if ptr == nil {
		return nil
	}
	return reflect.ValueOf(ptr).Elem().Interface()
}

func cloneOptionalBool(ptr OptionalBool) OptionalBool {
	if ptr == nil {
		return nil
	}
	clone := optBool.newPointer()
	if clone == nil {
		return nil
	}
	if ptr.IsSet() {
		clone.Set(ptr.Value())
	}
	return clone
}

func mergeOptionalBoolFromMap(ptr OptionalBool, data map[string]any) (bool, error) {
	if ptr == nil {
		return false, fmt.Errorf("cfgx: nil OptionalBool target")
	}
	var handled bool
	if raw, ok := data["value"]; ok {
		switch v := raw.(type) {
		case bool:
			ptr.Set(v)
			handled = true
		case string:
			parsed, err := parseBoolString(v)
			if err != nil {
				return false, err
			}
			ptr.Set(parsed)
			handled = true
		}
	}
	if raw, ok := data["set"]; ok {
		if set, ok := raw.(bool); ok && !set {
			ptr.Unset()
			handled = true
		}
	}
	return handled, nil
}

func parseBoolString(val string) (bool, error) {
	val = strings.TrimSpace(strings.ToLower(val))
	switch val {
	case "1", "t", "true", "y", "yes", "on":
		return true, nil
	case "0", "f", "false", "n", "no", "off":
		return false, nil
	default:
		return strconv.ParseBool(val)
	}
}

// DurationHook converts strings (e.g., "5s") into time.Duration.
func DurationHook() mapstructure.DecodeHookFunc {
	return mapstructure.StringToTimeDurationHookFunc()
}

// TextUnmarshalerHook mirrors koanf's helper allowing encoding.TextUnmarshaler targets.
func TextUnmarshalerHook() mapstructure.DecodeHookFunc {
	return func(from reflect.Type, to reflect.Type, data any) (any, error) {
		if from.Kind() != reflect.String {
			return data, nil
		}
		result := reflect.New(to).Interface()
		unmarshaller, ok := result.(encoding.TextUnmarshaler)
		if !ok {
			return data, nil
		}

		dataVal := reflect.ValueOf(data)
		text := []byte(dataVal.String())
		if from.Kind() == to.Kind() {
			ptrVal := reflect.New(dataVal.Type())
			if ptrVal.Elem().CanSet() {
				ptrVal.Elem().Set(dataVal)
			}
			for _, candidate := range []reflect.Value{dataVal, ptrVal} {
				if marshaller, ok := candidate.Interface().(encoding.TextMarshaler); ok {
					marshaled, err := marshaller.MarshalText()
					if err != nil {
						return nil, err
					}
					text = marshaled
					break
				}
			}
		}

		if err := unmarshaller.UnmarshalText(text); err != nil {
			return nil, err
		}
		return result, nil
	}
}
