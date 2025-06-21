package config

import (
	"context"
	goerrors "errors"
	"strings"

	"github.com/goliatone/go-config/koanf/providers/env"
	"github.com/goliatone/go-errors"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

type LoaderBuilder[C Validable] func(*Container[C]) (Loader, error)

type LoaderType string
type Loader struct {
	Order int
	Type  LoaderType
	Load  func(context.Context, *koanf.Koanf) error
}

const (
	LoaderTypeDefault   LoaderType = "default"
	LoaderTypeLocalFile LoaderType = "file"
	LoaderTypeEnv       LoaderType = "env"
	LoaderTypeFlag      LoaderType = "pflag"
	LoaderTypeStruct    LoaderType = "struct"
)

var (
	DefaultOrderDef    = 0
	DefaultOrderStruct = 10
	DefaultOrderFile   = 20
	DefaultOrderEnv    = 30
	DefaultOrderFlag   = 40
)

var (
	DefaultEnvPrefix = "APP_"
	DefaultEnvDelim  = "__"
)

func (s LoaderType) String() string {
	return string(s)
}

func (p LoaderType) Valid() error {
	switch p {
	case LoaderTypeDefault, LoaderTypeLocalFile, LoaderTypeEnv, LoaderTypeFlag, LoaderTypeStruct:
		return nil
	default:
		return errors.New("invalid loader type", errors.CategoryValidation).
			WithTextCode("INVALID_LOADER_TYPE").
			WithMetadata(map[string]any{
				"loader_type": string(p),
				"valid_types": []string{
					string(LoaderTypeDefault),
					string(LoaderTypeLocalFile),
					string(LoaderTypeEnv),
					string(LoaderTypeFlag),
					string(LoaderTypeStruct),
				},
			})
	}
}

func DefaultValues[C Validable](def map[string]any, order ...int) LoaderBuilder[C] {
	return func(c *Container[C]) (Loader, error) {
		kprovider := confmap.Provider(def, ".")

		prv := Loader{
			Type:  LoaderTypeDefault,
			Order: getOrder(DefaultOrderDef, order...),
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				if err := k.Load(kprovider, nil); err != nil {
					return errors.Wrap(err, errors.CategoryOperation, "failed to load default values").
						WithTextCode("DEFAULT_VALUES_LOAD_FAILED").
						WithMetadata(map[string]any{
							"values_count": len(def),
						})
				}
				return nil
			},
		}

		return prv, nil
	}
}

func FileProvider[C Validable](filepath string, orders ...int) LoaderBuilder[C] {
	filetype := inferConfigFiletype(filepath)

	return func(c *Container[C]) (Loader, error) {
		parser := filetype.Parser()
		kprovider := file.Provider(filepath)

		p := Loader{
			Type:  LoaderTypeLocalFile,
			Order: getOrder(DefaultOrderFile, orders...),
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				c.logger.Debug("file provider", "filepath", filepath)
				merger := koanf.WithMergeFunc(MergeIgnoringNullValues)
				if err := k.Load(kprovider, parser, merger); err != nil {
					return errors.Wrap(err, errors.CategoryOperation, "failed to load configuration from file").
						WithTextCode("FILE_LOAD_FAILED").
						WithMetadata(map[string]any{
							"filepath":  filepath,
							"file_type": string(filetype),
						})
				}
				return nil
			},
		}
		return p, nil
	}
}

