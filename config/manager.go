package config

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/goliatone/go-config/koanf/solvers"
	"github.com/goliatone/go-config/logger"
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
			mgr.logger.Error("failed to apply option %d: %w", i, err)
			return nil, fmt.Errorf("failed to apply option %d: %w", i, err)
		}
	}

	// providers could have been set via options
	if len(mgr.providers) == 0 && mgr.configPath != "" {
		mgr.logger.Debug("no providers, loading default...")
		f := OptionalProvider(FileProvider[C](mgr.configPath))
		p, err := f(mgr)
		if err != nil {
			mgr.logger.Error("error creating default loader: %s", err)
			return nil, fmt.Errorf("error creating default loader: %w", err)
		}
		mgr.providers = append(mgr.providers, p)
	}

	for _, src := range mgr.providers {
		if err := src.Type.Valid(); err != nil {
			mgr.logger.Error("invalid source type for provider %s: %s", src.Type, err)
			return nil, fmt.Errorf("invalid source type for provider %s: %w", src.Type, err)
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
		c.logger.Error("failed to validate config: %s", err)
		return fmt.Errorf("failed to validate config: %w", err)
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

	for _, source := range c.providers {
		c.logger.Debug("= loading source: %s", source.Type)
		if err := source.Load(ctx, c.K); err != nil {
			c.logger.Error("faield to load config from %s: %s", source.Type, err)
			return fmt.Errorf("faield to load config from %s: %w", source.Type, err)
		}
	}

	for _, solver := range c.solvers {
		solver.Solve(c.K)
	}

	if err := c.K.Unmarshal("", c.base); err != nil {
		c.logger.Error("failed to unmarshal config: %s", err)
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if c.mustValidate {
		if err := c.Validate(); err != nil {
			c.logger.Error("failed to validate: %s", err)
			return err
		}
	}

	return nil
}

func (c *Container[C]) Raw() C {
	return c.base
}
