package config

import (
	"github.com/goliatone/go-config/koanf/solvers"
	"github.com/goliatone/go-config/logger"
	"github.com/goliatone/go-errors"
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

func WithoutDefaultConfigPath[C Validable]() Option[C] {
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
		for i, factory := range factories {
			provider, err := factory(c)
			if err != nil {
				return errors.Wrap(err, errors.CategoryOperation, "failed to create loader provider").
					WithTextCode("PROVIDER_CREATION_FAILED").
					WithMetadata(map[string]any{
						"factory_index":   i,
						"total_factories": len(factories),
					})
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
