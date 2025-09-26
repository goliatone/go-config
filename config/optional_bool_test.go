package config

import (
	"encoding/json"
	"testing"
)

func TestOptionalBool_BasicOperations(t *testing.T) {
	t.Run("zero value is unset", func(t *testing.T) {
		var ob OptionalBool
		if ob.IsSet() {
			t.Error("zero value should be unset")
		}
		if ob.Value() != false {
			t.Error("zero value should return false from Value()")
		}
		if ob.BoolOr(true) != true {
			t.Error("zero value should return default from BoolOr()")
		}
	})

	t.Run("set true", func(t *testing.T) {
		var ob OptionalBool
		ob.Set(true)
		if !ob.IsSet() {
			t.Error("should be set after Set(true)")
		}
		if !ob.Value() {
			t.Error("should return true from Value()")
		}
		if !ob.BoolOr(false) {
			t.Error("should return true from BoolOr() when set to true")
		}
	})

	t.Run("set false", func(t *testing.T) {
		var ob OptionalBool
		ob.Set(false)
		if !ob.IsSet() {
			t.Error("should be set after Set(false)")
		}
		if ob.Value() {
			t.Error("should return false from Value()")
		}
		if ob.BoolOr(true) {
			t.Error("should return false from BoolOr() when set to false")
		}
	})

	t.Run("unset after set", func(t *testing.T) {
		var ob OptionalBool
		ob.Set(true)
		ob.Unset()
		if ob.IsSet() {
			t.Error("should be unset after Unset()")
		}
		if ob.Value() != false {
			t.Error("should return false from Value() after unset")
		}
		if ob.BoolOr(true) != true {
			t.Error("should return default from BoolOr() after unset")
		}
	})
}

func TestOptionalBool_Constructors(t *testing.T) {
	t.Run("NewOptionalBool true", func(t *testing.T) {
		ob := NewOptionalBool(true)
		if !ob.IsSet() {
			t.Error("should be set")
		}
		if !ob.Value() {
			t.Error("should be true")
		}
	})

	t.Run("NewOptionalBool false", func(t *testing.T) {
		ob := NewOptionalBool(false)
		if !ob.IsSet() {
			t.Error("should be set")
		}
		if ob.Value() {
			t.Error("should be false")
		}
	})

	t.Run("NewOptionalBoolUnset", func(t *testing.T) {
		ob := NewOptionalBoolUnset()
		if ob.IsSet() {
			t.Error("should be unset")
		}
		if ob.Value() != false {
			t.Error("should return false from Value()")
		}
	})
}

