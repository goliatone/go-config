package config

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/goliatone/go-config/cfgx"
	"github.com/goliatone/go-config/koanf/solvers"
	"github.com/goliatone/go-config/logger"
	"github.com/goliatone/go-errors"
	"github.com/knadh/koanf/v2"
	"github.com/mitchellh/copystructure"
)

var (
	DefaultDelimiter      = "."
	DefaultConfigFilepath = "config/app.json"
	DefaultLoadTimeout    = 30 * time.Second
)

type Validable interface {
	Validate() error
}

type ValidationMode int

const (
	ValidationNone ValidationMode = iota
	ValidationSemantic
)

type Normalizer[C any] func(C) error
type Validator[C any] func(C) error

type Container[C Validable] struct {
	K              *koanf.Koanf
	base           C
	providers      []Provider
	mustValidate   bool
	validationMode ValidationMode
	baseValidate   bool
	failFast       bool
	strictDecode   bool
	normalizers    []Normalizer[C]
	validators     []Validator[C]
	strictMerge    bool
	loadTimeout    time.Duration
	delimiter      string
	configPath     string
	solvers        []solvers.ConfigSolver
	solverPasses   int
	logger         logger.Logger

	loaders []ProviderBuilder[C]
}

// WithValidation is a legacy alias for WithValidationMode.
// If both methods are used, last call wins by simple mutation order.
func (c *Container[C]) WithValidation(v bool) *Container[C] {
	if v {
		c.validationMode = ValidationSemantic
	} else {
		c.validationMode = ValidationNone
	}
	c.mustValidate = c.validationMode == ValidationSemantic
	return c
}

// WithValidationMode sets semantic validation behavior.
// If both WithValidation and WithValidationMode are used, last call wins.
func (c *Container[C]) WithValidationMode(mode ValidationMode) *Container[C] {
	switch mode {
	case ValidationNone, ValidationSemantic:
		c.validationMode = mode
	default:
		c.validationMode = ValidationSemantic
	}
	c.mustValidate = c.validationMode == ValidationSemantic
	return c
}

func (c *Container[C]) WithBaseValidate(enabled bool) *Container[C] {
	c.baseValidate = enabled
	return c
}

func (c *Container[C]) WithFailFast(enabled bool) *Container[C] {
	c.failFast = enabled
	return c
}

func (c *Container[C]) WithStrictDecode(enabled bool) *Container[C] {
	c.strictDecode = enabled
	return c
}

func (c *Container[C]) WithNormalizer(normalizers ...Normalizer[C]) *Container[C] {
	for _, normalizer := range normalizers {
		if normalizer == nil {
			continue
		}
		c.normalizers = append(c.normalizers, normalizer)
	}
	return c
}

func (c *Container[C]) WithValidator(validators ...Validator[C]) *Container[C] {
	for _, validator := range validators {
		if validator == nil {
			continue
		}
		c.validators = append(c.validators, validator)
	}
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

// WithSolvers replaces the solver list, allowing explicit ordering.
func (c *Container[C]) WithSolvers(slvrs ...solvers.ConfigSolver) *Container[C] {
	c.solvers = append([]solvers.ConfigSolver{}, slvrs...)
	return c
}

// WithSolverPasses sets the maximum number of solver passes (minimum 1).
func (c *Container[C]) WithSolverPasses(passes int) *Container[C] {
	if passes < 1 {
		passes = 1
	}
	c.solverPasses = passes
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
		mustValidate:   true,
		validationMode: ValidationSemantic,
		baseValidate:   true,
		failFast:       true,
		strictDecode:   false,
		strictMerge:    true,
		base:           c,
		delimiter:      DefaultDelimiter,
		loadTimeout:    DefaultLoadTimeout,
		configPath:     DefaultConfigFilepath,
		logger:         logger.NewDefaultLogger("config"),
		solverPasses:   1,
		solvers: []solvers.ConfigSolver{
			solvers.NewVariablesSolver("${", "}"),
			solvers.NewURISolver("@", "://"),
			solvers.NewExpressionSolver("{{", "}}"),
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
	if len(c.solvers) > 0 {
		maxPasses := c.solverPasses
		if maxPasses < 1 {
			maxPasses = 1
		}
		for pass := 0; pass < maxPasses; pass++ {
			before, ok := snapshotConfig(c.K)
			for _, solver := range c.solvers {
				solver.Solve(c.K)
			}
			if !ok {
				continue
			}
			after := c.K.Raw()
			if reflect.DeepEqual(before, after) {
				break
			}
		}
	}

	// unmarshal configuration into our base struct via cfgx
	decoded, err := cfgx.Build[C](c.K.Raw(),
		cfgx.WithDefaults(c.base),
		cfgx.WithTagName[C]("koanf"),
	)
	if err != nil {
		return errors.Wrap(err, errors.CategoryOperation, "failed to unmarshal configuration data").
			WithTextCode("CONFIG_UNMARSHAL_FAILED").
			WithMetadata(map[string]any{
				"delimiter":    c.delimiter,
				"strict_merge": c.strictMerge,
			})
	}
	c.assignBase(decoded)

	// we can now validate the resulting config object
	if c.mustValidate && c.baseValidate {
		if err := c.Validate(); err != nil {
			return err // already wrapped in Validate() method
		}
	}

	return nil
}

func (c *Container[C]) Raw() C {
	return c.base
}

func (c *Container[C]) assignBase(value C) {
	baseVal := reflect.ValueOf(&c.base).Elem()
	newVal := reflect.ValueOf(value)

	if baseVal.Kind() == reflect.Pointer && newVal.Kind() == reflect.Pointer && baseVal.Type() == newVal.Type() {
		if baseVal.IsNil() || newVal.IsNil() {
			baseVal.Set(newVal)
			return
		}
		baseVal.Elem().Set(newVal.Elem())
		return
	}
	baseVal.Set(newVal)
}

func snapshotConfig(k *koanf.Koanf) (any, bool) {
	if k == nil {
		return nil, false
	}
	raw := k.Raw()
	cloned, err := copystructure.Copy(raw)
	if err != nil {
		return raw, false
	}
	return cloned, true
}
