package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf/v2"
)

// Test configuration structure with both OptionalBool and regular bool
type TestConfig struct {
	OptionalFeature *OptionalBool `koanf:"optional_feature"`
	RegularFeature  bool          `koanf:"regular_feature"`
	Name            string        `koanf:"name"`
	Debug           bool          `koanf:"debug"`
}

// Implement Validable interface for TestConfig
func (tc TestConfig) Validate() error {
	return nil
}

// TestProviderIntegrationWithNewMerge tests that all providers work correctly with MergeWithBooleanPrecedence
func TestProviderIntegrationWithNewMerge(t *testing.T) {

	tests := []struct {
		name           string
		defaults       map[string]any
		fileContent    string
		expectedResult TestConfig
		description    string
	}{
		{
			name: "optional_bool_respects_precedence",
			defaults: map[string]any{
				"optional_feature": NewOptionalBool(true),
				"regular_feature":  true,
				"name":             "default",
				"debug":            false,
			},
			fileContent: `{
				"optional_feature": null,
				"regular_feature": false,
				"name": "from_file",
				"debug": true
			}`,
			expectedResult: TestConfig{
				OptionalFeature: NewOptionalBool(true), // null in file shouldn't overwrite default
				RegularFeature:  false,                 // false in file should overwrite default true
				Name:            "from_file",
				Debug:           true,
			},
			description: "OptionalBool null values should not overwrite defaults, regular values should work normally",
		},
		{
			name: "optional_bool_explicit_false_overwrites",
			defaults: map[string]any{
				"optional_feature": NewOptionalBool(true),
				"regular_feature":  true,
				"name":             "default",
			},
			fileContent: `{
				"optional_feature": false,
				"regular_feature": false,
				"name": ""
			}`,
			expectedResult: TestConfig{
				OptionalFeature: NewOptionalBool(false), // explicit false should overwrite
				RegularFeature:  false,                  // regular false should overwrite
				Name:            "default",              // empty string shouldn't overwrite
			},
			description: "Explicit false OptionalBool should overwrite, empty strings should not",
		},
		{
			name: "mixed_unset_values",
			defaults: map[string]any{
				"optional_feature": NewOptionalBoolUnset(),
				"regular_feature":  false,
				"name":             "default",
			},
			fileContent: `{
				"regular_feature": true,
				"name": "override"
			}`,
			expectedResult: TestConfig{
				OptionalFeature: NewOptionalBoolUnset(), // should remain unset
				RegularFeature:  true,                   // should be overridden
				Name:            "override",
			},
			description: "Unset OptionalBool should remain unset when not provided in file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tempDir, err := os.MkdirTemp("", "config_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			configPath := filepath.Join(tempDir, "test.json")
			if err := os.WriteFile(configPath, []byte(tt.fileContent), 0644); err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			// Create a test container
			container := &Container[TestConfig]{
				logger: &testLogger{},
			}

			// Test file provider with new merge function
			provider, err := FileProvider[TestConfig](configPath)(container)
			if err != nil {
				t.Fatalf("Failed to create file provider: %v", err)
			}

			// Load defaults first
			k := koanf.New(".")
			defaultProvider, err := DefaultValuesProvider[TestConfig](tt.defaults)(container)
			if err != nil {
				t.Fatalf("Failed to create default provider: %v", err)
			}

			if err := defaultProvider.Load(context.Background(), k); err != nil {
				t.Fatalf("Failed to load defaults: %v", err)
			}

			// Load file with new merge function (should be automatically used by provider)
			if err := provider.Load(context.Background(), k); err != nil {
				t.Fatalf("Failed to load file: %v", err)
			}

			// Unmarshal into struct
			var result TestConfig
			if err := k.UnmarshalWithConf("", &result, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
				t.Fatalf("Failed to unmarshal config: %v", err)
			}

			// Verify results
			if tt.expectedResult.OptionalFeature != nil && result.OptionalFeature != nil {
				if tt.expectedResult.OptionalFeature.IsSet() != result.OptionalFeature.IsSet() {
					t.Errorf("OptionalFeature IsSet(): expected %v, got %v",
						tt.expectedResult.OptionalFeature.IsSet(), result.OptionalFeature.IsSet())
				}
				if tt.expectedResult.OptionalFeature.IsSet() && result.OptionalFeature.IsSet() {
					if tt.expectedResult.OptionalFeature.Value() != result.OptionalFeature.Value() {
						t.Errorf("OptionalFeature Value(): expected %v, got %v",
							tt.expectedResult.OptionalFeature.Value(), result.OptionalFeature.Value())
					}
				}
			} else if tt.expectedResult.OptionalFeature != result.OptionalFeature {
				t.Errorf("OptionalFeature: expected %v, got %v", tt.expectedResult.OptionalFeature, result.OptionalFeature)
			}

			if result.RegularFeature != tt.expectedResult.RegularFeature {
				t.Errorf("RegularFeature: expected %v, got %v", tt.expectedResult.RegularFeature, result.RegularFeature)
			}

			if result.Name != tt.expectedResult.Name {
				t.Errorf("Name: expected %q, got %q", tt.expectedResult.Name, result.Name)
			}

			if result.Debug != tt.expectedResult.Debug {
				t.Errorf("Debug: expected %v, got %v", tt.expectedResult.Debug, result.Debug)
			}

			t.Logf("Description: %s", tt.description)
		})
	}
}

