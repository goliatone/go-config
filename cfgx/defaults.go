package cfgx

import (
	"errors"
	"reflect"
	"sync"
)

var ErrDefaultsClonerRequired = errors.New("cfgx: defaults require an explicit isolated cloner")

func WithDefaultsCloner[T any](cloner func(T) (T, error)) Option[T] {
	return func(b *builder[T]) { b.defaultsCloner = cloner }
}

type referenceID struct {
	Type             reflect.Type
	Pointer          uintptr
	Length, Capacity int
}

func referenceKey(value reflect.Value) referenceID {
	id := referenceID{Type: value.Type(), Pointer: uintptr(value.UnsafePointer())}
	if value.Kind() == reflect.Slice {
		id.Length, id.Capacity = value.Len(), value.Cap()
	}
	return id
}

// CloneDefaults isolates ordinary state, preserves opaque pointers and scalar private state.
// Reference-bearing private state requires a caller-supplied DefaultsCloner.
func CloneDefaults[T any](value T, policy OpaquePolicy) (T, error) {
	var zero T
	if err := ValidateOpaque(value, policy); err != nil {
		return zero, err
	}
	cloner := defaultsCloner{policy: policy, seen: map[referenceID]reflect.Value{}}
	cloned, err := cloner.clone(reflect.ValueOf(&value).Elem())
	if err != nil {
		return zero, err
	}
	var result T
	reflect.ValueOf(&result).Elem().Set(cloned)
	return result, nil
}

type defaultsCloner struct {
	policy OpaquePolicy
	seen   map[referenceID]reflect.Value
}

var lockerType = reflect.TypeOf((*sync.Locker)(nil)).Elem()

func (c *defaultsCloner) opaque(t reflect.Type) bool { return c.policy != nil && c.policy(t) }

func (c *defaultsCloner) privateSafe(value reflect.Value) bool {
	if c.opaque(value.Type()) {
		return value.Kind() == reflect.Pointer
	}
	switch value.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return value.IsNil()
	case reflect.UnsafePointer:
		return value.IsNil()
	case reflect.Interface:
		return value.IsNil() || c.privateSafe(value.Elem())
	case reflect.Struct:
		if reflect.PointerTo(value.Type()).Implements(lockerType) {
			return false
		}
		for i := 0; i < value.NumField(); i++ {
			if !c.privateSafe(value.Field(i)) {
				return false
			}
		}
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if !c.privateSafe(value.Index(i)) {
				return false
			}
		}
	}
	return true
}

func (c *defaultsCloner) clone(value reflect.Value) (reflect.Value, error) {
	if c.opaque(value.Type()) {
		return value, nil
	}
	out := reflect.New(value.Type()).Elem()
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return out, nil
		}
		nested, err := c.clone(value.Elem())
		if err != nil {
			return out, err
		}
		out.Set(nested)
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if value.IsNil() {
			return out, nil
		}
		id := referenceKey(value)
		if prior, ok := c.seen[id]; ok {
			return prior, nil
		}
		switch value.Kind() {
		case reflect.Pointer:
			out.Set(reflect.New(value.Type().Elem()))
			c.seen[id] = out
			nested, err := c.clone(value.Elem())
			if err != nil {
				return out, err
			}
			out.Elem().Set(nested)
		case reflect.Map:
			out.Set(reflect.MakeMapWithSize(value.Type(), value.Len()))
			c.seen[id] = out
			it := value.MapRange()
			for it.Next() {
				key, err := c.clone(it.Key())
				if err != nil {
					return out, err
				}
				item, err := c.clone(it.Value())
				if err != nil {
					return out, err
				}
				out.SetMapIndex(key, item)
			}
		case reflect.Slice:
			out.Set(reflect.MakeSlice(value.Type(), value.Len(), value.Len()))
			c.seen[id] = out
			for i := 0; i < value.Len(); i++ {
				item, err := c.clone(value.Index(i))
				if err != nil {
					return out, err
				}
				out.Index(i).Set(item)
			}
		}
	case reflect.Struct:
		if reflect.PointerTo(value.Type()).Implements(lockerType) {
			return out, ErrDefaultsClonerRequired
		}
		out.Set(value)
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).PkgPath != "" {
				if !c.privateSafe(value.Field(i)) {
					return out, ErrDefaultsClonerRequired
				}
				continue
			}
			item, err := c.clone(value.Field(i))
			if err != nil {
				return out, err
			}
			out.Field(i).Set(item)
		}
	case reflect.Array:
		for i := 0; i < value.Len(); i++ {
			item, err := c.clone(value.Index(i))
			if err != nil {
				return out, err
			}
			out.Index(i).Set(item)
		}
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		if !value.IsNil() {
			return out, ErrDefaultsClonerRequired
		}
	default:
		out.Set(value)
	}
	return out, nil
}

func validateClonerResult[T any](source, candidate T, policy OpaquePolicy) error {
	before, after := reflect.ValueOf(source), reflect.ValueOf(candidate)
	if !before.IsValid() || !after.IsValid() || before.Kind() != reflect.Pointer || after.Kind() != reflect.Pointer || before.IsNil() {
		return nil
	}
	if policy != nil && policy(before.Type()) {
		return nil
	}
	if before.Type() == after.Type() && before.Pointer() == after.Pointer() {
		return ErrDefaultsClonerRequired
	}
	return nil
}
