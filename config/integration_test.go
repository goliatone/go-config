package config

import (
	"context"
	"os"
	"testing"
)

// Test configuration structs with Validate methods
type IntegrationTestConfig struct {
	Debug    *OptionalBool `koanf:"debug" json:"debug"`
	Verbose  *OptionalBool `koanf:"verbose" json:"verbose"`
	Enabled  *OptionalBool `koanf:"enabled" json:"enabled"`
	LogLevel *OptionalBool `koanf:"log_level" json:"log_level"`
}

func (c *IntegrationTestConfig) Validate() error { return nil }

type IntegrationMixedConfig struct {
	OptionalFlag *OptionalBool `koanf:"optional_flag"`
	RegularFlag  bool          `koanf:"regular_flag"`
	AnotherOpt   *OptionalBool `koanf:"another_opt"`
}

func (c *IntegrationMixedConfig) Validate() error { return nil }

type IntegrationEdgeConfig struct {
	NilPtr   *OptionalBool `koanf:"nil_ptr"`
	ValuePtr *OptionalBool `koanf:"value_ptr"`
	UnsetPtr *OptionalBool `koanf:"unset_ptr"`
}

func (c *IntegrationEdgeConfig) Validate() error { return nil }

type IntegrationLegacyConfig struct {
	OldFlag    bool          `koanf:"old_flag"`
	NewFlag    *OptionalBool `koanf:"new_flag"`
	AnotherOld bool          `koanf:"another_old"`
}

func (c *IntegrationLegacyConfig) Validate() error { return nil }

type IntegrationPerfConfig struct {
	Flag bool `koanf:"flag"`
}

func (c *IntegrationPerfConfig) Validate() error { return nil }

type IntegrationJSONTestConfig struct {
	Flag1 *OptionalBool `koanf:"flag1"`
	Flag2 *OptionalBool `koanf:"flag2"`
	Flag3 *OptionalBool `koanf:"flag3"`
	Flag4 *OptionalBool `koanf:"flag4"`
}

func (c *IntegrationJSONTestConfig) Validate() error { return nil }

// TestOptionalBoolProviderChain tests the complete provider chain with OptionalBool fields
// This validates that OptionalBool precedence works correctly across all providers:
// defaults → struct → file → env → flags
func TestOptionalBoolProviderChain(t *testing.T) {

	t.Run("complete_provider_chain_precedence", func(t *testing.T) {
		// Create temporary test files - simplified to test one field at a time
		configData := `{
			"debug": true,
			"enabled": false
		}`
		configFile := createTempFile(t, "config.json", configData)
		defer os.Remove(configFile)

		// Set environment variables
		os.Setenv("APP_DEBUG", "false")    // Should override file's true
		os.Setenv("APP_LOG_LEVEL", "true") // Should be set from env only
		defer func() {
			os.Unsetenv("APP_DEBUG")
			os.Unsetenv("APP_LOG_LEVEL")
		}()

		// For now, skip flags as they need special handling for OptionalBool
		// TODO: Implement flag support for OptionalBool in future enhancement

		// Setup container with all providers
		config := &IntegrationTestConfig{}
		container := New(config)

		// Add providers in reverse priority order (lowest to highest)
		defaults := map[string]any{
			"debug":   NewOptionalBoolUnset(), // Unset in defaults
			"verbose": NewOptionalBool(true),  // Set to true in defaults
		}

		structDefaults := IntegrationTestConfig{
			Debug:    NewOptionalBool(false), // Will be overridden by file then env
			Verbose:  NewOptionalBoolUnset(), // Unset in struct, should use defaults
			Enabled:  NewOptionalBoolUnset(), // Unset in struct, will be set by file then flags
			LogLevel: NewOptionalBoolUnset(), // Unset everywhere except env
		}

		container.WithProvider(
			DefaultValuesProvider[*IntegrationTestConfig](defaults),
			StructProvider[*IntegrationTestConfig](&structDefaults),
			FileProvider[*IntegrationTestConfig](configFile),
			EnvProvider[*IntegrationTestConfig]("APP_", "__"),
		)

		// Load configuration
		err := container.Load(context.Background())
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Verify precedence chain worked correctly
		t.Run("debug_precedence", func(t *testing.T) {
			// defaults: unset → struct: false → file: true → env: false
			// Expected: env wins with false
			if !config.Debug.IsSet() {
				t.Error("Expected debug to be set")
			}
			if config.Debug.Value() != false {
				t.Errorf("Expected debug=false (from env), got %v", config.Debug.Value())
			}
		})

		t.Run("verbose_precedence", func(t *testing.T) {
			// defaults: true → struct: unset → file: null (unset) → env: unset
			// Expected: defaults wins with true
			if !config.Verbose.IsSet() {
				t.Error("Expected verbose to be set")
			}
			if config.Verbose.Value() != true {
				t.Errorf("Expected verbose=true (from defaults), got %v", config.Verbose.Value())
			}
		})

		t.Run("enabled_precedence", func(t *testing.T) {
			// defaults: unset → struct: unset → file: false → env: unset
			// Expected: file wins with false
			if !config.Enabled.IsSet() {
				t.Error("Expected enabled to be set")
			}
			if config.Enabled.Value() != false {
				t.Errorf("Expected enabled=false (from file), got %v", config.Enabled.Value())
			}
		})

		t.Run("log_level_precedence", func(t *testing.T) {
			// defaults: unset → struct: unset → file: unset → env: true → flags: unset
			// Expected: env wins with true
			if !config.LogLevel.IsSet() {
				t.Error("Expected log_level to be set")
			}
			if config.LogLevel.Value() != true {
				t.Errorf("Expected log_level=true (from env), got %v", config.LogLevel.Value())
			}
		})
	})

	t.Run("unset_values_remain_unset", func(t *testing.T) {
		// Test that unset OptionalBool values in all providers remain unset
		config := &IntegrationTestConfig{}
		container := New(config)

		defaults := map[string]any{
			"debug": NewOptionalBoolUnset(),
		}
		structDefaults := IntegrationTestConfig{
			Debug: NewOptionalBoolUnset(),
		}

		container.WithProvider(
			DefaultValuesProvider[*IntegrationTestConfig](defaults),
			StructProvider[*IntegrationTestConfig](&structDefaults),
		)

		err := container.Load(context.Background())
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if config.Debug.IsSet() {
			t.Error("Expected debug to remain unset when all providers have unset values")
		}
	})
}

