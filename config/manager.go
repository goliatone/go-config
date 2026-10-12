package config

import (
	"context"
	stderrors "errors"
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

type ValidationIssue struct {
	Stage   string
	Path    string
	Code    string
	Message string
	Cause   error
}

type ValidationReport struct {
	Issues []ValidationIssue
}

func (r *ValidationReport) Error() string {
	if r == nil || len(r.Issues) == 0 {
		return "configuration validation failed"
	}
	if len(r.Issues) == 1 {
		issue := r.Issues[0]
		if issue.Stage != "" {
			return fmt.Sprintf("%s: %s", issue.Stage, issue.Message)
		}
		return issue.Message
	}
	return fmt.Sprintf("configuration validation failed with %d issues", len(r.Issues))
}

func (r *ValidationReport) Unwrap() error {
	if r == nil || len(r.Issues) == 0 {
		return nil
	}
	causes := make([]error, 0, len(r.Issues))
	for _, issue := range r.Issues {
		if issue.Cause != nil {
			causes = append(causes, issue.Cause)
		}
	}
	if len(causes) == 0 {
		return nil
	}
	return stderrors.Join(causes...)
}

type Container[C Validable] struct {
	K                        *koanf.Koanf
	base                     C
	providers                []Provider
	mustValidate             bool
	validationMode           ValidationMode
	baseValidate             bool
	failFast                 bool
	strictDecode             bool
	defaultTransformers      bool
	globalStringTransformers []StringTransformer
	keyedStringTransformers  map[string][]StringTransformer
	normalizers              []Normalizer[C]
	validators               []Validator[C]
	strictMerge              bool
	loadTimeout              time.Duration
	delimiter                string
	configPath               string
	reservedDirectives       []string
	finalSolvers             []solvers.FinalSolver
	opaquePolicy             cfgx.OpaquePolicy
	defaultsCloner           func(C) (C, error)
	stagedLoad               bool
	solvers                  []solvers.ConfigSolver
	solverPasses             int
	expressionFunctions      map[string]ExpressionFunction
	logger                   logger.Logger

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

func (c *Container[C]) WithDefaultTransformers(enabled bool) *Container[C] {
	c.defaultTransformers = enabled
	return c
}

func (c *Container[C]) WithStringTransformer(transformers ...StringTransformer) *Container[C] {
	for _, transformer := range transformers {
		if transformer == nil {
			continue
		}
		c.globalStringTransformers = append(c.globalStringTransformers, transformer)
	}
	return c
}

func (c *Container[C]) WithStringTransformerForKey(path string, transformers ...StringTransformer) *Container[C] {
	if path == "" {
		return c
	}
	if c.keyedStringTransformers == nil {
		c.keyedStringTransformers = map[string][]StringTransformer{}
	}
	for _, transformer := range transformers {
		if transformer == nil {
			continue
		}
		c.keyedStringTransformers[path] = append(c.keyedStringTransformers[path], transformer)
	}
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

func (c *Container[C]) WithDefaultsCloner(cloner func(C) (C, error)) *Container[C] {
	c.defaultsCloner = cloner
	return c
}

func (c *Container[C]) WithOpaqueValues(policy cfgx.OpaquePolicy) *Container[C] {
	c.opaquePolicy = policy
	return c
}

func (c *Container[C]) WithReservedDirectives(prefixes ...string) *Container[C] {
	for _, prefix := range prefixes {
		if prefix != "" {
			c.reservedDirectives = append(c.reservedDirectives, prefix)
		}
	}
	return c
}

func (c *Container[C]) WithFinalSolver(finals ...solvers.FinalSolver) *Container[C] {
	for _, final := range finals {
		if final != nil {
			c.finalSolvers = append(c.finalSolvers, final)
		}
	}
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
		mustValidate:        true,
		validationMode:      ValidationSemantic,
		baseValidate:        true,
		failFast:            true,
		strictDecode:        false,
		defaultTransformers: true,
		strictMerge:         true,
		base:                c,
		delimiter:           DefaultDelimiter,
		loadTimeout:         DefaultLoadTimeout,
		configPath:          DefaultConfigFilepath,
		logger:              logger.NewDefaultLogger("config"),
		solverPasses:        1,
		solvers: []solvers.ConfigSolver{
			solvers.NewVariablesSolver("${", "}"),
			solvers.NewURISolver("@", "://"),
			solvers.NewExpressionSolver("{{", "}}"),
			solvers.NewSelectSolver("$select", "$default"),
		},
		expressionFunctions:     map[string]ExpressionFunction{},
		keyedStringTransformers: map[string][]StringTransformer{},
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
		if len(c.finalSolvers) > 0 || c.opaquePolicy != nil {
			return errors.New("configuration validation failed", errors.CategoryValidation).WithTextCode("CONFIG_VALIDATION_FAILED")
		}
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
	if len(c.finalSolvers) == 0 && c.opaquePolicy == nil {
		return c.load(ctx)
	}
	working := *c
	working.stagedLoad = true
	working.providers = append([]Provider(nil), c.providers...)
	if err := working.load(ctx); err != nil {
		return err
	}
	c.base = working.base
	c.K = working.K
	c.providers = working.providers
	return nil
}

func (c *Container[C]) load(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, c.loadTimeout)
	defer cancel()

	// reset config state i.e. so if we remove keys the are gone
	c.newConfig()

	if len(c.loaders) > 0 {
		c.providers = nil
		for i, factory := range c.loaders {
			provider, err := factory(c)
			if err != nil {
				return c.providerFailure(err, errors.CategoryOperation, "failed to create provider", "PROVIDER_CREATION_FAILED", "provider_create", map[string]any{
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
			return c.providerFailure(err, errors.CategoryOperation, "failed to create default file provider", "DEFAULT_PROVIDER_FAILED", "default_provider_create", map[string]any{
				"config_path": c.configPath,
			})
		}
		c.providers = append(c.providers, p)
	}

	// validate our providers
	for i, src := range c.providers {
		if err := src.Validate(); err != nil {
			return c.providerFailure(err, errors.CategoryValidation, "invalid provider source type", "INVALID_PROVIDER_TYPE", "provider_validate", map[string]any{
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
		if c.stagedLoad {
			c.logger.Debug("= loading source", "source_index", i)
		} else {
			c.logger.Debug("= loading source", "source_type", source.Type())
		}
		if err := source.Load(ctx, c.K); err != nil {
			return c.providerFailure(err, errors.CategoryOperation, "failed to load configuration from source", "CONFIG_LOAD_FAILED", "provider_load", map[string]any{
				"source_type":   string(source.Type()),
				"source_index":  i,
				"total_sources": len(c.providers),
			})
		}
	}

	// run all solvers
	effectiveSolvers := c.effectiveSolvers()
	if len(effectiveSolvers) > 0 {
		maxPasses := c.solverPasses
		if maxPasses < 1 {
			maxPasses = 1
		}
		for pass := 0; pass < maxPasses; pass++ {
			before, ok := snapshotConfig(c.K)
			for _, solver := range effectiveSolvers {
				solver = solvers.ReserveURIPrefixes(solver, c.reservedDirectives...)
				solver.Solve(c.K)
				if reporter, ok := solver.(solvers.ErrorReporter); ok {
					if solverErr := reporter.Err(); solverErr != nil {
						return solverFailure(solverErr, false)
					}
				}
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

	input := c.K.Raw()
	for _, final := range c.finalSolvers {
		if err := ctx.Err(); err != nil {
			return solverFailure(err, true)
		}
		if err := final.SolveContext(ctx, input); err != nil {
			return solverFailure(err, true)
		}
	}
	if c.stagedLoad {
		if err := ctx.Err(); err != nil {
			return solverFailure(err, true)
		}
	}

	// unmarshal configuration into our base struct via cfgx
	buildOpts := []cfgx.Option[C]{
		cfgx.WithDefaults(c.base),
		cfgx.WithTagName[C]("koanf"),
	}
	if c.stagedLoad {
		cloner := c.defaultsCloner
		if cloner == nil {
			cloner = func(value C) (C, error) { return cfgx.CloneDefaults(value, c.opaquePolicy) }
		}
		buildOpts = append(buildOpts, cfgx.WithDefaultsCloner[C](cloner))
	} else if c.defaultsCloner != nil {
		buildOpts = append(buildOpts, cfgx.WithDefaultsCloner[C](c.defaultsCloner))
	}
	if c.opaquePolicy != nil {
		buildOpts = append(buildOpts, cfgx.WithOpaqueValues[C](c.opaquePolicy))
	}
	if c.strictDecode {
		buildOpts = append(buildOpts, cfgx.WithStrictKeys[C]())
	}

	decoded, err := cfgx.Build[C](input, buildOpts...)
	if err != nil {
		if c.stagedLoad {
			err = cfgx.SafeBuildError(err)
		}
		return errors.Wrap(err, errors.CategoryOperation, "failed to unmarshal configuration data").
			WithTextCode("CONFIG_UNMARSHAL_FAILED").
			WithMetadata(map[string]any{
				"delimiter":     c.delimiter,
				"strict_merge":  c.strictMerge,
				"strict_decode": c.strictDecode,
			})
	}
	if c.stagedLoad {
		c.base = decoded
	} else {
		c.assignBase(decoded)
	}

	// run post-decode semantic pipeline (transform/normalize/validate).
	if err := c.runSemanticValidation(); err != nil {
		return err
	}

	if err := cfgx.ValidateOpaque(c.base, c.opaquePolicy); err != nil {
		return errors.Wrap(err, errors.CategoryValidation, "configuration validation failed").WithTextCode("CONFIG_VALIDATION_FAILED")
	}
	if c.stagedLoad {
		if err := ctx.Err(); err != nil {
			return solverFailure(err, true)
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
	if !newVal.IsValid() {
		return
	}

	if baseVal.Kind() == reflect.Pointer && newVal.Kind() == reflect.Pointer && baseVal.Type() == newVal.Type() {
		if baseVal.IsNil() || newVal.IsNil() {
			baseVal.Set(newVal)
			return
		}
		dst := baseVal.Elem()
		src := newVal.Elem()
		if dst.Kind() == reflect.Struct && src.Kind() == reflect.Struct {
			assignExportedStructFields(dst, src)
			return
		}
		dst.Set(src)
		return
	}

	if baseVal.Kind() == reflect.Struct && newVal.Kind() == reflect.Struct && baseVal.Type() == newVal.Type() {
		assignExportedStructFields(baseVal, newVal)
		return
	}

	baseVal.Set(newVal)
}

func assignExportedStructFields(dst, src reflect.Value) {
	t := dst.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		dst.Field(i).Set(src.Field(i))
	}
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

func (c *Container[C]) runSemanticValidation() error {
	report := &ValidationReport{}

	record := func(stage, path, code string, err error) error {
		if err == nil {
			return nil
		}

		message, cause := "configuration stage failed", safeContextCause(err)
		if !c.stagedLoad {
			message, cause = err.Error(), err
		}
		report.Issues = append(report.Issues, ValidationIssue{
			Stage:   stage,
			Path:    path,
			Code:    code,
			Message: message,
			Cause:   cause,
		})

		if c.failFast {
			return c.wrapValidationReport(report)
		}
		return nil
	}

	if err := c.runStringTransformers(record); err != nil {
		return err
	}

	if c.validationMode == ValidationNone {
		if len(report.Issues) > 0 {
			return c.wrapValidationReport(report)
		}
		return nil
	}

	for _, normalizer := range c.normalizers {
		if normalizer == nil {
			continue
		}
		if err := record("normalize", "", "CONFIG_NORMALIZATION_FAILED", normalizer(c.base)); err != nil {
			return err
		}
	}

	for _, validator := range c.validators {
		if validator == nil {
			continue
		}
		if err := record("validate", "", "CONFIG_VALIDATION_FAILED", validator(c.base)); err != nil {
			return err
		}
	}

	if c.baseValidate {
		if err := record("validate", "", "CONFIG_VALIDATION_FAILED", c.base.Validate()); err != nil {
			return err
		}
	}

	if len(report.Issues) > 0 {
		return c.wrapValidationReport(report)
	}

	return nil
}

func (c *Container[C]) wrapValidationReport(report *ValidationReport) error {
	if report == nil || len(report.Issues) == 0 {
		return nil
	}

	metadata := map[string]any{
		"issues_count": len(report.Issues),
	}
	first := report.Issues[0]
	if first.Stage != "" {
		metadata["stage"] = first.Stage
	}
	if first.Code != "" {
		metadata["issue_code"] = first.Code
	}

	return errors.Wrap(report, errors.CategoryValidation, "configuration validation failed").
		WithTextCode("CONFIG_VALIDATION_FAILED").
		WithMetadata(metadata)
}

func solverFailure(err error, terminal bool) error {
	if !terminal {
		var selected *solvers.SelectResolutionError
		if stderrors.As(err, &selected) {
			return errors.Wrap(err, errors.CategoryValidation, "failed to resolve select configuration").
				WithTextCode("CONFIG_SELECT_RESOLUTION_FAILED").WithMetadata(map[string]any{
				"solver": "select", "select_path": selected.SelectPath,
				"select_value": selected.SelectValue, "default_key": selected.DefaultKey,
				"failing_node_path": selected.NodePath,
			})
		}
	}
	code, stage := "CONFIG_SOLVER_FAILED", "solver"
	if terminal {
		code, stage = "CONFIG_FINAL_SOLVER_FAILED", "terminal"
	}
	safeCause := safeContextCause(err)
	failure := errors.New("configuration solver failed", errors.CategoryOperation)
	failure.Source = safeCause
	return failure.
		WithTextCode(code).WithMetadata(map[string]any{"stage": stage})
}

func safeContextCause(err error) error {
	if stderrors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if stderrors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return nil
}
