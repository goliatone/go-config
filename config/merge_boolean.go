package config

import "reflect"

// MergeWithBooleanPrecedence merges src into dst with OptionalBool-aware precedence logic.
// This function respects OptionalBool.IsSet() metadata for proper precedence decisions,
// while maintaining backward compatibility with existing merge behavior for non-OptionalBool types.
func MergeWithBooleanPrecedence(src, dst map[string]any) error {
	return mergeRecursive(src, dst)
}

// mergeRecursive performs the recursive merge operation
func mergeRecursive(src, dst map[string]any) error {
	for key, srcVal := range src {
		dstVal, exists := dst[key]

		// Check if both source and destination are maps - if so, always recurse
		if srcMap, ok := srcVal.(map[string]any); ok {
			if dstMap, ok := dstVal.(map[string]any); ok {
				// Recursive merge for maps
				if err := mergeRecursive(srcMap, dstMap); err != nil {
					return err
				}
				continue
			}
		}

		// For non-map values, determine if we should overwrite based on type-specific logic
		shouldOverwrite := shouldOverwriteValue(dstVal, srcVal, exists)
		if shouldOverwrite {
			dst[key] = srcVal
		}
	}
	return nil
}

// shouldOverwriteValue determines if the source value should overwrite the destination value
func shouldOverwriteValue(dst, src any, dstExists bool) bool {
	// Handle nil source values
	if src == nil {
		return false
	}

	// Check if source is OptionalBool pointer (most common case)
	if srcOB, ok := src.(*OptionalBool); ok {
		// Handle nil OptionalBool pointer
		if srcOB == nil {
			return false
		}
		// OptionalBool source: only overwrite if explicitly set
		return srcOB.IsSet()
	}

	// Check if source is OptionalBool by value (for interface{} containers)
	if srcVal := reflect.ValueOf(src); srcVal.IsValid() && srcVal.Type() == reflect.TypeOf(OptionalBool{}) {
		srcOB := srcVal.Interface().(OptionalBool)
		return srcOB.IsSet()
	}

	// Non-OptionalBool types: use existing merge logic
	return shouldOverwriteExisting(dst, src, dstExists)
}

// shouldOverwriteExisting implements the existing merge logic from MergeIgnoringNullValues
func shouldOverwriteExisting(_, src any, dstExists bool) bool {
	// If destination doesn't exist, always overwrite
	if !dstExists {
		return true
	}

	// Apply existing merge rules based on source type
	switch srcVal := src.(type) {
	case string:
		return srcVal != ""
	case []any:
		return len(srcVal) > 0
	case map[string]any:
		// Maps are handled recursively, not overwritten
		return false
	default:
		// All other types (including regular bool) always overwrite
		return true
	}
}
