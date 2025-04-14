package config

import (
	"fmt"

	"github.com/goliatone/go-config/koanf/solvers"
	"github.com/goliatone/go-config/logger"
)

type Option[C Validable] func(c *Container[C]) error

func WithValidation[C Validable](v bool) Option[C] {
	return func(c *Container[C]) error {
		c.mustValidate = v
		return nil
	}
}

func WithConfigPath[C Validable](p string) Option[C] {
	return func(c *Container[C]) error {
		c.configPath = p
		return nil
	}
}

func WithoutDefualtConfigPath[C Validable]() Option[C] {
	return WithConfigPath[C]("")
}

func WithSolver[C Validable](srcs ...solvers.ConfigSolver) Option[C] {
	return func(c *Container[C]) error {
		c.solvers = append(c.solvers, srcs...)
		return nil
	}
}

func WithLoader[C Validable](factories ...LoaderBuilder[C]) Option[C] {
	return func(c *Container[C]) error {
		for _, factory := range factories {
			provider, err := factory(c)
			if err != nil {
				return fmt.Errorf("failed to create provider: %w", err)
			}
			c.providers = append(c.providers, provider)
		}
		return nil
	}
}

func WithLogger[C Validable](logger logger.Logger) Option[C] {
	return func(c *Container[C]) error {
		c.logger = logger
		return nil
	}
}
