package config

import (
	"encoding/json"
	"strconv"
	"strings"
)

// OptionalBool represents a tri-state boolean value that can be set, unset, or have a value.
// The zero value represents an unset state, allowing proper precedence handling in configuration merging.
// This type enables distinguishing between explicitly set false values and unset values,
// which is crucial for proper boolean precedence across configuration providers.
type OptionalBool struct {
	value *bool
}

// Set sets the OptionalBool to the given boolean value.
func (ob *OptionalBool) Set(value bool) {
	ob.value = &value
}

// Unset clears the OptionalBool, making it unset.
func (ob *OptionalBool) Unset() {
	ob.value = nil
}

// IsSet returns true if the OptionalBool has been explicitly set to a value.
func (ob *OptionalBool) IsSet() bool {
	return ob.value != nil
}

// Value returns the boolean value if set, or false if unset.
func (ob *OptionalBool) Value() bool {
	if ob.value == nil {
		return false
	}
	return *ob.value
}

// BoolOr returns the boolean value if set, or the provided default if unset.
func (ob *OptionalBool) BoolOr(defaultValue bool) bool {
	if ob.value == nil {
		return defaultValue
	}
	return *ob.value
}

// String returns a string representation of the OptionalBool.
func (ob *OptionalBool) String() string {
	if ob.value == nil {
		return "<unset>"
	}
	return strconv.FormatBool(*ob.value)
}

// MarshalJSON implements json.Marshaler interface.
// Unset values are marshaled as null.
func (ob *OptionalBool) MarshalJSON() ([]byte, error) {
	if ob.value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*ob.value)
}

// UnmarshalJSON implements json.Unmarshaler interface.
// Accepts boolean values, null (for unset), and string representations of booleans.
func (ob *OptionalBool) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		ob.Unset()
		return nil
	}

	var value bool
	if err := json.Unmarshal(data, &value); err != nil {
		// Try to parse as string
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		parsed, err := parseBoolString(str)
		if err != nil {
			return err
		}
		ob.Set(parsed)
		return nil
	}

	ob.Set(value)
	return nil
}

// MarshalText implements encoding.TextMarshaler interface.
// Unset values are marshaled as empty string.
func (ob *OptionalBool) MarshalText() ([]byte, error) {
	if ob.value == nil {
		return []byte(""), nil
	}
	return []byte(strconv.FormatBool(*ob.value)), nil
}

// UnmarshalText implements encoding.TextUnmarshaler interface.
// Accepts boolean string representations, with empty string meaning unset.
func (ob *OptionalBool) UnmarshalText(data []byte) error {
	text := string(data)
	if text == "" {
		ob.Unset()
		return nil
	}

	parsed, err := parseBoolString(text)
	if err != nil {
		return err
	}
	ob.Set(parsed)
	return nil
}

// parseBoolString parses various string representations of booleans.
// Supports standard strconv.ParseBool plus common aliases.
func parseBoolString(s string) (bool, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	// Handle empty string as unset - but this should be handled by caller
	if s == "" {
		return false, nil
	}

	// Use standard library parsing which handles:
	// "1", "t", "T", "TRUE", "true", "True"  -> true
	// "0", "f", "F", "FALSE", "false", "False" -> false
	return strconv.ParseBool(s)
}

// NewOptionalBool creates a new OptionalBool with the given value.
func NewOptionalBool(value bool) *OptionalBool {
	ob := &OptionalBool{}
	ob.Set(value)
	return ob
}

// NewOptionalBoolUnset creates a new unset OptionalBool.
func NewOptionalBoolUnset() *OptionalBool {
	return &OptionalBool{}
}
