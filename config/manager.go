package config

import (
	"context"
	"sort"
	"time"

	"github.com/goliatone/go-config/koanf/solvers"
	"github.com/goliatone/go-config/logger"
	"github.com/goliatone/go-errors"
	"github.com/knadh/koanf/v2"
)

var (
	DefaultDelimiter      = "."
	DefaultConfigFilepath = "config/app.json"
)

type Validable interface {
	Validate() error
}

type Container[C Validable] struct {
	K            *koanf.Koanf
	base         C
	providers    []Loader
	mustValidate bool
	strictMerge  bool
	loadTimeout  time.Duration
	delimiter    string
	configPath   string
	solvers      []solvers.ConfigSolver
	logger       logger.Logger
}

func New[C Validable](c C, opts ...Option[C]) (*Container[C], error) {

	mgr := &Container[C]{
		mustValidate: true,
		strictMerge:  true,
		base:         c,
		loadTimeout:  time.Second * 10,
		delimiter:    DefaultDelimiter,
		configPath:   DefaultConfigFilepath,
		logger:       logger.NewDefaultLogger("config"),
		solvers: []solvers.ConfigSolver{
			solvers.NewVariablesSolver("${", "}"),
			solvers.NewURISolver("@", "://"),
		},
	}

	for i, opt := range opts {
		err := opt(mgr)
		if err != nil {
			mgr.logger.Error("failed to apply option", "index", i, err)
			return nil, errors.Wrap(err, errors.CategoryOperation, "failed to apply configuration option").
				WithTextCode("CONFIG_OPTION_FAILED").
				WithMetadata(map[string]any{
					"option_index":  i,
					"total_options": len(opts),
				})
		}
	}

	// providers could have been set via options
	if len(mgr.providers) == 0 && mgr.configPath != "" {
		mgr.logger.Debug("no providers, loading default...")
		f := OptionalProvider(FileProvider[C](mgr.configPath))
		p, err := f(mgr)
		if err != nil {
			mgr.logger.Error("error creating default loader", err)
			return nil, errors.Wrap(err, errors.CategoryOperation, "failed to create default file provider").
				WithTextCode("DEFAULT_PROVIDER_FAILED").
				WithMetadata(map[string]any{
					"config_path": mgr.configPath,
				})
		}
		mgr.providers = append(mgr.providers, p)
	}

	for i, src := range mgr.providers {
		if err := src.Type.Valid(); err != nil {
			mgr.logger.Error("invalid source type for provider", "src_type", src.Type, err)
			return nil, errors.Wrap(err, errors.CategoryValidation, "invalid provider source type").
				WithTextCode("INVALID_PROVIDER_TYPE").
				WithMetadata(map[string]any{
					"source_type":    string(src.Type),
					"provider_index": i,
				})
		}
	}

	mgr.K = koanf.NewWithConf(koanf.Conf{
		Delim:       mgr.delimiter,
		StrictMerge: mgr.strictMerge,
	})

	return mgr, nil
}

func (c *Container[C]) Validate() error {
	if err := c.base.Validate(); err != nil {
		c.logger.Error("failed to validate config", err)
		return errors.Wrap(err, errors.CategoryValidation, "configuration validation failed").
			WithTextCode("CONFIG_VALIDATION_FAILED")
	}
	return nil
}

func (c *Container[C]) Load(ctxs ...context.Context) error {
	bctx := context.Background()
	if len(ctxs) > 1 {
		bctx = ctxs[0]
	}

	ctx, cancel := context.WithTimeout(bctx, c.loadTimeout)
	defer cancel()

	sort.Slice(c.providers, func(i, j int) bool {
		return c.providers[i].Order < c.providers[j].Order
	})

	for i, source := range c.providers {
		c.logger.Debug("= loading source", "source_type", source.Type)
		if err := source.Load(ctx, c.K); err != nil {
			c.logger.Error("failed to load config from", "source_type", source.Type, err)
			return errors.Wrap(err, errors.CategoryOperation, "failed to load configuration from source").
				WithTextCode("CONFIG_LOAD_FAILED").
				WithMetadata(map[string]any{
					"source_type":   string(source.Type),
					"source_index":  i,
					"total_sources": len(c.providers),
				})
		}
	}

	for _, solver := range c.solvers {
		solver.Solve(c.K)
	}

	if err := c.K.Unmarshal("", c.base); err != nil {
		c.logger.Error("failed to unmarshal config", err)
		return errors.Wrap(err, errors.CategoryOperation, "failed to unmarshal configuration data").
			WithTextCode("CONFIG_UNMARSHAL_FAILED").
			WithMetadata(map[string]any{
				"delimiter":    c.delimiter,
				"strict_merge": c.strictMerge,
			})
	}

	if c.mustValidate {
		if err := c.Validate(); err != nil {
			c.logger.Error("failed to validate", err)
			return err // Already wrapped in Validate() method
		}
	}

	return nil
}

func (c *Container[C]) Raw() C {
	return c.base
}