// TestRegressionExistingMergeBehavior ensures existing configurations still work
func TestRegressionExistingMergeBehavior(t *testing.T) {
	tests := []struct {
		name     string
		src      map[string]any
		dest     map[string]any
		expected map[string]any
	}{
		{
			name: "existing_string_behavior",
			src: map[string]any{
				"empty_string":  "",
				"filled_string": "value",
			},
			dest: map[string]any{
				"empty_string":  "original",
				"filled_string": "original",
			},
			expected: map[string]any{
				"empty_string":  "original", // Empty strings don't overwrite
				"filled_string": "value",    // Non-empty strings do overwrite
			},
		},
		{
			name: "existing_slice_behavior",
			src: map[string]any{
				"empty_slice":  []any{},
				"filled_slice": []any{"item"},
			},
			dest: map[string]any{
				"empty_slice":  []any{"original"},
				"filled_slice": []any{"original"},
			},
			expected: map[string]any{
				"empty_slice":  []any{"original"}, // Empty slices don't overwrite
				"filled_slice": []any{"item"},     // Non-empty slices do overwrite
			},
		},
		{
			name: "existing_bool_behavior",
			src: map[string]any{
				"bool_false": false,
				"bool_true":  true,
			},
			dest: map[string]any{
				"bool_false": true,
				"bool_true":  false,
			},
			expected: map[string]any{
				"bool_false": false, // false overwrites true (existing behavior)
				"bool_true":  true,  // true overwrites false (existing behavior)
			},
		},
		{
			name: "existing_nil_behavior",
			src: map[string]any{
				"nil_value":  nil,
				"some_value": "value",
			},
			dest: map[string]any{
				"nil_value":  "original",
				"some_value": "original",
			},
			expected: map[string]any{
				"nil_value":  "original", // nil doesn't overwrite
				"some_value": "value",    // non-nil does overwrite
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test both old and new merge functions for regression
			for _, mergeFunc := range []struct {
				name string
				fn   func(map[string]any, map[string]any) error
			}{
				{"MergeIgnoringNullValues", MergeIgnoringNullValues},
				{"MergeWithBooleanPrecedence", MergeWithBooleanPrecedence},
			} {
				t.Run(mergeFunc.name, func(t *testing.T) {
					// Make a copy of dest
					dest := make(map[string]any)
					for k, v := range tt.dest {
						if slice, ok := v.([]any); ok {
							// Deep copy slices
							destSlice := make([]any, len(slice))
							copy(destSlice, slice)
							dest[k] = destSlice
						} else {
							dest[k] = v
						}
					}

					err := mergeFunc.fn(tt.src, dest)
					if err != nil {
						t.Fatalf("%s failed: %v", mergeFunc.name, err)
					}

					for k, expectedV := range tt.expected {
						actualV, exists := dest[k]
						if !exists {
							t.Errorf("Key %q missing from result", k)
							continue
						}

						// Special handling for slices
						if expectedSlice, ok := expectedV.([]any); ok {
							if actualSlice, ok := actualV.([]any); ok {
								if len(expectedSlice) != len(actualSlice) {
									t.Errorf("Key %q slice length: expected %d, got %d", k, len(expectedSlice), len(actualSlice))
								} else {
									for i, expectedItem := range expectedSlice {
										if actualSlice[i] != expectedItem {
											t.Errorf("Key %q[%d]: expected %v, got %v", k, i, expectedItem, actualSlice[i])
										}
									}
								}
							} else {
								t.Errorf("Key %q type: expected []any, got %T", k, actualV)
							}
						} else if actualV != expectedV {
							t.Errorf("Key %q: expected %v, got %v", k, expectedV, actualV)
						}
					}
				})
			}
		})
	}
}