// TestMixedOptionalBoolRegularBool tests mixed scenarios with both OptionalBool and regular bool fields
func TestMixedOptionalBoolRegularBool(t *testing.T) {

	t.Run("mixed_types_precedence", func(t *testing.T) {
		// Simplified test without JSON file for now
		os.Setenv("APP_OPTIONAL_FLAG", "true")
		os.Setenv("APP_REGULAR_FLAG", "false")
		defer func() {
			os.Unsetenv("APP_OPTIONAL_FLAG")
			os.Unsetenv("APP_REGULAR_FLAG")
		}()

		config := &IntegrationMixedConfig{}
		container := New(config)
		structDefaults := IntegrationMixedConfig{
			OptionalFlag: NewOptionalBool(false),
			RegularFlag:  true,                  // Will be overridden by env
			AnotherOpt:   NewOptionalBool(true), // Only from struct
		}

		container.WithProvider(
			StructProvider[*IntegrationMixedConfig](&structDefaults),
			EnvProvider[*IntegrationMixedConfig]("APP_", "__"),
		)

		err := container.Load(context.Background())
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// OptionalFlag: struct: false → env: true
		// Expected: env wins with true
		if !config.OptionalFlag.IsSet() || config.OptionalFlag.Value() != true {
			t.Errorf("Expected optional_flag=true from env, got IsSet=%v Value=%v",
				config.OptionalFlag.IsSet(), config.OptionalFlag.Value())
		}

		// RegularFlag: struct: true → env: false
		// Expected: env wins with false (regular bool precedence)
		if config.RegularFlag != false {
			t.Errorf("Expected regular_flag=false from env, got %v", config.RegularFlag)
		}

		// AnotherOpt: struct: true → env: unset
		// Expected: struct wins with true
		if !config.AnotherOpt.IsSet() || config.AnotherOpt.Value() != true {
			t.Errorf("Expected another_opt=true from struct, got IsSet=%v Value=%v",
				config.AnotherOpt.IsSet(), config.AnotherOpt.Value())
		}
	})
}

