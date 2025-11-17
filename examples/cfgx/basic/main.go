package main

import (
	"fmt"
	"time"

	"github.com/goliatone/go-config/cfgx"
)

// RedisConfig showcases how a downstream package can expose a typed config while
// accepting dynamic inputs (maps, structs of functions, etc.).
type RedisConfig struct {
	Addr    string        `mapstructure:"addr"`
	DB      int           `mapstructure:"db"`
	Timeout time.Duration `mapstructure:"timeout"`
	Labels  []string      `mapstructure:"labels"`
}

func Defaults() RedisConfig {
	return RedisConfig{
		Addr:    "127.0.0.1:6379",
		Timeout: 5 * time.Second,
		Labels:  []string{"cache"},
	}
}

func (c *RedisConfig) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("redis addr is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

func main() {
	// Input can contain functions for lazy evaluation plus overrides merged at runtime.
	input := map[string]any{
		"addr": func() string { return "redis.internal:6380" },
		"labels": []any{
			func() any { return "primary" },
			"analytics",
		},
	}

	cfg, err := cfgx.Build[RedisConfig](input,
		cfgx.WithDefaults(Defaults()),
		cfgx.WithPreprocessEvalFuncs[RedisConfig](),
		cfgx.WithMerge[RedisConfig](map[string]any{"timeout": "2s"}),
		cfgx.WithValidator((*RedisConfig).Validate),
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("redis addr=%s db=%d timeout=%s labels=%v\n", cfg.Addr, cfg.DB, cfg.Timeout, cfg.Labels)
}
