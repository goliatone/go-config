package config

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/goliatone/go-config/koanf/solvers"
	"github.com/knadh/koanf/v2"
)

const defaultDelimiter = "."
const defaultConfigFilepath = "config/app.json"

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
}

func New[C Validable](c C, opts ...Option[C]) (*Container[C], error) {
	mgr := &Container[C]{
		mustValidate: true,
		strictMerge:  true,
		base:         c,
		loadTimeout:  time.Second * 10,
		delimiter:    defaultDelimiter,
		configPath:   defaultConfigFilepath,
		solvers: []solvers.ConfigSolver{
			solvers.NewVariablesSolver("${", "}"),
			solvers.NewURISolver("@", "://"),
		},
	}

	for i, opt := range opts {
		err := opt(mgr)
		if err != nil {
			return nil, fmt.Errorf("failed to apply option %d: %w", i, err)
		}
	}

	for _, src := range mgr.providers {
		if err := src.Type.Valid(); err != nil {
			return nil, fmt.Errorf("invalid source type for provider %s: %w", src.Type, err)
		}
	}

	mgr.K = koanf.NewWithConf(koanf.Conf{
		Delim:       mgr.delimiter,
		StrictMerge: mgr.strictMerge,
	})

	return mgr, nil
}

func (m *Container[C]) Validate() error {
	if err := m.base.Validate(); err != nil {
		return fmt.Errorf("failed to validate config: %w", err)
	}

	return nil
}

func (m *Container[C]) Load(ctxs ...context.Context) error {
	basectx := context.Background()
	if len(ctxs) > 1 {
		basectx = ctxs[0]
	}

	ctx, cancel := context.WithTimeout(basectx, m.loadTimeout)
	defer cancel()

	sort.Slice(m.providers, func(i, j int) bool {
		return m.providers[i].Order < m.providers[j].Order
	})

	for _, source := range m.providers {
		if err := source.Load(ctx, m.K); err != nil {
			return fmt.Errorf("faield to load config from %s: %w", source.Type, err)
		}
	}

	for _, solver := range m.solvers {
		solver.Solve(m.K)
	}

	if err := m.K.Unmarshal("", m.base); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if m.mustValidate {
		if err := m.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (m *Container[C]) Raw() C {
	return m.base
}
