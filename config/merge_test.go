package config

import (
	"testing"
)

// TestMergeIgnoringNullValues_BooleanHandling demonstrates current boolean merge behavior
func TestMergeIgnoringNullValues_BooleanHandling(t *testing.T) {
	tests := []struct {
		name     string
		src      map[string]any
		dest     map[string]any
		expected map[string]any
		problem  string
	}{
		{
			name:     "boolean_true_overwrites_false",
			src:      map[string]any{"enabled": true},
			dest:     map[string]any{"enabled": false},
			expected: map[string]any{"enabled": true},
			problem:  "This works correctly - true should overwrite false",
		},
		{
			name:     "boolean_false_overwrites_true",
			src:      map[string]any{"enabled": false},
			dest:     map[string]any{"enabled": true},
			expected: map[string]any{"enabled": false},
			problem:  "PROBLEM: false always overwrites true, even when false means 'unset'",
		},
		{
			name:     "boolean_false_overwrites_unset",
			src:      map[string]any{"enabled": false},
			dest:     map[string]any{},
			expected: map[string]any{"enabled": false},
			problem:  "PROBLEM: Cannot distinguish between explicit false vs zero-value false",
		},
		{
			name:     "unset_boolean_does_not_affect_true",
			src:      map[string]any{},
			dest:     map[string]any{"enabled": true},
			expected: map[string]any{"enabled": true},
			problem:  "This works correctly - unset values don't overwrite existing values",
		},
		{
			name:     "nil_boolean_does_not_overwrite",
			src:      map[string]any{"enabled": nil},
			dest:     map[string]any{"enabled": true},
			expected: map[string]any{"enabled": true},
			problem:  "This works correctly - nil values are ignored",
		},
		{
			name: "mixed_types_boolean_and_string",
			src: map[string]any{
				"enabled": false, // This will overwrite
				"name":    "",    // This will NOT overwrite (empty string)
			},
			dest: map[string]any{
				"enabled": true,
				"name":    "original",
			},
			expected: map[string]any{
				"enabled": false,      // PROBLEM: false overwrites true
				"name":    "original", // Correct: empty string doesn't overwrite
			},
			problem: "PROBLEM: Inconsistent behavior - empty string doesn't overwrite but false boolean does",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of dest to avoid modifying the test case
			dest := make(map[string]any)
			for k, v := range tt.dest {
				dest[k] = v
			}

			err := MergeIgnoringNullValues(tt.src, dest)
			if err != nil {
				t.Fatalf("MergeIgnoringNullValues failed: %v", err)
			}

			// Verify the result matches expected
			for k, expectedV := range tt.expected {
				if actualV, exists := dest[k]; !exists {
					t.Errorf("Expected key %q not found in result", k)
				} else if actualV != expectedV {
					t.Errorf("Key %q: expected %v, got %v", k, expectedV, actualV)
				}
			}

			// Verify no extra keys
			for k := range dest {
				if _, exists := tt.expected[k]; !exists {
					t.Errorf("Unexpected key %q in result: %v", k, dest[k])
				}
			}

			t.Logf("Problem: %s", tt.problem)
		})
	}
}

