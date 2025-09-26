package config

import (
	"encoding"
	"reflect"
	"strings"

	"github.com/go-viper/mapstructure/v2"
)

var (
	optionalBoolType    = reflect.TypeOf(OptionalBool{})
	optionalBoolPtrType = reflect.TypeOf(&OptionalBool{})
)

// optionalBoolDecodeHook normalises data destined for OptionalBool fields so providers can
// supply plain booleans, strings, or structured maps while preserving set/unset semantics.
func optionalBoolDecodeHook() mapstructure.DecodeHookFunc {
	return func(from reflect.Type, to reflect.Type, data any) (any, error) {
		if !isOptionalBoolTarget(to) {
			return data, nil
		}

		if data == nil {
			if to == optionalBoolPtrType {
				return (*OptionalBool)(nil), nil
			}
			return OptionalBool{}, nil
		}

		switch v := data.(type) {
		case *OptionalBool:
			if v == nil {
				if to == optionalBoolPtrType {
					return (*OptionalBool)(nil), nil
				}
				return OptionalBool{}, nil
			}
			if to == optionalBoolPtrType {
				return cloneOptionalBool(v), nil
			}
			cloned := cloneOptionalBool(v)
			if cloned == nil {
				return OptionalBool{}, nil
			}
			return *cloned, nil
		case OptionalBool:
			if to == optionalBoolPtrType {
				clone := &OptionalBool{}
				if v.IsSet() {
					clone.Set(v.Value())
				}
				return clone, nil
			}
			return v, nil
		case bool:
			if to == optionalBoolPtrType {
				return NewOptionalBool(v), nil
			}
			ob := OptionalBool{}
			ob.Set(v)
			return ob, nil
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed == "" || strings.EqualFold(trimmed, "null") {
				if to == optionalBoolPtrType {
					return NewOptionalBoolUnset(), nil
				}
				return OptionalBool{}, nil
			}
			parsed, err := parseBoolString(trimmed)
			if err != nil {
				return nil, err
			}
			if to == optionalBoolPtrType {
				return NewOptionalBool(parsed), nil
			}
			ob := OptionalBool{}
			ob.Set(parsed)
			return ob, nil
		case map[string]any:
			ob := &OptionalBool{}
			handled, err := mergeOptionalBoolFromMap(ob, v)
			if err != nil {
				return nil, err
			}
			if !handled {
				return data, nil
			}
			if to == optionalBoolPtrType {
				return ob, nil
			}
			return *ob, nil
		default:
			return data, nil
		}
	}
}

// OptionalBoolDecodeHookForTest exposes the OptionalBool decode hook for external callers (tests/debugging).
func isOptionalBoolTarget(t reflect.Type) bool {
	return t == optionalBoolType || t == optionalBoolPtrType
}

// textUnmarshalerDecodeHook mirrors koanf's internal helper so we can compose hooks while
// still supporting custom encoding.Text(Un)Marshaler implementations.
func textUnmarshalerDecodeHook() mapstructure.DecodeHookFuncType {
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
