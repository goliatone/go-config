package config

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	transformStageCodeError = "transformer_error"
	transformStageCodePanic = "transformer_panic"
)

func (c *Container[C]) effectiveGlobalStringTransformers() []StringTransformer {
	transformers := make([]StringTransformer, 0, len(c.globalStringTransformers)+1)
	if c.defaultTransformers {
		transformers = append(transformers, TrimSpace)
	}
	transformers = append(transformers, c.globalStringTransformers...)
	return transformers
}

func (c *Container[C]) runStringTransformers(record func(stage, path, code string, err error) error) error {
	global := c.effectiveGlobalStringTransformers()
	if len(global) == 0 && len(c.keyedStringTransformers) == 0 {
		return nil
	}

	root := reflect.ValueOf(&c.base).Elem()
	return c.transformValue(root, "", global, record)
}

func (c *Container[C]) transformValue(
	value reflect.Value,
	path string,
	global []StringTransformer,
	record func(stage, path, code string, err error) error,
) error {
	if !value.IsValid() {
		return nil
	}

	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return nil
		}
		elem := value.Elem()
		switch elem.Kind() {
		case reflect.Struct:
			return c.transformStruct(elem, path, global, record)
		case reflect.Slice:
			if elem.Type().Elem().Kind() != reflect.String {
				return nil
			}
			return c.transformStringSlice(elem, path, path, global, record)
		default:
			return nil
		}
	case reflect.Struct:
		return c.transformStruct(value, path, global, record)
	case reflect.String:
		next, err := c.applyStringTransformers(path, path, value.String(), global, record)
		if err != nil {
			return err
		}
		value.SetString(next)
	case reflect.Slice:
		if value.Type().Elem().Kind() != reflect.String {
			return nil
		}
		return c.transformStringSlice(value, path, path, global, record)
	}

	return nil
}

func (c *Container[C]) transformStruct(
	value reflect.Value,
	path string,
	global []StringTransformer,
	record func(stage, path, code string, err error) error,
) error {
	t := value.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		name, ignore, explicitName := parseKoanfFieldTag(field)
		if ignore {
			continue
		}

		fieldValue := value.Field(i)
		nextPath := joinKeyPath(path, name)
		if field.Anonymous && !explicitName && isStructLikeField(field.Type) {
			nextPath = path
		}

		if err := c.transformValue(fieldValue, nextPath, global, record); err != nil {
			return err
		}
	}

	return nil
}

func (c *Container[C]) transformStringSlice(
	value reflect.Value,
	path string,
	issuePath string,
	global []StringTransformer,
	record func(stage, path, code string, err error) error,
) error {
	for i := 0; i < value.Len(); i++ {
		item := value.Index(i)
		if item.Kind() != reflect.String {
			continue
		}

		next, err := c.applyStringTransformers(path, fmt.Sprintf("%s[%d]", issuePath, i), item.String(), global, record)
		if err != nil {
			return err
		}
		item.SetString(next)
	}

	return nil
}

func (c *Container[C]) applyStringTransformers(
	keyPath, issuePath, value string,
	global []StringTransformer,
	record func(stage, path, code string, err error) error,
) (string, error) {
	current := value

	for _, transformer := range global {
		next, code, err := invokeStringTransformer(transformer, current)
		if err != nil {
			if recordErr := record("transform", issuePath, code, err); recordErr != nil {
				return current, recordErr
			}
			return current, nil
		}
		current = next
	}

	for _, transformer := range c.keyedStringTransformers[keyPath] {
		next, code, err := invokeStringTransformer(transformer, current)
		if err != nil {
			if recordErr := record("transform", issuePath, code, err); recordErr != nil {
				return current, recordErr
			}
			return current, nil
		}
		current = next
	}

	return current, nil
}

func invokeStringTransformer(transformer StringTransformer, value string) (out string, code string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			out = value
			code = transformStageCodePanic
			err = fmt.Errorf("string transformer panic: %v", recovered)
		}
	}()

	out, err = transformer(value)
	if err != nil {
		return value, transformStageCodeError, err
	}

	return out, "", nil
}

func parseKoanfFieldTag(field reflect.StructField) (name string, ignore bool, explicitName bool) {
	name = field.Name

	tag := field.Tag.Get("koanf")
	if tag == "" {
		return name, false, false
	}

	if tag == "-" {
		return "", true, false
	}

	parts := strings.Split(tag, ",")
	head := strings.TrimSpace(parts[0])
	if head == "-" {
		return "", true, false
	}
	if head != "" {
		return head, false, true
	}

	return name, false, false
}

func isStructLikeField(fieldType reflect.Type) bool {
	switch fieldType.Kind() {
	case reflect.Struct:
		return true
	case reflect.Pointer:
		return fieldType.Elem().Kind() == reflect.Struct
	default:
		return false
	}
}

func joinKeyPath(base, segment string) string {
	if segment == "" {
		return base
	}
	if base == "" {
		return segment
	}
	return base + "." + segment
}