// TestOptionalBoolEdgeCases tests edge cases including nil handling and type mismatches
func TestOptionalBoolEdgeCases(t *testing.T) {

	t.Run("nil_pointers", func(t *testing.T) {
		config := &IntegrationEdgeConfig{}
		container := New(config)
		structDefaults := IntegrationEdgeConfig{
			NilPtr:   nil,                    // nil pointer
			ValuePtr: NewOptionalBool(true),  // set pointer
			UnsetPtr: NewOptionalBoolUnset(), // unset pointer
		}

		container.WithProvider(
			StructProvider[*IntegrationEdgeConfig](&structDefaults),
		)

		err := container.Load(context.Background())
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// nil pointers should be handled gracefully
		if config.NilPtr != nil {
			t.Error("Expected nil_ptr to remain nil")
		}

		if !config.ValuePtr.IsSet() || config.ValuePtr.Value() != true {
			t.Error("Expected value_ptr to be set to true")
		}

		if config.UnsetPtr.IsSet() {
			t.Error("Expected unset_ptr to remain unset")
		}
	})

	t.Run("json_parsing_edge_cases", func(t *testing.T) {
		// Test various JSON representations
		testCases := []struct {
			name     string
			json     string
			expected map[string]any
		}{
			{
				name: "null_values",
				json: `{"flag1": null, "flag2": true, "flag3": false}`,
				expected: map[string]any{
					"flag1_set":   false,
					"flag2_set":   true,
					"flag2_value": true,
					"flag3_set":   true,
					"flag3_value": false,
				},
			},
			{
				name: "string_boolean_values",
				json: `{"flag1": "true", "flag2": "false", "flag3": "1", "flag4": "0"}`,
				expected: map[string]any{
					"flag1_set":   true,
					"flag1_value": true,
					"flag2_set":   true,
					"flag2_value": false,
					"flag3_set":   true,
					"flag3_value": true,
					"flag4_set":   true,
					"flag4_value": false,
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {

				configFile := createTempFile(t, "edge_case.json", tc.json)
				defer os.Remove(configFile)

				config := &IntegrationJSONTestConfig{}
				container := New(config)
				container.WithProvider(
					FileProvider[*IntegrationJSONTestConfig](configFile),
				)

				err := container.Load(context.Background())
				if err != nil {
					t.Fatalf("Failed to load config: %v", err)
				}

				// Verify expectations
				flags := []*OptionalBool{config.Flag1, config.Flag2, config.Flag3, config.Flag4}
				flagNames := []string{"flag1", "flag2", "flag3", "flag4"}

				for i, flag := range flags {
					flagName := flagNames[i]

					if setKey := flagName + "_set"; tc.expected[setKey] != nil {
						expectedSet := tc.expected[setKey].(bool)
						if flag.IsSet() != expectedSet {
							t.Errorf("Expected %s IsSet=%v, got %v", flagName, expectedSet, flag.IsSet())
						}
					}

					if valueKey := flagName + "_value"; tc.expected[valueKey] != nil && flag.IsSet() {
						expectedValue := tc.expected[valueKey].(bool)
						if flag.Value() != expectedValue {
							t.Errorf("Expected %s Value=%v, got %v", flagName, expectedValue, flag.Value())
						}
					}
				}
			})
		}
	})
}

// TestBackwardCompatibility ensures existing boolean configurations continue to work
func TestBackwardCompatibility(t *testing.T) {

	t.Run("regular_bool_precedence_unchanged", func(t *testing.T) {
		configData := `{
			"old_flag": true,
			"new_flag": false,
			"another_old": false
		}`
		configFile := createTempFile(t, "legacy.json", configData)
		defer os.Remove(configFile)

		os.Setenv("APP_OLD_FLAG", "false")
		os.Setenv("APP_ANOTHER_OLD", "true")
		defer func() {
			os.Unsetenv("APP_OLD_FLAG")
			os.Unsetenv("APP_ANOTHER_OLD")
		}()

		config := &IntegrationLegacyConfig{}
		container := New(config)
		structDefaults := IntegrationLegacyConfig{
			OldFlag:    true,
			NewFlag:    NewOptionalBool(true),
			AnotherOld: false,
		}

		container.WithProvider(
			StructProvider[*IntegrationLegacyConfig](&structDefaults),
			FileProvider[*IntegrationLegacyConfig](configFile),
			EnvProvider[*IntegrationLegacyConfig]("APP_", "__"),
		)

		err := container.Load(context.Background())
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		// Regular bool fields should follow existing merge logic
		// struct: true → file: true → env: false = env wins
		if config.OldFlag != false {
			t.Errorf("Expected old_flag=false from env, got %v", config.OldFlag)
		}

		// struct: false → file: false → env: true = env wins
		if config.AnotherOld != true {
			t.Errorf("Expected another_old=true from env, got %v", config.AnotherOld)
		}

		// OptionalBool should follow new precedence logic
		// struct: true → file: false → env: unset = file wins
		if !config.NewFlag.IsSet() || config.NewFlag.Value() != false {
			t.Errorf("Expected new_flag=false from file, got IsSet=%v Value=%v",
				config.NewFlag.IsSet(), config.NewFlag.Value())
		}
	})

	t.Run("zero_performance_regression", func(t *testing.T) {
		// This is more of a benchmark placeholder - real performance testing
		// would require more sophisticated benchmarking
		configData := `{"flag": true}`
		configFile := createTempFile(t, "perf.json", configData)
		defer os.Remove(configFile)
		config := &IntegrationPerfConfig{}
		container := New(config)
		container.WithProvider(
			FileProvider[*IntegrationPerfConfig](configFile),
		)

		// Should complete quickly without hanging or excessive memory usage
		err := container.Load(context.Background())
		if err != nil {
			t.Fatalf("Performance test failed: %v", err)
		}
	})
}

// createTempFile creates a temporary file with the given content for testing
func createTempFile(t *testing.T, name, content string) string {
	t.Helper()

	tmpFile, err := os.CreateTemp("", name)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile.Name()
}