// TestCurrentProviderPrecedenceWithBooleans demonstrates the provider precedence issues
func TestCurrentProviderPrecedenceWithBooleans(t *testing.T) {
	// This test demonstrates how the provider chain would work with current merge logic
	// Simulating: defaults -> struct -> file -> env -> flags

	type testCase struct {
		name        string
		defaults    map[string]any
		structData  map[string]any
		fileData    map[string]any
		envData     map[string]any
		flagsData   map[string]any
		expected    map[string]any
		problemDesc string
	}

	tests := []testCase{
		{
			name:        "boolean_precedence_chain_problem",
			defaults:    map[string]any{}, // No defaults
			structData:  map[string]any{"debug": true},
			fileData:    map[string]any{"debug": false}, // Should win if explicitly set
			envData:     map[string]any{},               // Unset
			flagsData:   map[string]any{},               // Unset
			expected:    map[string]any{"debug": false},
			problemDesc: "PROBLEM: Cannot tell if file's 'debug: false' means 'explicitly disabled' or 'use default'",
		},
		{
			name:        "later_provider_cannot_unset",
			defaults:    map[string]any{"feature": false},
			structData:  map[string]any{"feature": true},
			fileData:    map[string]any{}, // Wants to use struct value
			envData:     map[string]any{}, // Wants to use file/struct value
			flagsData:   map[string]any{}, // Wants to use env/file/struct value
			expected:    map[string]any{"feature": true},
			problemDesc: "This case actually works - unset values don't overwrite existing ones",
		},
		{
			name:        "conflicting_boolean_sources",
			defaults:    map[string]any{"enabled": false},
			structData:  map[string]any{"enabled": true},
			fileData:    map[string]any{"enabled": false}, // Explicit override
			envData:     map[string]any{"enabled": true},  // Higher precedence
			flagsData:   map[string]any{},                 // Unset
			expected:    map[string]any{"enabled": true},  // Env should win
			problemDesc: "This works but we can't tell if false values are explicit or defaults",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the provider chain by applying merges in order
			result := make(map[string]any)

			// Step 1: Load defaults (no merge needed for first load)
			for k, v := range tt.defaults {
				result[k] = v
			}

			// Step 2: Merge struct data (providers don't use merge for struct)
			// But let's simulate what would happen if they did
			if len(tt.structData) > 0 {
				if err := MergeIgnoringNullValues(tt.structData, result); err != nil {
					t.Fatalf("Struct merge failed: %v", err)
				}
			}

			// Step 3: Merge file data (uses MergeIgnoringNullValues)
			if len(tt.fileData) > 0 {
				if err := MergeIgnoringNullValues(tt.fileData, result); err != nil {
					t.Fatalf("File merge failed: %v", err)
				}
			}

			// Step 4: Merge env data (uses MergeIgnoringNullValues)
			if len(tt.envData) > 0 {
				if err := MergeIgnoringNullValues(tt.envData, result); err != nil {
					t.Fatalf("Env merge failed: %v", err)
				}
			}

			// Step 5: Merge flags data (flags don't use merge in current implementation)
			// But let's test what would happen if they did
			if len(tt.flagsData) > 0 {
				if err := MergeIgnoringNullValues(tt.flagsData, result); err != nil {
					t.Fatalf("Flags merge failed: %v", err)
				}
			}

			// Verify result
			for k, expectedV := range tt.expected {
				if actualV, exists := result[k]; !exists {
					t.Errorf("Expected key %q not found in result", k)
				} else if actualV != expectedV {
					t.Errorf("Key %q: expected %v, got %v", k, expectedV, actualV)
				}
			}

			t.Logf("Problem Description: %s", tt.problemDesc)
			t.Logf("Final Result: %+v", result)
		})
	}
}

// TestStringVsBooleanMergeBehavior shows the inconsistency between string and boolean handling
func TestStringVsBooleanMergeBehavior(t *testing.T) {
	tests := []struct {
		name        string
		src         map[string]any
		dest        map[string]any
		expected    map[string]any
		explanation string
	}{
		{
			name: "empty_string_vs_false_boolean",
			src: map[string]any{
				"str_field":  "",    // Empty string - will NOT overwrite
				"bool_field": false, // False boolean - WILL overwrite
			},
			dest: map[string]any{
				"str_field":  "original",
				"bool_field": true,
			},
			expected: map[string]any{
				"str_field":  "original", // Empty string preserved original
				"bool_field": false,      // False boolean overwrote original
			},
			explanation: "INCONSISTENCY: Empty strings don't overwrite, but false booleans do",
		},
		{
			name: "zero_values_behavior",
			src: map[string]any{
				"str_field":   "",      // Zero value string
				"bool_field":  false,   // Zero value boolean
				"int_field":   0,       // Zero value int
				"slice_field": []any{}, // Zero value slice
			},
			dest: map[string]any{
				"str_field":   "existing",
				"bool_field":  true,
				"int_field":   42,
				"slice_field": []any{"item"},
			},
			expected: map[string]any{
				"str_field":   "existing",    // Empty string doesn't overwrite
				"bool_field":  false,         // False boolean overwrites
				"int_field":   0,             // Zero int overwrites (default case)
				"slice_field": []any{"item"}, // Empty slice doesn't overwrite
			},
			explanation: "Zero value behavior is inconsistent across types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := make(map[string]any)
			for k, v := range tt.dest {
				dest[k] = v
			}

			err := MergeIgnoringNullValues(tt.src, dest)
			if err != nil {
				t.Fatalf("Merge failed: %v", err)
			}

			for k, expected := range tt.expected {
				if actual, exists := dest[k]; !exists {
					t.Errorf("Key %q missing from result", k)
				} else {
					// Special handling for slices since they can't be compared with ==
					if actualSlice, ok := actual.([]any); ok {
						if expectedSlice, ok := expected.([]any); ok {
							if len(actualSlice) != len(expectedSlice) {
								t.Errorf("Key %q slice length: expected %d, got %d", k, len(expectedSlice), len(actualSlice))
							}
						} else {
							t.Errorf("Key %q type mismatch: expected %T, got []any", k, expected)
						}
					} else if actual != expected {
						t.Errorf("Key %q: expected %v (%T), got %v (%T)", k, expected, expected, actual, actual)
					}
				}
			}

			t.Logf("Explanation: %s", tt.explanation)
		})
	}
}

