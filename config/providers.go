package config

import (
	"context"
	goerrors "errors"
	"os"
	"strings"
	"syscall"

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

// TODO: rename to ProviderBuilder
type ProviderBuilder[C Validable] func(*Container[C]) (Provider, error)

type ProviderType string

type Provider interface {
	Type() ProviderType
	Priority() int
	Validate() error
	Load(context.Context, *koanf.Koanf) error
}

type Loader struct {
	order        int
	providerType ProviderType
	load         func(context.Context, *koanf.Koanf) error
}

func (l *Loader) Priority() int {
	return l.order
}

func (l *Loader) Type() ProviderType {
	return l.providerType
}

func (l *Loader) Load(ctx context.Context, k *koanf.Koanf) error {
	return l.load(ctx, k)
}

func (l *Loader) Validate() error {
	return l.providerType.validate()
}

const (
	ProviderTypeDefault   ProviderType = "default"
	ProviderTypeLocalFile ProviderType = "file"
	ProviderTypeEnv       ProviderType = "env"
	ProviderTypeFlag      ProviderType = "pflag"
	ProviderTypeStruct    ProviderType = "struct"
)

type Priority int

// container.WithFileProvider("config.json", PriorityConfig.WithOffset(-10)) // 190
// container.WithFileProvider("local.json", PriorityConfig.WithOffset(10))   // 210
// container.WithStructProvider(defaults, PriorityStruct.WithOffset(5))      // 105
func (p Priority) WithOffset(offset int) Priority {
	return Priority(int(p) + offset)
}

var (
	PriorityDefaults Priority = 0
	PriorityStruct   Priority = 10
	PriorityConfig   Priority = 20
	PriorityEnv      Priority = 30
	PriorityFlags    Priority = 40
)

var (
	DefaultEnvPrefix    = "APP_"
	DefaultEnvDelimiter = "__" // so we can have composed_words
)

// containsOptionalBool checks if a map contains any OptionalBool values
func containsOptionalBool(data map[string]any) bool {
	for _, v := range data {
		switch v.(type) {
		case *OptionalBool, OptionalBool:
			return true
		case map[string]any:
			if containsOptionalBool(v.(map[string]any)) {
				return true
			}
		}
	}
	return false
}

// optionalBoolAwareProvider preserves OptionalBool types during koanf loading
type optionalBoolAwareProvider struct {
	data  map[string]any
	delim string
}

func (p *optionalBoolAwareProvider) Read() (map[string]any, error) {
	// Simply flatten the map while preserving OptionalBool types
	result := make(map[string]any)
	flattenMapWithOptionalBool("", p.data, p.delim, result)
	return result, nil
}

func (p *optionalBoolAwareProvider) ReadBytes() ([]byte, error) {
	// Not needed for our use case since we implement Read()
	return nil, nil
}

// flattenMapWithOptionalBool flattens nested maps while preserving OptionalBool types
func flattenMapWithOptionalBool(prefix string, data map[string]any, delim string, result map[string]any) {
	for k, v := range data {
		key := k
		if prefix != "" {
			key = prefix + delim + k
		}

		switch val := v.(type) {
		case map[string]any:
			// Recursively flatten nested maps
			flattenMapWithOptionalBool(key, val, delim, result)
		case *OptionalBool, OptionalBool:
			// Store OptionalBool as-is
			result[key] = val
		default:
			// All other types are stored as-is
			result[key] = val
		}
	}
}

func (s ProviderType) String() string {
	return string(s)
}

func (p ProviderType) validate() error {
	switch p {
	case ProviderTypeDefault, ProviderTypeLocalFile, ProviderTypeEnv, ProviderTypeFlag, ProviderTypeStruct:
		return nil
	default:
		return errors.New("invalid loader type", errors.CategoryValidation).
			WithTextCode("INVALID_LOADER_TYPE").
			WithMetadata(map[string]any{
				"loader_type": string(p),
				"valid_types": []string{
					string(ProviderTypeDefault),
					string(ProviderTypeLocalFile),
					string(ProviderTypeEnv),
					string(ProviderTypeFlag),
					string(ProviderTypeStruct),
				},
			})
	}
}

func DefaultValuesProvider[C Validable](def map[string]any, order ...int) ProviderBuilder[C] {
	return func(c *Container[C]) (Provider, error) {
		// use OptionalBool aware provider if any values are OptionalBool
		hasOptionalBool := containsOptionalBool(def)
		var kprovider interface {
			Read() (map[string]any, error)
			ReadBytes() ([]byte, error)
		}

		if hasOptionalBool {
			kprovider = &optionalBoolAwareProvider{data: def, delim: "."}
		} else {
			kprovider = confmap.Provider(def, ".")
		}

		prv := &Loader{
			providerType: ProviderTypeDefault,
			order:        getOrder(PriorityDefaults, order...),
			load: func(ctx context.Context, k *koanf.Koanf) error {
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

func FileProvider[C Validable](filepath string, orders ...int) ProviderBuilder[C] {
	filetype := inferConfigFiletype(filepath)

	return func(c *Container[C]) (Provider, error) {
		parser := filetype.Parser()
		kprovider := file.Provider(filepath)

		p := &Loader{
			providerType: ProviderTypeLocalFile,
			order:        getOrder(PriorityConfig, orders...),
			load: func(ctx context.Context, k *koanf.Koanf) error {
				c.logger.Debug("file provider", "filepath", filepath)
				merger := koanf.WithMergeFunc(MergeWithBooleanPrecedence)
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
func EnvProvider[C Validable](prefix, delim string, order ...int) ProviderBuilder[C] {
	return func(c *Container[C]) (Provider, error) {
		prv := &Loader{
			providerType: ProviderTypeEnv,
			order:        getOrder(PriorityEnv, order...),
			load: func(ctx context.Context, k *koanf.Koanf) error {
				parser := json.Parser()
				merger := koanf.WithMergeFunc(MergeWithBooleanPrecedence)
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

func FlagsProvider[C Validable](flagset *pflag.FlagSet, order ...int) ProviderBuilder[C] {
	return func(c *Container[C]) (Provider, error) {
		if flagset == nil {
			return &Loader{}, errors.New("flagset cannot be nil", errors.CategoryBadInput).
				WithTextCode("NIL_FLAGSET")
		}

		prv := &Loader{
			providerType: ProviderTypeFlag,
			order:        getOrder(PriorityFlags, order...),
			load: func(ctx context.Context, k *koanf.Koanf) error {
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

func StructProvider[C Validable](v Validable, order ...int) ProviderBuilder[C] {
	if v == nil {
		return func(c *Container[C]) (Provider, error) {
			return &Loader{}, errors.New("struct cannot be nil", errors.CategoryBadInput).
				WithTextCode("NIL_STRUCT")
		}
	}

	return func(c *Container[C]) (Provider, error) {
		kprv := structs.Provider(v, "koanf")

		prv := &Loader{
			providerType: ProviderTypeStruct,
			order:        getOrder(PriorityStruct, order...),
			load: func(ctx context.Context, k *koanf.Koanf) error {
				c.logger.Debug("struct provider")
				merger := koanf.WithMergeFunc(MergeWithBooleanPrecedence)
				if err := k.Load(kprv, nil, merger); err != nil {
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
			// ignore absent files but surface other errors i.e. JSON parsing blow up
			return os.IsNotExist(err) || goerrors.Is(err, syscall.ENOENT)
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
func OptionalProvider[C Validable](f ProviderBuilder[C], errIgnoreFuncs ...ErrorFilter) ProviderBuilder[C] {
	// pick the default error filter if none provided
	errIgnore := DefaultErrorFilter()
	if len(errIgnoreFuncs) > 0 {
		errIgnore = errIgnoreFuncs[0]
	}

	return func(c *Container[C]) (Provider, error) {
		baseProvider, err := f(c)
		if err != nil {
			return &Loader{}, err
		}

		p := &Loader{
			providerType: baseProvider.Type(),
			order:        getOrder(PriorityDefaults, baseProvider.Priority()),
			load: func(ctx context.Context, k *koanf.Koanf) error {
				if err := baseProvider.Load(ctx, k); !errIgnore(err) {
					return err
				}
				return nil
			},
		}
		return p, nil
	}
}

func getOrder(defaultOrder Priority, orders ...int) int {
	if len(orders) > 0 {
		return orders[0]
	}
	return int(defaultOrder)
}
