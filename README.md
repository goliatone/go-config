# Go Config

This package is a collection of utilities, extensions, and helpers meant to ease configuration management for Go applications using [koanf](https://github.com/knadh/koanf).

## Installation

To install the package, run:

```sh
go get github.com/goliatone/go-config
```

**Note**: This project requires Go 1.18+ for generics support.

## Configuration Container

The configuration container is a flexible package for Go that loads configuration values from multiple sources (files, environment variables, command line flags, and in-code structs). It supports merging, validation, and variable substitution through configurable solvers.

### Features
- **Multi-Source Loading**: Load configuration from JSON, YAML, or TOML files, environment variables, command line flags, or directly from Go structs.
- **Validation**: Each configuration struct can implement a `Validate()` method to enforce required rules.
- **Flexible Merging**: Loaders are applied in a defined order so that later sources override earlier values.
- **Optional Loaders**: Easily wrap a provider so that certain errors (such as missing optional files) are ignored.
- **Variable Substitution**: Built in support for variable substitution (e.g. env vars) and URI solving.
- **Error Handling**: Structured error handling with categories and metadata for better debugging.

### Struct Tag Configuration for File Parsing

When loading from YAML or JSON files, you need to provide appropriate struct tags to map file keys to struct fields:

```go
type Config struct {
    // For YAML files with snake_case keys
    ServerPort int    `json:"server_port" yaml:"server_port"`
    APIKey     string `json:"api_key" yaml:"api_key"`

    // Nested structures
    Database struct {
        MaxConns int `json:"max_conns" yaml:"max_conns"`
    } `json:"database" yaml:"database"`
}
```

**Important**: The `koanf` tags are used internally for key paths, but you still need `json` and `yaml` tags for proper file parsing.

### Basic Example

```go
package main

import (
	"context"
	"fmt"

	"github.com/goliatone/go-config"
)

type AppConfig struct {
	Name    string `koanf:"name"`
	Env     string `koanf:"env"`
	Version string `koanf:"version"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"database"`
	Server struct {
		Port int    `koanf:"port"`
		Host string `koanf:"host"`
	} `koanf:"server"`
}

func (c AppConfig) Validate() error {
	if c.Name == "" || c.Env == "" || c.Version == "" {
		return fmt.Errorf("missing required app config values")
	}
	if c.Database.DSN == "" {
		return fmt.Errorf("missing required database DSN")
	}
	return nil
}

func main() {
	cfg := &AppConfig{}

	// Create container with default file provider (config/app.json)
	container := config.New(cfg)

	// Load configuration
	if err := container.LoadWithDefaults(); err != nil {
		panic(err)
	}

	fmt.Printf("App: %s v%s\n", cfg.Name, cfg.Version)
	fmt.Printf("Database: %s\n", cfg.Database.DSN)
}
```

### Advanced Example with Multiple Sources

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/goliatone/go-config"
	"github.com/spf13/pflag"
)

type AppConfig struct {
	Name string `koanf:"name"`
	Port int    `koanf:"port"`
	Debug bool  `koanf:"debug"`
}

func (c AppConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("app name is required")
	}
	return nil
}

func main() {
	cfg := &AppConfig{}

	// Set up command line flags
	flags := pflag.NewFlagSet("config", pflag.ContinueOnError)
	flags.String("name", "", "Application name")
	flags.Int("port", 8080, "Server port")
	flags.Bool("debug", false, "Enable debug mode")
	flags.Parse(os.Args[1:])

	// create container with multiple providers
	container := config.New(cfg).
		WithConfigPath("configs/app.yaml").
		WithProvider(
			config.DefaultValuesProvider(map[string]any{
				"name":  "MyApp",
				"port":  8080,
				"debug": false,
			}),
			config.FileProvider[*AppConfig]("configs/app.yaml"),
			config.EnvProvider[*AppConfig]("APP_", "__"),
			config.FlagsProvider[*AppConfig](flags),
		)
	if err != nil {
		panic(err)
	}

	if err := container.Load(context.Background()); err != nil {
		panic(err)
	}

	fmt.Printf("Final config: %+v\n", cfg)
}
```

### Configuration Options

```go
// Disable validation
container.WithValidation(false)

// Custom config file path
container.WithConfigPath("custom/path.json")

// Add custom logger
container.WithLogger(myLogger)

// Add variable solvers for substitution
container.WithSolver(
	solvers.NewVariablesSolver("${", "}"),
	solvers.NewURISolver("@", "://"),
)
```

### Loading Methods

The library provides multiple loading methods:

```go
// Load with explicit context (recommended)
err := container.Load(context.Background())

// LoadWithDefaults - convenience method that uses context.Background()
err := container.LoadWithDefaults()