// TestMergeWithBooleanPrecedence_OptionalBoolHandling tests the new OptionalBool-aware merge logic
func TestMergeWithBooleanPrecedence_OptionalBoolHandling(t *testing.T) {
	tests := []struct {
		name     string
		src      map[string]any
		dest     map[string]any
		expected map[string]any
		desc     string
	}{
		{
			name:     "optional_bool_set_true_overwrites_anything",
			src:      map[string]any{"enabled": NewOptionalBool(true)},
			dest:     map[string]any{"enabled": false},
			expected: map[string]any{"enabled": NewOptionalBool(true)},
			desc:     "OptionalBool set to true should overwrite any destination value",
		},
		{
			name:     "optional_bool_set_false_overwrites_anything",
			src:      map[string]any{"enabled": NewOptionalBool(false)},
			dest:     map[string]any{"enabled": true},
			expected: map[string]any{"enabled": NewOptionalBool(false)},
			desc:     "OptionalBool set to false should overwrite any destination value",
		},
		{
			name:     "optional_bool_unset_does_not_overwrite",
			src:      map[string]any{"enabled": NewOptionalBoolUnset()},
			dest:     map[string]any{"enabled": true},
			expected: map[string]any{"enabled": true},
			desc:     "OptionalBool unset should not overwrite existing destination value",
		},
		{
			name:     "optional_bool_unset_does_not_overwrite_false",
			src:      map[string]any{"enabled": NewOptionalBoolUnset()},
			dest:     map[string]any{"enabled": false},
			expected: map[string]any{"enabled": false},
			desc:     "OptionalBool unset should not overwrite even false destination value",
		},
		{
			name:     "optional_bool_unset_allows_creation_in_empty_dest",
			src:      map[string]any{"enabled": NewOptionalBoolUnset()},
			dest:     map[string]any{},
			expected: map[string]any{},
			desc:     "OptionalBool unset should not create new key in destination",
		},
		{
			name: "mixed_optional_bool_and_regular_bool",
			src: map[string]any{
				"opt_bool_set":   NewOptionalBool(false), // Should overwrite
				"opt_bool_unset": NewOptionalBoolUnset(), // Should not overwrite
				"regular_bool":   false,                  // Should overwrite (existing behavior)
			},
			dest: map[string]any{
				"opt_bool_set":   true,
				"opt_bool_unset": true,
				"regular_bool":   true,
			},
			expected: map[string]any{
				"opt_bool_set":   NewOptionalBool(false), // Overwritten
				"opt_bool_unset": true,                   // Not overwritten
				"regular_bool":   false,                  // Overwritten (existing behavior)
			},
			desc: "Mixed OptionalBool and regular bool types should each follow their own rules",
		},
		{
			name: "backward_compatibility_non_optional_bool",
			src: map[string]any{
				"str_field":   "",      // Should not overwrite
				"bool_field":  false,   // Should overwrite
				"slice_field": []any{}, // Should not overwrite
			},
			dest: map[string]any{
				"str_field":   "original",
				"bool_field":  true,
				"slice_field": []any{"item"},
			},
			expected: map[string]any{
				"str_field":   "original",    // Not overwritten
				"bool_field":  false,         // Overwritten (existing behavior)
				"slice_field": []any{"item"}, // Not overwritten
			},
			desc: "Non-OptionalBool types should maintain existing merge behavior",
		},
		{
			name:     "nil_optional_bool_pointer_does_not_overwrite",
			src:      map[string]any{"enabled": (*OptionalBool)(nil)},
			dest:     map[string]any{"enabled": true},
			expected: map[string]any{"enabled": true},
			desc:     "Nil OptionalBool pointer should not overwrite existing values",
		},
		{
			name: "optional_bool_by_value_set",
			src: func() map[string]any {
				ob := OptionalBool{}
				ob.Set(true)
				return map[string]any{"enabled": ob}
			}(),
			dest: map[string]any{"enabled": false},
			expected: func() map[string]any {
				ob := OptionalBool{}
				ob.Set(true)
				return map[string]any{"enabled": ob}
			}(),
			desc: "OptionalBool by value (not pointer) when set should overwrite",
		},
		{
			name: "optional_bool_by_value_unset",
			src: func() map[string]any {
				ob := OptionalBool{} // unset by default
				return map[string]any{"enabled": ob}
			}(),
			dest:     map[string]any{"enabled": true},
			expected: map[string]any{"enabled": true},
			desc:     "OptionalBool by value (not pointer) when unset should not overwrite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of dest to avoid modifying the test case
			dest := make(map[string]any)
			for k, v := range tt.dest {
				dest[k] = v
			}

			err := MergeWithBooleanPrecedence(tt.src, dest)
			if err != nil {
				t.Fatalf("MergeWithBooleanPrecedence failed: %v", err)
			}

			// Verify the result matches expected
			for k, expectedV := range tt.expected {
				actualV, exists := dest[k]
				if !exists {
					t.Errorf("Expected key %q not found in result", k)
					continue
				}

				// Special handling for OptionalBool comparison
				if expectedOB, ok := expectedV.(*OptionalBool); ok {
					if actualOB, ok := actualV.(*OptionalBool); ok {
						if expectedOB.IsSet() != actualOB.IsSet() {
							t.Errorf("Key %q IsSet(): expected %v, got %v", k, expectedOB.IsSet(), actualOB.IsSet())
						}
						if expectedOB.IsSet() && actualOB.IsSet() && expectedOB.Value() != actualOB.Value() {
							t.Errorf("Key %q Value(): expected %v, got %v", k, expectedOB.Value(), actualOB.Value())
						}
					} else {
						t.Errorf("Key %q type: expected *OptionalBool, got %T", k, actualV)
					}
				} else if expectedOB, ok := expectedV.(OptionalBool); ok {
					if actualOB, ok := actualV.(OptionalBool); ok {
						if expectedOB.IsSet() != actualOB.IsSet() {
							t.Errorf("Key %q IsSet(): expected %v, got %v", k, expectedOB.IsSet(), actualOB.IsSet())
						}
						if expectedOB.IsSet() && actualOB.IsSet() && expectedOB.Value() != actualOB.Value() {
							t.Errorf("Key %q Value(): expected %v, got %v", k, expectedOB.Value(), actualOB.Value())
						}
					} else {
						t.Errorf("Key %q type: expected OptionalBool, got %T", k, actualV)
					}
				} else {
					// Regular comparison with special handling for slices
					if actualSlice, ok := actualV.([]any); ok {
						if expectedSlice, ok := expectedV.([]any); ok {
							if len(actualSlice) != len(expectedSlice) {
								t.Errorf("Key %q slice length: expected %d, got %d", k, len(expectedSlice), len(actualSlice))
							} else {
								for i, expectedItem := range expectedSlice {
									if actualSlice[i] != expectedItem {
										t.Errorf("Key %q[%d]: expected %v, got %v", k, i, expectedItem, actualSlice[i])
									}
								}
							}
						} else {
							t.Errorf("Key %q type: expected []any, got %T", k, expectedV)
						}
					} else if actualV != expectedV {
						t.Errorf("Key %q: expected %v, got %v", k, expectedV, actualV)
					}
				}
			}

			// Verify no extra keys
			for k := range dest {
				if _, exists := tt.expected[k]; !exists {
					t.Errorf("Unexpected key %q in result: %v", k, dest[k])
				}
			}

			t.Logf("Description: %s", tt.desc)
		})
	}
}