func TestOptionalBool_String(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *OptionalBool
		expected string
	}{
		{
			name:     "unset",
			setup:    func() *OptionalBool { return &OptionalBool{} },
			expected: "<unset>",
		},
		{
			name:     "set true",
			setup:    func() *OptionalBool { return NewOptionalBool(true) },
			expected: "true",
		},
		{
			name:     "set false",
			setup:    func() *OptionalBool { return NewOptionalBool(false) },
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ob := tt.setup()
			if got := ob.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOptionalBool_JSONMarshal(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *OptionalBool
		expected string
	}{
		{
			name:     "unset marshals to null",
			setup:    func() *OptionalBool { return &OptionalBool{} },
			expected: "null",
		},
		{
			name:     "true marshals to true",
			setup:    func() *OptionalBool { return NewOptionalBool(true) },
			expected: "true",
		},
		{
			name:     "false marshals to false",
			setup:    func() *OptionalBool { return NewOptionalBool(false) },
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ob := tt.setup()
			data, err := json.Marshal(ob)
			if err != nil {
				t.Fatalf("Marshal error: %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("Marshal() = %s, want %s", string(data), tt.expected)
			}
		})
	}
}

func TestOptionalBool_JSONUnmarshal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectSet   bool
		expectValue bool
		expectError bool
	}{
		{
			name:        "null unmarshals to unset",
			input:       "null",
			expectSet:   false,
			expectValue: false,
			expectError: false,
		},
		{
			name:        "true unmarshals to set true",
			input:       "true",
			expectSet:   true,
			expectValue: true,
			expectError: false,
		},
		{
			name:        "false unmarshals to set false",
			input:       "false",
			expectSet:   true,
			expectValue: false,
			expectError: false,
		},
		{
			name:        "string true unmarshals to set true",
			input:       `"true"`,
			expectSet:   true,
			expectValue: true,
			expectError: false,
		},
		{
			name:        "string false unmarshals to set false",
			input:       `"false"`,
			expectSet:   true,
			expectValue: false,
			expectError: false,
		},
		{
			name:        "string T unmarshals to set true",
			input:       `"T"`,
			expectSet:   true,
			expectValue: true,
			expectError: false,
		},
		{
			name:        "string F unmarshals to set false",
			input:       `"F"`,
			expectSet:   true,
			expectValue: false,
			expectError: false,
		},
		{
			name:        "string 1 unmarshals to set true",
			input:       `"1"`,
			expectSet:   true,
			expectValue: true,
			expectError: false,
		},
		{
			name:        "string 0 unmarshals to set false",
			input:       `"0"`,
			expectSet:   true,
			expectValue: false,
			expectError: false,
		},
		{
			name:        "invalid string causes error",
			input:       `"invalid"`,
			expectSet:   false,
			expectValue: false,
			expectError: true,
		},
		{
			name:        "invalid json causes error",
			input:       `{invalid}`,
			expectSet:   false,
			expectValue: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ob OptionalBool
			err := json.Unmarshal([]byte(tt.input), &ob)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ob.IsSet() != tt.expectSet {
				t.Errorf("IsSet() = %v, want %v", ob.IsSet(), tt.expectSet)
			}

			if ob.Value() != tt.expectValue {
				t.Errorf("Value() = %v, want %v", ob.Value(), tt.expectValue)
			}
		})
	}
}

func TestOptionalBool_TextMarshal(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *OptionalBool
		expected string
	}{
		{
			name:     "unset marshals to empty string",
			setup:    func() *OptionalBool { return &OptionalBool{} },
			expected: "",
		},
		{
			name:     "true marshals to true",
			setup:    func() *OptionalBool { return NewOptionalBool(true) },
			expected: "true",
		},
		{
			name:     "false marshals to false",
			setup:    func() *OptionalBool { return NewOptionalBool(false) },
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ob := tt.setup()
			data, err := ob.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText error: %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("MarshalText() = %s, want %s", string(data), tt.expected)
			}
		})
	}
}

func TestOptionalBool_TextUnmarshal(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectSet   bool
		expectValue bool
		expectError bool
	}{
		{
			name:        "empty string unmarshals to unset",
			input:       "",
			expectSet:   false,
			expectValue: false,
			expectError: false,
		},
		{
			name:        "true unmarshals to set true",
			input:       "true",
			expectSet:   true,
			expectValue: true,
			expectError: false,
		},
		{
			name:        "false unmarshals to set false",
			input:       "false",
			expectSet:   true,
			expectValue: false,
			expectError: false,
		},
		{
			name:        "TRUE unmarshals to set true",
			input:       "TRUE",
			expectSet:   true,
			expectValue: true,
			expectError: false,
		},
		{
			name:        "FALSE unmarshals to set false",
			input:       "FALSE",
			expectSet:   true,
			expectValue: false,
			expectError: false,
		},
		{
			name:        "T unmarshals to set true",
			input:       "T",
			expectSet:   true,
			expectValue: true,
			expectError: false,
		},
		{
			name:        "F unmarshals to set false",
			input:       "F",
			expectSet:   true,
			expectValue: false,
			expectError: false,
		},
		{
			name:        "1 unmarshals to set true",
			input:       "1",
			expectSet:   true,
			expectValue: true,
			expectError: false,
		},
		{
			name:        "0 unmarshals to set false",
			input:       "0",
			expectSet:   true,
			expectValue: false,
			expectError: false,
		},
		{
			name:        "whitespace trimmed",
			input:       "  true  ",
			expectSet:   true,
			expectValue: true,
			expectError: false,
		},
		{
			name:        "invalid string causes error",
			input:       "invalid",
			expectSet:   false,
			expectValue: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ob OptionalBool
			err := ob.UnmarshalText([]byte(tt.input))

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ob.IsSet() != tt.expectSet {
				t.Errorf("IsSet() = %v, want %v", ob.IsSet(), tt.expectSet)
			}

			if ob.Value() != tt.expectValue {
				t.Errorf("Value() = %v, want %v", ob.Value(), tt.expectValue)
			}
		})
	}
}