// MustLoad - panics on error
container.MustLoad(context.Background())

// MustLoadWithDefaults - panics on error, uses context.Background()
container.MustLoadWithDefaults()
```

### Provider Order

Providers are loaded in order of their `Order` field, with higher numbers overriding lower numbers:

- **Default values**: 0
- **Struct provider**: 10
- **File provider**: 20
- **Environment provider**: 30
- **Flags provider**: 40

You can customize the order when creating providers:

```go
config.FileProvider[*AppConfig]("config.json", 15) // Custom order
```

### FileProvider with Complex Configurations

When using `FileProvider` with complex nested structures, be aware that the provider may not properly merge deeply nested values. Consider this approach:

```go
// Method 1: Using go-config FileProvider (may have issues with deep nesting)
container := config.New(cfg).
    WithProvider(
        config.FileProvider[*Config]("config.yaml"),
        config.EnvProvider[*Config]("APP_", "_"),
    )

// Method 2: Manual parsing for complex YAML (more reliable)
func LoadConfig(path string) (*Config, error) {
    cfg := &Config{}

    // Parse YAML manually first
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    if err := yaml.Unmarshal(data, cfg); err != nil {
        return nil, err
    }

    // Then apply environment overrides
    container := config.New(cfg).
        WithProvider(config.EnvProvider[*Config]("APP_", "_"))

    if err := container.Load(context.Background()); err != nil {
        return nil, err
    }

    return container.Raw(), nil
}
```

### Boolean Precedence and OptionalBool

This library provides enhanced boolean handling through the `OptionalBool` type to address the common problem where boolean precedence fails once any provider sets a value to `true`.

#### The Boolean Precedence Problem

With regular `bool` fields, the standard precedence chain (defaults→struct→file→env→flags) breaks down:

```go
type Config struct {
    EnableFeature bool `yaml:"enable_feature"`
}

// If defaults set EnableFeature = true, environment variables
// cannot override it to false due to merge logic limitations
```

#### OptionalBool Solution

The `OptionalBool` type maintains metadata about whether a value was explicitly set:

```go
import "github.com/goliatone/go-config/config"

type Config struct {
    // Use OptionalBool for fields that need proper precedence
    EnableFeature config.OptionalBool `yaml:"enable_feature" json:"enable_feature"`

    // Regular bool for fields that don't need precedence control
    InternalFlag bool `yaml:"internal_flag" json:"internal_flag"`
}

func (c *Config) setupDefaults() {
    // Set default that can be properly overridden
    c.EnableFeature = config.OptionalBool{}.Set(true)
}
```

#### When to Use OptionalBool vs bool

**Use OptionalBool when:**
- The field needs to respect full provider precedence (defaults can be overridden by any provider)
- You need to distinguish between "not set" and "explicitly false"
- The field is likely to be configured via multiple sources (file, env, flags)

**Use regular bool when:**
- The field is internal or computed (not loaded from config)
- Simple true/false without precedence concerns
- Performance is critical (OptionalBool has minimal but measurable overhead)

#### Migration Guide

To convert existing boolean fields to OptionalBool:

1. **Update struct definition:**
```go
// Before
type Config struct {
    EnableCache bool `yaml:"enable_cache"`
}

// After
type Config struct {
    EnableCache config.OptionalBool `yaml:"enable_cache"`
}
```

2. **Update default value assignment:**
```go
// Before
cfg.EnableCache = true

// After
cfg.EnableCache = config.OptionalBool{}.Set(true)
```

3. **Update value access:**
```go
// Before
if cfg.EnableCache {
    // use feature
}

// After
if cfg.EnableCache.BoolOr(false) {
    // use feature
}
```

4. **Test the migration:**
```go
func TestBooleanPrecedence(t *testing.T) {
    cfg := &Config{}

    // Test that environment can override defaults
    os.Setenv("APP_ENABLE_CACHE", "false")
    defer os.Unsetenv("APP_ENABLE_CACHE")

    container := config.New(cfg).
        WithProvider(
            config.DefaultValuesProvider(map[string]any{
                "enable_cache": true, // default is true
            }),
            config.EnvProvider[*Config]("APP_", "_"),
        )

    err := container.Load(context.Background())
    assert.NoError(t, err)

    // Should be false due to env override
    assert.False(t, cfg.EnableCache.BoolOr(true))
}
```

#### Troubleshooting Boolean Issues

**Problem**: Environment variables not overriding file/default boolean values

**Solution**: Convert the boolean field to `OptionalBool` type. Regular `bool` fields cannot distinguish between "not set" and "explicitly false", breaking precedence.

**Problem**: OptionalBool field not parsing from JSON/YAML

**Solution**: Ensure your struct has proper file format tags:
```go
type Config struct {
    // Correct - includes both yaml and json tags
    Feature config.OptionalBool `yaml:"feature" json:"feature"`
}
```

**Problem**: Getting zero values instead of defaults with OptionalBool

**Solution**: Use `BoolOr()` method with appropriate fallback:
```go
// Wrong - returns false if not explicitly set
enabled := cfg.Feature.Value()