func TestMergeWithBooleanPrecedence_MapOverwritesScalar(t *testing.T) {
	src := map[string]any{
		"db": map[string]any{
			"dsn": "postgres://localhost",
		},
	}
	dest := map[string]any{
		"db": "sqlite.db",
	}

	if err := MergeWithBooleanPrecedence(src, dest); err != nil {
		t.Fatalf("MergeWithBooleanPrecedence failed: %v", err)
	}

	db, ok := dest["db"].(map[string]any)
	if !ok {
		t.Fatalf("expected db to be map after merge, got %T", dest["db"])
	}
	if db["dsn"] != "postgres://localhost" {
		t.Fatalf("expected db.dsn to be overwritten, got %v", db["dsn"])
	}
}

// TestMergeWithBooleanPrecedence_RecursiveMapHandling tests recursive map merge behavior
func TestMergeWithBooleanPrecedence_RecursiveMapHandling(t *testing.T) {
	tests := []struct {
		name     string
		src      map[string]any
		dest     map[string]any
		expected map[string]any
	}{
		{
			name: "nested_map_with_optional_bool",
			src: map[string]any{
				"config": map[string]any{
					"enabled": NewOptionalBool(false),
					"debug":   NewOptionalBoolUnset(),
				},
			},
			dest: map[string]any{
				"config": map[string]any{
					"enabled": true,
					"debug":   true,
				},
			},
			expected: map[string]any{
				"config": map[string]any{
					"enabled": NewOptionalBool(false), // Should overwrite
					"debug":   true,                   // Should not overwrite
				},
			},
		},
		{
			name: "deeply_nested_optional_bool",
			src: map[string]any{
				"app": map[string]any{
					"features": map[string]any{
						"feature_a": NewOptionalBool(true),
						"feature_b": NewOptionalBoolUnset(),
					},
				},
			},
			dest: map[string]any{
				"app": map[string]any{
					"features": map[string]any{
						"feature_a": false,
						"feature_b": true,
					},
				},
			},
			expected: map[string]any{
				"app": map[string]any{
					"features": map[string]any{
						"feature_a": NewOptionalBool(true), // Should overwrite
						"feature_b": true,                  // Should not overwrite
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a deep copy of dest
			dest := deepCopyMap(tt.dest)

			err := MergeWithBooleanPrecedence(tt.src, dest)
			if err != nil {
				t.Fatalf("MergeWithBooleanPrecedence failed: %v", err)
			}

			// Verify nested structure
			verifyNestedMap(t, tt.expected, dest, "")
		})
	}
}

// Helper function to deep copy a map for testing
func deepCopyMap(original map[string]any) map[string]any {
	copy := make(map[string]any)
	for k, v := range original {
		if subMap, ok := v.(map[string]any); ok {
			copy[k] = deepCopyMap(subMap)
		} else {
			copy[k] = v
		}
	}
	return copy
}

// Helper function to verify nested maps in tests
func verifyNestedMap(t *testing.T, expected, actual map[string]any, path string) {
	for k, expectedV := range expected {
		actualV, exists := actual[k]
		keyPath := path + k

		if !exists {
			t.Errorf("Expected key %q not found", keyPath)
			continue
		}

		if expectedMap, ok := expectedV.(map[string]any); ok {
			if actualMap, ok := actualV.(map[string]any); ok {
				verifyNestedMap(t, expectedMap, actualMap, keyPath+".")
			} else {
				t.Errorf("Key %q: expected map[string]any, got %T", keyPath, actualV)
			}
		} else if expectedOB, ok := expectedV.(*OptionalBool); ok {
			if actualOB, ok := actualV.(*OptionalBool); ok {
				if expectedOB.IsSet() != actualOB.IsSet() {
					t.Errorf("Key %q IsSet(): expected %v, got %v", keyPath, expectedOB.IsSet(), actualOB.IsSet())
				}
				if expectedOB.IsSet() && actualOB.IsSet() && expectedOB.Value() != actualOB.Value() {
					t.Errorf("Key %q Value(): expected %v, got %v", keyPath, expectedOB.Value(), actualOB.Value())
				}
			} else {
				// Check if we have a regular bool instead of OptionalBool (which is valid for unset scenarios)
				if expectedOB.IsSet() {
					t.Errorf("Key %q type: expected *OptionalBool, got %T", keyPath, actualV)
				} else {
					// For unset OptionalBool, the value should remain as the original type
					t.Logf("Key %q: unset OptionalBool preserved original value %v (%T)", keyPath, actualV, actualV)
				}
			}
		} else {
			if actualV != expectedV {
				t.Errorf("Key %q: expected %v, got %v", keyPath, expectedV, actualV)
			}
		}
	}
}
