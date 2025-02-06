package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/goliatone/go-config/koanf/providers/env"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

type ProviderFactory[C Validable] func(*Container[C]) (Provider, error)

type ProviderType string
type Provider struct {
	Order int
	Type  ProviderType
	Load  func(context.Context, *koanf.Koanf) error
}

const (
	ProviderTypeDefault   ProviderType = "default"
	ProviderTypeLocalFile ProviderType = "file"
	ProviderTypeEnv       ProviderType = "env"
	ProviderTypeFlag      ProviderType = "pflag"
	ProviderTypeStruct    ProviderType = "struct"
)

var (
	DefaultOrderDef      = 0
	DefaultOrderEnv      = 10
	DefaultOrderFile     = 20
	DefaultOrderFlag     = 30
	DefaultOrderStruct   = 40
	DefaultOrderOptional = 50
)

var (
	DefaultEnvPrefix = "APP_"
	DefaultEnvDelim  = "__"
)

func (s ProviderType) String() string {
	return string(s)
}

func (p ProviderType) Valid() error {
	switch p {
	case ProviderTypeDefault, ProviderTypeLocalFile, ProviderTypeEnv, ProviderTypeFlag, ProviderTypeStruct:
		return nil
	default:
		return fmt.Errorf("invalid source type: %s", p)
	}
}

func DefaultValues[C Validable](def map[string]any, order ...int) ProviderFactory[C] {
	return func(c *Container[C]) (Provider, error) {
		kprovider := confmap.Provider(def, ".")

		o := DefaultOrderDef
		if len(order) > 0 {
			o = order[0]
		}

		prv := Provider{
			Type:  ProviderTypeDefault,
			Order: o,
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				if err := k.Load(kprovider, nil); err != nil {
					return fmt.Errorf("failed to load config from posix flags: %w", err)
				}
				return nil
			},
		}

		return prv, nil
	}
}

func File[C Validable](filepath string, order ...int) ProviderFactory[C] {
	filetype := inferConfigFiletype(filepath)

	return func(c *Container[C]) (Provider, error) {
		parser := filetype.Parser()
		kprovider := file.Provider(filepath)

		o := DefaultOrderFile
		if len(order) > 0 {
			o = order[0]
		}

		prv := Provider{
			Type:  ProviderTypeLocalFile,
			Order: o,
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				if err := k.Load(kprovider, parser); err != nil {
					return fmt.Errorf("failed to load config from posix flags: %w", err)
				}
				return nil
			},
		}

		return prv, nil
	}
}

// prefix string, delim string
// "APP_", "__"
func Env[C Validable](prefix, delim string, order ...int) ProviderFactory[C] {
	return func(c *Container[C]) (Provider, error) {

		o := DefaultOrderEnv
		if len(order) > 0 {
			o = order[0]
		}

		prv := Provider{
			Type:  ProviderTypeEnv,
			Order: o,
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				prv := env.Provider(prefix, delim, func(s string) string {
					return strings.Replace(strings.ToLower(
						strings.TrimPrefix(s, prefix)), delim, ".", -1)
				})
				if err := k.Load(prv, nil); err != nil {
					return fmt.Errorf("failed to load config from posix flags: %w", err)
				}
				return nil
			},
		}

		return prv, nil
	}
}

func PFlags[C Validable](flagset *pflag.FlagSet, order ...int) ProviderFactory[C] {
	return func(c *Container[C]) (Provider, error) {
		if flagset == nil {
			return Provider{}, fmt.Errorf("flagset cannot be nil")
		}

		o := DefaultOrderFlag
		if len(order) > 0 {
			o = order[0]
		}

		prv := Provider{
			Type:  ProviderTypeFlag,
			Order: o,
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				prv := posflag.Provider(flagset, defaultDelimiter, k)
				if err := k.Load(prv, nil); err != nil {
					return fmt.Errorf("failed to load config from posix flags: %w", err)
				}
				return nil
			},
		}

		return prv, nil
	}
}

func StructProvider[C Validable](v Validable, order ...int) ProviderFactory[C] {
	if v == nil {
		return func(c *Container[C]) (Provider, error) {
			return Provider{}, fmt.Errorf("struct cannot be nil")
		}
	}

	return func(c *Container[C]) (Provider, error) {
		o := DefaultOrderStruct
		if len(order) > 0 {
			o = order[0]
		}

		kprv := structs.Provider(v, "koanf")

		prv := Provider{
			Type:  ProviderTypeStruct,
			Order: o,
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				if err := k.Load(kprv, nil); err != nil {
					return fmt.Errorf("faild to load cofig from struct: %w", err)
				}
				return nil
			},
		}
		return prv, nil
	}
}

// OptionalFactory is valuable when we might have optional sources, e.g. a file in
// a given path,
func OptionalFactory[C Validable](f ProviderFactory[C], allowedErrors ...error) ProviderFactory[C] {
	errorShouldTrigger := func(err error) bool {
		if err == nil {
			return false
		}
		if len(allowedErrors) == 0 {
			return true
		}
		for _, allowed := range allowedErrors {
			if errors.Is(err, allowed) {
				return true
			}
		}
		return false
	}

	return func(c *Container[C]) (Provider, error) {
		pprv, err := f(c)
		if err != nil {
			return Provider{}, err
		}

		prv := Provider{
			Type:  ProviderTypeStruct,
			Order: pprv.Order,
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				if err := pprv.Load(ctx, k); errorShouldTrigger(err) {
					return err
				}
				return nil
			},
		}
		return prv, nil
	}
}
