package config

import "strings"

// MergeWithBooleanPrecedence merges src into dst with OptionalBool-aware precedence logic.
// OptionalBool values keep their pointer/value form and only overwrite when explicitly set.
func MergeWithBooleanPrecedence(src, dst map[string]any) error {
	return mergeRecursive(src, dst)
}

func mergeRecursive(src, dst map[string]any) error {
	for key, srcVal := range src {
		dstVal, exists := dst[key]

		if srcMap, ok := srcVal.(map[string]any); ok {
			if dstMap, ok := dstVal.(map[string]any); ok {
				if err := mergeRecursive(srcMap, dstMap); err != nil {
					return err
				}
				continue
			}
		}

		handled, err := mergeOptionalBoolValue(dst, key, srcVal, dstVal)
		if err != nil {
			return err
		}
		if handled {
			continue
		}

		if shouldOverwriteValue(srcVal, exists) {
			dst[key] = srcVal
		}
	}
	return nil
}

func shouldOverwriteValue(src any, dstExists bool) bool {
	if src == nil {
		return false
	}

	if !dstExists {
		return true
	}

	switch v := src.(type) {
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return false
	default:
		return true
	}
}

func mergeOptionalBoolValue(dst map[string]any, key string, srcVal, dstVal any) (bool, error) {
	switch val := srcVal.(type) {
	case nil:
		return true, nil
	case *OptionalBool:
		if val == nil || !val.IsSet() {
			return true, nil
		}
		dst[key] = cloneOptionalBool(val)
		return true, nil
	case OptionalBool:
		if !val.IsSet() {
			return true, nil
		}
		dst[key] = val
		return true, nil
	case bool:
		if isOptionalBoolValue(dstVal) {
			if _, isValue := dstVal.(OptionalBool); isValue {
				ob := OptionalBool{}
				ob.Set(val)
				dst[key] = ob
			} else {
				dst[key] = NewOptionalBool(val)
			}
			return true, nil
		}
		return false, nil
	case string:
		if !isOptionalBoolValue(dstVal) {
			return false, nil
		}
		trimmed := strings.TrimSpace(val)
		if trimmed == "" || strings.EqualFold(trimmed, "null") {
			return true, nil
		}
		parsed, err := parseBoolString(trimmed)
		if err != nil {
			return false, nil
		}
		dst[key] = NewOptionalBool(parsed)
		return true, nil
	case map[string]any:
		ob := &OptionalBool{}
		handled, err := mergeOptionalBoolFromMap(ob, val)
		if err != nil {
			return true, err
		}
		if !handled {
			return false, nil
		}
		if !ob.IsSet() {
			return true, nil
		}
		dst[key] = ob
		return true, nil
	default:
		if _, ok := dstVal.(*OptionalBool); ok {
			return true, nil
		}
		if _, ok := dstVal.(OptionalBool); ok {
			return true, nil
		}
		return false, nil
	}
}

func mergeOptionalBoolFromMap(dst *OptionalBool, payload map[string]any) (bool, error) {
	if dst == nil {
		return false, nil
	}

	var handled bool

	if setRaw, ok := payload["set"]; ok {
		handled = true
		if setBool, ok := setRaw.(bool); ok && !setBool {
			dst.Unset()
			return true, nil
		}
	}

	if valRaw, ok := payload["value"]; ok {
		handled = true
		switch v := valRaw.(type) {
		case bool:
			dst.Set(v)
			return true, nil
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed == "" || strings.EqualFold(trimmed, "null") {
				dst.Unset()
				return true, nil
			}
			parsed, err := parseBoolString(trimmed)
			if err != nil {
				return true, err
			}
			dst.Set(parsed)
			return true, nil
		case *OptionalBool:
			if v != nil && v.IsSet() {
				dst.Set(v.Value())
			} else {
				dst.Unset()
			}
			return true, nil
		case OptionalBool:
			if v.IsSet() {
				dst.Set(v.Value())
			} else {
				dst.Unset()
			}
			return true, nil
		}
	}

	return handled, nil
}

func isOptionalBoolValue(val any) bool {
	switch val.(type) {
	case *OptionalBool, OptionalBool:
		return true
	default:
		return false
	}
}
