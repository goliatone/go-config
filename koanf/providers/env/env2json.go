package env

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tidwall/sjson"
)

// Env implements an environment variables provider.
type Env struct {
	prefix string
	delim  string
	cb     func(key string, value string) (string, any)
	out    string
}

// Provider works like built in env provider but with support for
// arrays:
// APP_DATABASE__0__PASSWORD=password_1
// APP_DATABASE__1__PASSWORD=password_2
// APP_DATABASE__2__PASSWORD=password_3
//
// Provider returns an environment variables provider that returns
// a nested map[string]any of environment variable where the
// nesting hierarchy of keys is defined by delim. For instance, the
// delim "." will convert the key `parent.child.key: 1`
// to `{parent: {child: {key: 1}}}`.
//
// If prefix is specified (case-sensitive), only the env vars with
// the prefix are captured. cb is an optional callback that takes
// a string and returns a string (the env variable name) in case
// transformations have to be applied, for instance, to lowercase
// everything, strip prefixes and replace _ with . etc.
// If the callback returns an empty string, the variable will be
// ignored.
func Provider(prefix, delim string, cb func(s string) string) *Env {
	e := &Env{
		prefix: prefix,
		delim:  delim,
		out:    "{}",
	}
	if cb != nil {
		e.cb = func(key string, value string) (string, any) {
			return cb(key), value
		}
	}
	return e
}

// ProviderWithValue works exactly the same as Provider except the callback
// takes a (key, value) with the variable name and value and allows you
// to modify both. This is useful for cases where you may want to return
// other types like a string slice instead of just a string.
func ProviderWithValue(prefix, delim string, cb func(key string, value string) (string, any)) *Env {
	return &Env{
		prefix: prefix,
		delim:  delim,
		cb:     cb,
	}
}

// ReadBytes reads the contents of a file on disk and returns the bytes.
func (e *Env) ReadBytes() ([]byte, error) {
	// Collect the environment variable keys.
	var keys []string
	for _, k := range os.Environ() {
		if e.prefix != "" {
			if strings.HasPrefix(k, e.prefix) {
				keys = append(keys, k)
			}
		} else {
			keys = append(keys, k)
		}
	}

	for _, k := range keys {
		parts := strings.SplitN(k, "=", 2)
		fmt.Printf("== var %s\n", k)

		var (
			key   string
			value any
		)

		// If there's a transformation callback,
		// run it through every key/value.
		if e.cb != nil {
			key, value = e.cb(parts[0], parts[1])
			// If the callback blanked the key, it should be omitted
			if key == "" {
				continue
			}
		} else {
			key = parts[0]
			value = parts[1]
		}
		fmt.Printf("== key %s\n", key)
		if err := e.set(key, value); err != nil {
			return []byte{}, err
		}
	}
	fmt.Println("==== done: " + e.out)
	return []byte(e.out), nil
}

func (e *Env) set(key string, value any) error {
	out, err := sjson.Set(e.out, strings.Replace(key, e.delim, ".", -1), value)
	if err != nil {
		return err
	}

	e.out = out

	return nil
}

// Read is not supported by the file provider.
func (e *Env) Read() (map[string]any, error) {
	return nil, errors.New("envextended provider does not support this method")
}
