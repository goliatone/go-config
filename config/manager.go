package config

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/goliatone/go-config/koanf/solvers"
	"github.com/goliatone/go-config/logger"
	"github.com/goliatone/go-errors"
	"github.com/knadh/koanf/v2"
)

var (
	DefaultDelimiter      = "."
	DefaultConfigFilepath = "config/app.json"
	DefaultLoadTimeout    = 30 * time.Second
)

type Validable interface {
	Validate() error
}

type Container[C Validable] struct {
	K            *koanf.Koanf
	base         C
	providers    []Provider
	mustValidate bool
	strictMerge  bool
	loadTimeout  time.Duration
	delimiter    string
	configPath   string
	solvers      []solvers.ConfigSolver
	logger       logger.Logger

	loaders []ProviderBuilder[C]
}

func (c *Container[C]) WithValidation(v bool) *Container[C] {
	c.mustValidate = v
	return c
}

func (c *Container[C]) WithStrictMerge() *Container[C] {
	c.strictMerge = true
	return c
}

func (c *Container[C]) WithTimeout(timeout time.Duration) *Container[C] {
	c.loadTimeout = timeout
	return c
}

func (c *Container[C]) WithConfigPath(p string) *Container[C] {
	c.configPath = p
	return c
}

func (c *Container[C]) WithSolver(slvrs ...solvers.ConfigSolver) *Container[C] {
	c.solvers = append(c.solvers, slvrs...)
	return c
}

func (c *Container[C]) WithLogger(l logger.Logger) *Container[C] {
	c.logger = l
	return c
}

func (c *Container[C]) WithProvider(factories ...ProviderBuilder[C]) *Container[C] {
	for _, factory := range factories {
		if factory != nil {
			c.loaders = append(c.loaders, factory)
		}
	}
	return c
}

func New[C Validable](c C) *Container[C] {

	mgr := &Container[C]{
		mustValidate: true,
		strictMerge:  true,
		base:         c,
		delimiter:    DefaultDelimiter,
		loadTimeout:  DefaultLoadTimeout,
		configPath:   DefaultConfigFilepath,
		logger:       logger.NewDefaultLogger("config"),
		solvers: []solvers.ConfigSolver{
			solvers.NewVariablesSolver("${", "}"),
			solvers.NewURISolver("@", "://"),
		},
	}

	mgr.newConfig()

	return mgr
}

func (c *Container[C]) newConfig() {
	c.K = koanf.NewWithConf(koanf.Conf{
		Delim:       c.delimiter,
		StrictMerge: c.strictMerge,
	})
}

func (c *Container[C]) Validate() error {
	if err := c.base.Validate(); err != nil {
		return errors.Wrap(err, errors.CategoryValidation, "configuration validation failed").
			WithTextCode("CONFIG_VALIDATION_FAILED")
	}
	return nil
}

func (c *Container[C]) MustValidate() *Container[C] {
	if err := c.Validate(); err != nil {
		panic(err)
	}
	return c
}

func (c *Container[C]) MustLoadWithDefaults() {
	c.MustLoad(context.Background())
}

func (c *Container[C]) LoadWithDefaults() error {
	return c.Load(context.Background())
}

func (c *Container[C]) MustLoad(ctx context.Context) {
	if err := c.Load(ctx); err != nil {
		panic(fmt.Sprintf("Failed to load configuration: %v", err))
	}
}

func (c *Container[C]) Load(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.loadTimeout)
	defer cancel()

	// reset config state i.e. so if we remove keys the are gone
	c.newConfig()

	if len(c.loaders) > 0 {
		c.providers = nil
		for i, factory := range c.loaders {
			provider, err := factory(c)
			if err != nil {
				return errors.Wrap(err, errors.CategoryOperation, "failed to create provider").
					WithTextCode("PROVIDER_CREATION_FAILED").
					WithMetadata(map[string]any{
						"factory_index":   i,
						"total_factories": len(c.loaders),
					})
			}
			c.providers = append(c.providers, provider)
		}
	}

	// providers could have been set via options
	if len(c.providers) == 0 && len(c.loaders) == 0 && c.configPath != "" {
		c.logger.Debug("no providers specified, loading default file provider...")
		f := OptionalProvider(FileProvider[C](c.configPath))
		p, err := f(c)
		if err != nil {
			return errors.Wrap(err, errors.CategoryOperation, "failed to create default file provider").
				WithTextCode("DEFAULT_PROVIDER_FAILED").
				WithMetadata(map[string]any{
					"config_path": c.configPath,
				})
		}
		c.providers = append(c.providers, p)
	}

	// validate our providers
	for i, src := range c.providers {
		if err := src.Validate(); err != nil {
			return errors.Wrap(err, errors.CategoryValidation, "invalid provider source type").
				WithTextCode("INVALID_PROVIDER_TYPE").
				WithMetadata(map[string]any{
					"source_type":    string(src.Type()),
					"provider_index": i,
				})
		}
	}

	sort.Slice(c.providers, func(i, j int) bool {
		return c.providers[i].Priority() < c.providers[j].Priority()
	})

	// load providers
	for i, source := range c.providers {
		c.logger.Debug("= loading source", "source_type", source.Type())
		if err := source.Load(ctx, c.K); err != nil {
			return errors.Wrap(err, errors.CategoryOperation, "failed to load configuration from source").
				WithTextCode("CONFIG_LOAD_FAILED").
				WithMetadata(map[string]any{
					"source_type":   string(source.Type()),
					"source_index":  i,
					"total_sources": len(c.providers),
				})
		}
	}

	// run all solvers
	for _, solver := range c.solvers {
		solver.Solve(c.K)
	}

	// unmarshal configuration into our base struct
	decoderConfig := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			optionalBoolDecodeHook(),
			mapstructure.StringToTimeDurationHookFunc(),
			textUnmarshalerDecodeHook(),
		),
		Result:           c.base,
		WeaklyTypedInput: true,
	}

	if err := c.K.UnmarshalWithConf("", c.base, koanf.UnmarshalConf{DecoderConfig: decoderConfig}); err != nil {
		return errors.Wrap(err, errors.CategoryOperation, "failed to unmarshal configuration data").
			WithTextCode("CONFIG_UNMARSHAL_FAILED").
			WithMetadata(map[string]any{
				"delimiter":    c.delimiter,
				"strict_merge": c.strictMerge,
			})
	}

	// we can now validate the resulting config object
	if c.mustValidate {
		if err := c.Validate(); err != nil {
			return err // already wrapped in Validate() method
		}
	}

	return nil
}

func (c *Container[C]) Raw() C {
	return c.base
}