func TestOptionalBool_RoundTrip(t *testing.T) {
	t.Run("JSON round-trip", func(t *testing.T) {
		testCases := []*OptionalBool{
			NewOptionalBoolUnset(),
			NewOptionalBool(true),
			NewOptionalBool(false),
		}

		for i, original := range testCases {
			// Marshal
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("case %d: marshal error: %v", i, err)
			}

			// Unmarshal
			var roundTrip OptionalBool
			err = json.Unmarshal(data, &roundTrip)
			if err != nil {
				t.Fatalf("case %d: unmarshal error: %v", i, err)
			}

			// Compare
			if original.IsSet() != roundTrip.IsSet() {
				t.Errorf("case %d: IsSet mismatch: original=%v, roundTrip=%v",
					i, original.IsSet(), roundTrip.IsSet())
			}
			if original.Value() != roundTrip.Value() {
				t.Errorf("case %d: Value mismatch: original=%v, roundTrip=%v",
					i, original.Value(), roundTrip.Value())
			}
		}
	})

	t.Run("Text round-trip", func(t *testing.T) {
		testCases := []*OptionalBool{
			NewOptionalBoolUnset(),
			NewOptionalBool(true),
			NewOptionalBool(false),
		}

		for i, original := range testCases {
			// Marshal
			data, err := original.MarshalText()
			if err != nil {
				t.Fatalf("case %d: marshal error: %v", i, err)
			}

			// Unmarshal
			var roundTrip OptionalBool
			err = roundTrip.UnmarshalText(data)
			if err != nil {
				t.Fatalf("case %d: unmarshal error: %v", i, err)
			}

			// Compare
			if original.IsSet() != roundTrip.IsSet() {
				t.Errorf("case %d: IsSet mismatch: original=%v, roundTrip=%v",
					i, original.IsSet(), roundTrip.IsSet())
			}
			if original.Value() != roundTrip.Value() {
				t.Errorf("case %d: Value mismatch: original=%v, roundTrip=%v",
					i, original.Value(), roundTrip.Value())
			}
		}
	})
}

func TestOptionalBool_DefaultFallbacks(t *testing.T) {
	t.Run("BoolOr with different defaults", func(t *testing.T) {
		var ob OptionalBool // unset

		if ob.BoolOr(true) != true {
			t.Error("BoolOr(true) should return true for unset value")
		}
		if ob.BoolOr(false) != false {
			t.Error("BoolOr(false) should return false for unset value")
		}

		ob.Set(true)
		if ob.BoolOr(false) != true {
			t.Error("BoolOr should return actual value when set, not default")
		}

		ob.Set(false)
		if ob.BoolOr(true) != false {
			t.Error("BoolOr should return actual value when set, not default")
		}
	})
}

func TestOptionalBool_EdgeCases(t *testing.T) {
	t.Run("multiple set operations", func(t *testing.T) {
		var ob OptionalBool
		ob.Set(true)
		ob.Set(false)
		ob.Set(true)

		if !ob.IsSet() {
			t.Error("should remain set after multiple Set operations")
		}
		if !ob.Value() {
			t.Error("should have last set value")
		}
	})

	t.Run("set after unset", func(t *testing.T) {
		var ob OptionalBool
		ob.Set(true)
		ob.Unset()
		ob.Set(false)

		if !ob.IsSet() {
			t.Error("should be set after set following unset")
		}
		if ob.Value() {
			t.Error("should have correct value after set following unset")
		}
	})

	t.Run("multiple unset operations", func(t *testing.T) {
		var ob OptionalBool
		ob.Set(true)
		ob.Unset()
		ob.Unset()

		if ob.IsSet() {
			t.Error("should remain unset after multiple Unset operations")
		}
	})
}
