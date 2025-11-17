package cfgx

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-viper/mapstructure/v2"
)

// Preprocessor functions transform raw input before decoding begins.
type Preprocessor func(any) (any, error)

// PreprocessEvalFuncs walks maps, structs, slices, and zero-argument function values, replacing
// function fields with their return values. Struct outputs are converted into map[string]any to
// align with mapstructure decoding.
func PreprocessEvalFuncs() Preprocessor {
	return func(input any) (any, error) {
		return evalFuncFields(input)
	}
}

// PreprocessMerge merges the provided sources into the existing input map. Sources can be maps or
// structs; later sources override earlier ones.
func PreprocessMerge(sources ...any) Preprocessor {
	return func(input any) (any, error) {
		base, err := toMap(input)
		if err != nil {
			return nil, err
		}
		for idx, src := range sources {
			if err := mergeInto(base, src); err != nil {
				return nil, fmt.Errorf("cfgx: merge source %d: %w", idx, err)
			}
		}
		return base, nil
	}
}

func evalFuncFields(input any) (any, error) {
	if input == nil {
		return nil, nil
	}
	val := reflect.ValueOf(input)
	switch val.Kind() {
	case reflect.Map:
		return evalMap(val)
	case reflect.Struct:
		return evalStruct(val)
	case reflect.Slice, reflect.Array:
		return evalSlice(val)
	case reflect.Pointer, reflect.Interface:
		if val.IsNil() {
			return nil, nil
		}
		return evalFuncFields(val.Elem().Interface())
	case reflect.Func:
		return callFunc(val)
	default:
		return input, nil
	}
}

func evalMap(val reflect.Value) (any, error) {
	result := make(map[string]any, val.Len())
	iter := val.MapRange()
	for iter.Next() {
		key := iter.Key()
		strKey, ok := key.Interface().(string)
		if !ok {
			return nil, fmt.Errorf("cfgx: expected string map key, got %T", key.Interface())
		}
		value := iter.Value().Interface()
		evaluated, err := evalFuncFields(value)
		if err != nil {
			return nil, err
		}
		result[strKey] = evaluated
	}
	return result, nil
}

func evalStruct(val reflect.Value) (any, error) {
	result := make(map[string]any, val.NumField())
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue // unexported
		}
		key := field.Tag.Get("mapstructure")
		if key == "" {
			key = field.Tag.Get("json")
		}
		if key == "" {
			key = strings.TrimSpace(field.Name)
		}
		if key == "-" {
			continue
		}
		value := val.Field(i).Interface()
		evaluated, err := evalFuncFields(value)
		if err != nil {
			return nil, err
		}
		result[key] = evaluated
	}
	return result, nil
}

func evalSlice(val reflect.Value) (any, error) {
	length := val.Len()
	result := make([]any, length)
	for i := 0; i < length; i++ {
		evaluated, err := evalFuncFields(val.Index(i).Interface())
		if err != nil {
			return nil, err
		}
		result[i] = evaluated
	}
	return result, nil
}

func callFunc(val reflect.Value) (any, error) {
	if val.Type().NumIn() != 0 || val.Type().NumOut() == 0 {
		return val.Interface(), nil
	}
	var (
		result any
		err    error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("cfgx: eval func panic: %v", r)
			}
		}()
		outputs := val.Call(nil)
		switch len(outputs) {
		case 1:
			result = outputs[0].Interface()
		case 2:
			if e, ok := outputs[1].Interface().(error); ok && e != nil {
				err = e
				return
			}
			result = outputs[0].Interface()
		default:
			result = val.Interface()
		}
	}()
	return result, err
}

func toMap(input any) (map[string]any, error) {
	if input == nil {
		return map[string]any{}, nil
	}
	switch v := input.(type) {
	case map[string]any:
		return cloneMap(v), nil
	default:
		val := reflect.ValueOf(input)
		if val.Kind() == reflect.Map {
			result := map[string]any{}
			iter := val.MapRange()
			for iter.Next() {
				key := iter.Key()
				strKey, ok := key.Interface().(string)
				if !ok {
					return nil, fmt.Errorf("cfgx: cannot convert map key %T to string", key.Interface())
				}
				evaluated, err := evalFuncFields(iter.Value().Interface())
				if err != nil {
					return nil, err
				}
				result[strKey] = evaluated
			}
			return result, nil
		}
		result := map[string]any{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			TagName:          "mapstructure",
			Result:           &result,
			WeaklyTypedInput: true,
		})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(input); err != nil {
			return nil, err
		}
		return result, nil
	}
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		if nested, ok := v.(map[string]any); ok {
			dst[k] = cloneMap(nested)
			continue
		}
		dst[k] = v
	}
	return dst
}

func mergeInto(dst map[string]any, src any) error {
	if src == nil {
		return nil
	}
	srcMap, err := toMap(src)
	if err != nil {
		return err
	}
	return mergeMaps(dst, srcMap)
}

func mergeMaps(dst, src map[string]any) error {
	for key, value := range src {
		if existing, ok := dst[key]; ok {
			existingMap, okExisting := existing.(map[string]any)
			incomingMap, okIncoming := value.(map[string]any)
			if okExisting && okIncoming {
				if err := mergeMaps(existingMap, incomingMap); err != nil {
					return err
				}
				continue
			}
		}
		dst[key] = value
	}
	return nil
}
