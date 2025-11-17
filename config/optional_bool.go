package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/goliatone/go-config/cfgx"
	"github.com/mitchellh/copystructure"
)

func init() {
	copystructure.Copiers[reflect.TypeOf(OptionalBool{})] = func(v any) (any, error) {
		ob := v.(OptionalBool)
		clone := OptionalBool{}
		if ob.IsSet() {
			clone.Set(ob.Value())
		}
		return clone, nil
	}
	copystructure.Copiers[reflect.TypeOf(&OptionalBool{})] = func(v any) (any, error) {
		if v == nil {
			return (*OptionalBool)(nil), nil
		}
		ob := v.(*OptionalBool)
		clone := &OptionalBool{}
		if ob != nil && ob.IsSet() {
			clone.Set(ob.Value())
		}
		return clone, nil
	}
	cfgx.RegisterOptionalBoolType(NewOptionalBoolUnset())
}

// OptionalBool carries three states: unset, explicitly true, explicitly false.
// The zero value is intentionally unset so callers can detect omission across provider merges.
type OptionalBool struct {
	set   bool
	value bool
}

// Set updates the value and marks the option as present.
func (ob *OptionalBool) Set(v bool) {
	if ob == nil {
		return
	}
	ob.value = v
	ob.set = true
}

// Unset clears the value so callers can detect omission again.
func (ob *OptionalBool) Unset() {
	if ob == nil {
		return
	}
	ob.value = false
	ob.set = false
}

// IsSet reports whether the field was supplied by any provider.
func (ob *OptionalBool) IsSet() bool {
	if ob == nil {
		return false
	}
	return ob.set
}

// BoolOr returns the stored value when set, otherwise the supplied default.
func (ob *OptionalBool) BoolOr(def bool) bool {
	if ob == nil {
		return def
	}
	if ob.set {
		return ob.value
	}
	return def
}

// Value returns the stored value. When unset it returns false.
func (ob *OptionalBool) Value() bool {
	if ob == nil {
		return false
	}
	return ob.value
}

// ValueOK returns the stored value along with the IsSet flag.
func (ob *OptionalBool) ValueOK() (bool, bool) {
	if ob == nil {
		return false, false
	}
	return ob.value, ob.set
}

// String returns a human readable representation for debugging.
func (ob *OptionalBool) String() string {
	if ob == nil {
		return "<nil>"
	}
	if !ob.set {
		return "<unset>"
	}
	return strconv.FormatBool(ob.value)
}

// MarshalJSON satisfies json.Marshaler so file providers round-trip correctly.
func (ob *OptionalBool) MarshalJSON() ([]byte, error) {
	if ob == nil || !ob.set {
		return []byte("null"), nil
	}
	return json.Marshal(ob.value)
}

// UnmarshalJSON satisfies json.Unmarshaler so file providers can populate OptionalBool.
func (ob *OptionalBool) UnmarshalJSON(data []byte) error {
	if ob == nil {
		return fmt.Errorf("optional bool: nil receiver")
	}

	trimmed := strings.TrimSpace(string(data))
	if strings.EqualFold(trimmed, "null") || trimmed == "" {
		ob.Unset()
		return nil
	}

	var direct bool
	if err := json.Unmarshal(data, &direct); err == nil {
		ob.Set(direct)
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err != nil {
		return fmt.Errorf("optional bool: unsupported json payload %q", data)
	}
	asString = strings.TrimSpace(asString)
	if asString == "" || strings.EqualFold(asString, "null") {
		ob.Unset()
		return nil
	}
	parsed, err := parseBoolString(asString)
	if err != nil {
		return err
	}
	ob.Set(parsed)
	return nil
}

// MarshalText satisfies encoding.TextMarshaler so env/flag providers can serialize values.
func (ob *OptionalBool) MarshalText() ([]byte, error) {
	if ob == nil || !ob.set {
		return []byte(""), nil
	}
	return []byte(strconv.FormatBool(ob.value)), nil
}

// UnmarshalText satisfies encoding.TextUnmarshaler so env/flag providers can populate values.
func (ob *OptionalBool) UnmarshalText(text []byte) error {
	if ob == nil {
		return fmt.Errorf("optional bool: nil receiver")
	}
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" || strings.EqualFold(trimmed, "null") {
		ob.Unset()
		return nil
	}
	parsed, err := parseBoolString(trimmed)
	if err != nil {
		return err
	}
	ob.Set(parsed)
	return nil
}

// parseBoolString parses canonical and common boolean aliases.
func parseBoolString(s string) (bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "1", "t", "true", "y", "yes", "on":
		return true, nil
	case "0", "f", "false", "n", "no", "off":
		return false, nil
	default:
		return strconv.ParseBool(s)
	}
}

// NewOptionalBool constructs an OptionalBool that is explicitly set.
func NewOptionalBool(value bool) *OptionalBool {
	ob := &OptionalBool{}
	ob.Set(value)
	return ob
}

// NewOptionalBoolUnset constructs an OptionalBool that starts unset.
func NewOptionalBoolUnset() *OptionalBool {
	return &OptionalBool{}
}

// cloneOptionalBool makes a copy preserving set semantics. It tolerates nil input.
func cloneOptionalBool(ob *OptionalBool) *OptionalBool {
	if ob == nil {
		return nil
	}
	clone := &OptionalBool{}
	if ob.set {
		clone.Set(ob.value)
	}
	return clone
}