// prefix string, delim string
// "APP_", "__"
func EnvProvider[C Validable](prefix, delim string, order ...int) LoaderBuilder[C] {
	return func(c *Container[C]) (Loader, error) {
		prv := Loader{
			Type:  LoaderTypeEnv,
			Order: getOrder(DefaultOrderEnv, order...),
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				parser := json.Parser()
				merger := koanf.WithMergeFunc(MergeIgnoringNullValues)
				kprov := env.Provider(prefix, ".", func(s string) string {
					return strings.Replace(strings.ToLower(
						strings.TrimPrefix(s, prefix)), delim, ".", -1)
				})

				kprov.SetLogger(c.logger)

				c.logger.Debug("env provider")
				if err := k.Load(kprov, parser, merger); err != nil {
					return errors.Wrap(err, errors.CategoryOperation, "failed to load environment variables").
						WithTextCode("ENV_LOAD_FAILED").
						WithMetadata(map[string]any{
							"prefix":    prefix,
							"delimiter": delim,
						})
				}
				return nil
			},
		}

		return prv, nil
	}
}

func FlagsProvider[C Validable](flagset *pflag.FlagSet, order ...int) LoaderBuilder[C] {
	return func(c *Container[C]) (Loader, error) {
		if flagset == nil {
			return Loader{}, errors.New("flagset cannot be nil", errors.CategoryBadInput).
				WithTextCode("NIL_FLAGSET")
		}

		prv := Loader{
			Type:  LoaderTypeFlag,
			Order: getOrder(DefaultOrderFlag, order...),
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				c.logger.Debug("flags provider")
				prv := posflag.Provider(flagset, DefaultDelimiter, k)
				if err := k.Load(prv, nil); err != nil {
					return errors.Wrap(err, errors.CategoryOperation, "failed to load configuration from posix flags").
						WithTextCode("FLAGS_LOAD_FAILED").
						WithMetadata(map[string]any{
							"delimiter": DefaultDelimiter,
						})
				}
				return nil
			},
		}

		return prv, nil
	}
}

func StructProvider[C Validable](v Validable, order ...int) LoaderBuilder[C] {
	if v == nil {
		return func(c *Container[C]) (Loader, error) {
			return Loader{}, errors.New("struct cannot be nil", errors.CategoryBadInput).
				WithTextCode("NIL_STRUCT")
		}
	}

	return func(c *Container[C]) (Loader, error) {
		kprv := structs.Provider(v, "koanf")

		prv := Loader{
			Type:  LoaderTypeStruct,
			Order: getOrder(DefaultOrderStruct, order...),
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				c.logger.Debug("struct provider")
				if err := k.Load(kprv, nil); err != nil {
					return errors.Wrap(err,
						errors.CategoryOperation,
						"failed to load configuration from struct",
					).
						WithTextCode("STRUCT_LOAD_FAILED")
				}
				return nil
			},
		}
		return prv, nil
	}
}

type ErrorFilter func(err error) bool

func DefaultErrorFilter(allowedErrors ...error) ErrorFilter {
	return func(err error) bool {
		if err == nil {
			return false
		}

		if len(allowedErrors) == 0 {
			return true
		}

		for _, allowed := range allowedErrors {
			if goerrors.Is(err, allowed) {
				return true
			}
		}
		return false
	}
}

// OptionalProvider wraps a provider so that some errors
// as defined by errIgnore are ignored
func OptionalProvider[C Validable](f LoaderBuilder[C], errIgnoreFuncs ...ErrorFilter) LoaderBuilder[C] {
	// pick the default error filter if none provided
	errIgnore := DefaultErrorFilter()
	if len(errIgnoreFuncs) > 0 {
		errIgnore = errIgnoreFuncs[0]
	}

	return func(c *Container[C]) (Loader, error) {
		baseProvider, err := f(c)
		if err != nil {
			return Loader{}, err
		}

		p := Loader{
			Type:  baseProvider.Type,
			Order: getOrder(DefaultOrderDef, baseProvider.Order),
			Load: func(ctx context.Context, k *koanf.Koanf) error {
				if err := baseProvider.Load(ctx, k); !errIgnore(err) {
					return err
				}
				return nil
			},
		}
		return p, nil
	}
}

func getOrder(defaultOrder int, orders ...int) int {
	if len(orders) > 0 {
		return orders[0]
	}
	return defaultOrder
}
