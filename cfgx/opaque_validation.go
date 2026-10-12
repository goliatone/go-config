package cfgx

import "reflect"

// ValidateOpaque checks destination types without traversing owned payloads.
func ValidateOpaque[T any](value T, policy OpaquePolicy) error {
	if policy == nil {
		return nil
	}
	type visit struct {
		referenceID
		Forbidden bool
	}
	seen := map[visit]bool{}
	var walk func(reflect.Value, bool) error
	walk = func(v reflect.Value, forbidden bool) error {
		if policy(v.Type()) {
			if v.Kind() != reflect.Pointer || forbidden {
				return ErrOpaqueDestination
			}
			return nil
		}
		if v.Kind() == reflect.Interface {
			if v.IsNil() {
				return nil
			}
			return walk(v.Elem(), forbidden || v.Type().NumMethod() == 0)
		}
		switch v.Kind() {
		case reflect.Pointer, reflect.Map, reflect.Slice:
			if v.IsNil() {
				return nil
			}
			key := visit{referenceKey(v), forbidden}
			if seen[key] {
				return nil
			}
			seen[key] = true
			switch v.Kind() {
			case reflect.Pointer:
				return walk(v.Elem(), forbidden)
			case reflect.Map:
				it := v.MapRange()
				for it.Next() {
					if err := walk(it.Key(), true); err != nil {
						return err
					}
					if err := walk(it.Value(), forbidden); err != nil {
						return err
					}
				}
			case reflect.Slice:
				for i := 0; i < v.Len(); i++ {
					if err := walk(v.Index(i), forbidden); err != nil {
						return err
					}
				}
			}
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if err := walk(v.Field(i), forbidden); err != nil {
					return err
				}
			}
		case reflect.Array:
			for i := 0; i < v.Len(); i++ {
				if err := walk(v.Index(i), forbidden); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(reflect.ValueOf(&value).Elem(), false)
}