// Correct - returns true if not set, actual value if set
enabled := cfg.Feature.BoolOr(true)
```

**Problem**: Performance concerns with OptionalBool

**Solution**: OptionalBool has minimal overhead. If performance is critical, profile first:
```go
// Benchmark test
func BenchmarkOptionalBool(b *testing.B) {
    opt := config.OptionalBool{}.Set(true)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = opt.BoolOr(false)
    }
}
```

#### Working with OptionalBool

```go
// Creating OptionalBool values
unset := config.OptionalBool{}                    // unset state
setTrue := config.OptionalBool{}.Set(true)        // explicitly true
setFalse := config.OptionalBool{}.Set(false)      // explicitly false

// Checking states
if opt.IsSet() {
    value := opt.Value() // Get actual bool value
} else {
    // Use default behavior
}

// Getting values with fallbacks
enabled := opt.BoolOr(true)  // true if unset, actual value if set
disabled := opt.BoolOr(false) // false if unset, actual value if set

// JSON/YAML marshaling works automatically
data, _ := json.Marshal(config{Feature: setTrue})
// Output: {"feature": true}
```

## Solvers

The solvers package provides variable post-processing for [koanf](https://github.com/knadh/koanf).

```go
import (
    "github.com/goliatone/go-config/solvers"
    "github.com/knadh/koanf/v2"
)

var k = koanf.New(".")

func main() {
    slvrs := []solvers.ConfigSolver{
        solvers.NewVariablesSolver("${", "}"),
        solvers.NewURLSolver("@", "://"),
    }

    if err := k.Load(file.Provider("config/cofig.json"), json.Parser()); err != nil {
		log.Fatalf("error loading config: %v", err)
	}

    for _, solver := range slvrs {
        solver.Solve(k)
    }
}
```

### Variable Resolution

Replace configuration values with references to other values in your config.

The following JSON example:

```json
{
    "app": {
        "name": "MyApp",
        "version": "1.0.0"
    },
    "database": {
        "url": "postgresql://localhost:5432",
        "name": "myapp_db"
    },
    "server": {
        "title": "${app.name} v${app.version}",
        "db_connection": "${database.url}/${database.name}"
    }
}
```

After variable resolution:

```json
{
    "app": {
        "name": "MyApp",
        "version": "1.0.0"
    },
    "database": {
        "url": "postgresql://localhost:5432",
        "name": "myapp_db"
    },
    "server": {
        "title": "MyApp v1.0.0",
        "db_connection": "postgresql://localhost:5432/myapp_db"
    }
}
```

You can use custom delimiters:

```go
// Using different variable syntax
varSolver := solvers.NewVariablesSolver("{{", "}}")
```

### URI Solver

#### `file`

The `file` solver will let you use a reference to a file and resolve the value on loading:

```json
{
    "version": "@file://verstion.text"
}
```

Will replace the reference with the contents of the **version.txt** file:

```json
{
    "version": "v0.0.1"
}
```

You can provide a custom `filesystem` implementation for the URI file solver:

```go
customFS := os.DirFS("./configs")
uriSolver := solvers.NewURISolverWithFS("@", "://", customFS)
```

You can use custom delimiters:

```go
// Using different URI syntax
uriSolver := solvers.NewURISolver("->", "://")
```

#### `base64`

The `base64` solver will let you encode a value using `base64` and solve the value on load. This is helpful in situations in which you might have characters that could break your environment variables.

```json
{
    "password": "@base64://I3B3MTI7UmFkZCRhLjI0Mw=="
}
```

Will replace the reference with the decoded value of the variable:

```json
{
    "password": "#pw12;Radd$a.243"
}
```

## Providers

### Env
Enhanced environment variable provider for [koanf](https://github.com/knadh/koanf) that extends the built in functionality with support for arrays and nested structures through environment variables.

**Features**:
- Array support through indexed environment variables
- Nested structure support using delimiters
- Prefix filtering for environment variables
- Custom key/value transformations
- Type conversion capabilities

#### Basic Usage

```go
import (
    "github.com/goliatone/go-config/env"
    "github.com/knadh/koanf/v2"
)

func main() {
    k := koanf.New(".")
    provider := Provider("APP_", "__", func(s string) string {
        return strings.ToLower(strings.Replace(s, "APP_", "", 1))
    })
    k.Load(provider, nil)
}
```

```ini
# Basic key-value
APP_ENV=development

