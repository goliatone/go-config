# Go Config

This package is a collection of utilities, extensions, and helpers meant to ease configuration management for Go applications using [koanf](https://github.com/knadh/koanf).

## Installation

To install the package, run:

```sh
go get github.com/goliatone/go-config
```

**Note**: This project requires Go 1.18+ for generics support.

## Manager

The manager container is a flexible configuration package for Go that loads configuration values from multiple sources (files, environment variables, command-line flags, and in-code structs). It supports merging, validation, and even variable substitution through configurable solvers.

### Features
- Multi-Source Loading: Load configuration from JSON, YAML, or TOML files, environment variables, command-line flags, or directly from Go structs.
- Validation: Each configuration struct can implement a `Validate()` method to enforce required rules.
- Flexible Merging: Loaders are applied in a defined order so that later sources override earlier values.
- Optional Loaders: Easily wrap a provider so that certain errors (such as missing optional files) are ignored.
- Solvers: Built-in support for variable substitution (e.g. env vars) and URI solving.
- Simple API: A single Container ties everything together, making it easy to initialize and access your configuration.

### Example

```go
package main

import (
	"context"
	"fmt"

	"github.com/goliatone/go-config/config"
)

type App struct {
	Name     string `koanf:"name"`
	Env      string `koanf:"env"`
	Version  string `koanf:"version"`
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"database"`
	Server struct {
		Env string `koanf:"env"`
	} `koanf:"server"`
	config *config.Container[*App]
}

func (c App) Validate() error {
	if c.Env == "" || c.Name == "" || c.Version == "" {
		return fmt.Errorf("missing required app config values")
	}

	if c.Database.DSN == "" {
		return fmt.Errorf("missing required database config values")
	}

	if c.Server.Env == "" {
		return fmt.Errorf("missing required server config values")
	}

	return nil
}

func main() {
	app := &App{}
	config, err := config.New(app)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	if err := config.Load(ctx); err != nil {
		panic(err)
	}

	app.config = config

	fmt.Println("====== APP ======")
	fmt.Println(app.Name)
	fmt.Println(app.Env)
	fmt.Println(app.Database.DSN)
}
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
CopycustomFS := os.DirFS("./configs")
uriSolver := solvers.NewURISolverWithFS("@", "://", customFS)
```

You can use custom delimiters:

```go
// Using different URI syntax
uriSolver := solvers.NewURISolver("->", "://")
```

#### `base64`

The `base64` solver will let you encode a value using base64 and solve the value on load. This is helpful in situations in which you might have characters that could break your environment variables.

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
Enhanced environment variable provider for [koanf](https://github.com/knadh/koanf) that extends the built-in functionality with support for arrays and nested structures through environment variables.

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


### Merge

#### IngoringNullValues

```go
k.Load(env.Provider(EnvPrefix, "__", func(s string) string {
    return strings.Replace(strings.ToLower(
        strings.TrimPrefix(s, EnvPrefix)), EnvLevel, ".", -1)
}), json.Parser(), koanf.WithMergeFunc(config.MergeIgnoringNullValues))
```