// Simple logger implementation for testing
type testLogger struct{}

func (l *testLogger) Debug(msg string, args ...any) {}
func (l *testLogger) Info(msg string, args ...any)  {}
func (l *testLogger) Warn(msg string, args ...any)  {}
func (l *testLogger) Error(msg string, args ...any) {}

// Benchmark tests to ensure no performance regression
func BenchmarkMergeIgnoringNullValues(b *testing.B) {
	src := map[string]any{
		"string_field": "value",
		"bool_field":   false,
		"int_field":    42,
		"slice_field":  []any{"item1", "item2"},
		"nested": map[string]any{
			"deep_bool":   true,
			"deep_string": "nested_value",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dest := map[string]any{
			"string_field": "original",
			"bool_field":   true,
			"int_field":    0,
			"slice_field":  []any{},
			"nested": map[string]any{
				"deep_bool":   false,
				"deep_string": "",
			},
		}

		err := MergeIgnoringNullValues(src, dest)
		if err != nil {
			b.Fatalf("Merge failed: %v", err)
		}
	}
}

func BenchmarkMergeWithBooleanPrecedence(b *testing.B) {
	src := map[string]any{
		"string_field": "value",
		"bool_field":   false,
		"int_field":    42,
		"slice_field":  []any{"item1", "item2"},
		"nested": map[string]any{
			"deep_bool":   true,
			"deep_string": "nested_value",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dest := map[string]any{
			"string_field": "original",
			"bool_field":   true,
			"int_field":    0,
			"slice_field":  []any{},
			"nested": map[string]any{
				"deep_bool":   false,
				"deep_string": "",
			},
		}

		err := MergeWithBooleanPrecedence(src, dest)
		if err != nil {
			b.Fatalf("Merge failed: %v", err)
		}
	}
}

func BenchmarkMergeWithOptionalBool(b *testing.B) {
	src := map[string]any{
		"string_field":        "value",
		"regular_bool":        false,
		"optional_bool_set":   NewOptionalBool(true),
		"optional_bool_unset": NewOptionalBoolUnset(),
		"int_field":           42,
		"nested": map[string]any{
			"deep_optional": NewOptionalBool(false),
			"deep_string":   "nested_value",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dest := map[string]any{
			"string_field":        "original",
			"regular_bool":        true,
			"optional_bool_set":   NewOptionalBool(false),
			"optional_bool_unset": NewOptionalBool(true),
			"int_field":           0,
			"nested": map[string]any{
				"deep_optional": NewOptionalBool(true),
				"deep_string":   "",
			},
		}

		err := MergeWithBooleanPrecedence(src, dest)
		if err != nil {
			b.Fatalf("Merge failed: %v", err)
		}
	}
}

// Benchmark comparison between old and new merge functions
func BenchmarkMergeComparison(b *testing.B) {
	// Test data without OptionalBool (should perform similarly)
	src := map[string]any{
		"field1": "value1",
		"field2": false,
		"field3": 42,
		"field4": []any{"a", "b"},
		"nested": map[string]any{
			"sub1": "subvalue",
			"sub2": true,
		},
	}

	benchmarks := []struct {
		name string
		fn   func(map[string]any, map[string]any) error
	}{
		{"Old", MergeIgnoringNullValues},
		{"New", MergeWithBooleanPrecedence},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dest := map[string]any{
					"field1": "original1",
					"field2": true,
					"field3": 0,
					"field4": []any{},
					"nested": map[string]any{
						"sub1": "original_sub",
						"sub2": false,
					},
				}

				err := bm.fn(src, dest)
				if err != nil {
					b.Fatalf("Merge failed: %v", err)
				}
			}
		})
	}
}