# Nested structures
APP_SERVER__HOST=localhost
APP_SERVER__PORT=5432

# Array elements
APP_DATABASE__0__HOST=primary.db
APP_DATABASE__0__PORT=5432
APP_DATABASE__1__HOST=replica.db
APP_DATABASE__1__PORT=5432
```

This produces a JSON structure like this:

```json
{
    "env": "development",
    "server": {
        "host": "localhost",
        "port": 5432
    },
    "database": [{
        "host": "primary.db",
        "port": 5432
    },{
        "host": "replica.db",
        "port": 5432
    }]
}
```


### Debugging Configuration Loading

When configuration values aren't loading as expected:

1. **Enable Debug Logging**: The library logs debug information about which values are loaded from which sources.

2. **Check Provider Order**: Remember providers are applied in order of precedence:
   - Later providers override earlier ones
   - Environment variables (precedence 30) override files (precedence 20)

3. **Validate Struct Tags**: Ensure your struct has proper tags for the file format:
   ```go
   // Correct
   type Config struct {
       Port int `json:"port" yaml:"port"`
   }

   // Incorrect - missing file format tags
   type Config struct {
       Port int `koanf:"port"` // koanf alone isn't enough for file parsing
   }
   ```

4. **Test File Parsing Separately**:
   ```go
   // Test if your YAML/JSON parses correctly
   var cfg Config
   data, _ := os.ReadFile("config.yaml")
   err := yaml.Unmarshal(data, &cfg)
   if err != nil {
       log.Printf("YAML parsing error: %v", err)
   }
   ```

### Complete Configuration Example

Here's a complete example showing best practices for configuration loading:

```go
package config

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/goliatone/go-config/config"
    "gopkg.in/yaml.v3"
)

type AppConfig struct {
    Server struct {
        Host string `json:"host" yaml:"host"`
        Port int    `json:"port" yaml:"port"`
    } `json:"server" yaml:"server"`

    Database struct {
        DSN        string `json:"dsn" yaml:"dsn"`
        MaxConns   int    `json:"max_conns" yaml:"max_conns"`
    } `json:"database" yaml:"database"`

    Features struct {
        RateLimit bool `json:"rate_limit" yaml:"rate_limit"`
        Caching   bool `json:"caching" yaml:"caching"`
    } `json:"features" yaml:"features"`
}

func (c *AppConfig) Validate() error {
    if c.Server.Port <= 0 || c.Server.Port > 65535 {
        return fmt.Errorf("invalid server port: %d", c.Server.Port)
    }
    if c.Database.DSN == "" {
        return fmt.Errorf("database DSN is required")
    }
    return nil
}

func LoadConfig(configPath string) (*AppConfig, error) {
    cfg := &AppConfig{
        // Set defaults
        Server: struct {
            Host string `json:"host" yaml:"host"`
            Port int    `json:"port" yaml:"port"`
        }{
            Host: "localhost",
            Port: 8080,
        },
        Database: struct {
            DSN      string `json:"dsn" yaml:"dsn"`
            MaxConns int    `json:"max_conns" yaml:"max_conns"`
        }{
            MaxConns: 10,
        },
    }

    // Load from file if it exists
    if configPath != "" {
        if _, err := os.Stat(configPath); err == nil {
            data, err := os.ReadFile(configPath)
            if err != nil {
                return nil, fmt.Errorf("failed to read config: %w", err)
            }

            ext := filepath.Ext(configPath)
            switch ext {
            case ".yaml", ".yml":
                err = yaml.Unmarshal(data, cfg)
            case ".json":
                err = json.Unmarshal(data, cfg)
            default:
                return nil, fmt.Errorf("unsupported format: %s", ext)
            }

            if err != nil {
                return nil, fmt.Errorf("failed to parse config: %w", err)
            }
        }
    }

    // Apply environment overrides
    container := config.New(cfg).
        WithValidation(false). // Validate manually after loading
        WithProvider(
            config.EnvProvider[*AppConfig]("APP_", "_"),
        )

    if err := container.Load(context.Background()); err != nil {
        return nil, fmt.Errorf("failed to load env overrides: %w", err)
    }

    finalConfig := container.Raw()

    // Validate final configuration
    if err := finalConfig.Validate(); err != nil {
        return nil, fmt.Errorf("config validation failed: %w", err)
    }

    return finalConfig, nil
}
```

### Merge

#### IngoringNullValues

```go
k.Load(env.Provider(EnvPrefix, "__", func(s string) string {
    return strings.Replace(strings.ToLower(
        strings.TrimPrefix(s, EnvPrefix)), EnvLevel, ".", -1)
}), json.Parser(), koanf.WithMergeFunc(config.MergeIgnoringNullValues))
```
