# Go Config

This package is a collection of utilities, extensions, and helpers meant to ease working with different golang configuration libraries.

## Koanf

### Solvers

The solvers package provides variable post-processing for [koanf](https://github.com/knadh/koanf).

```go
var k = koanf.New(".")

func main() {
    solvers := []solvers.ConfigSolver{
        ksolvers.NewVariablesSolver("${", "}"),
        ksolvers.NewURLSolver("@", "://"),
    }

    if err := k.Load(file.Provider("config/cofig.json"), json.Parser()); err != nil {
		log.Fatalf("error loading config: %v", err)
	}

    for _, solver := range solvers {
        solver.Solve(k)
    }
}
```

#### Variable Resolution

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

#### URI Solver

##### `file`

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

##### `base64`

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

### Providers

#### Env
Enhanced environment variable provider for [koanf](https://github.com/knadh/koanf) that extends the built-in functionality with support for arrays and nested structures through environment variables.

**Features**:
- Array support through indexed environment variables
- Nested structure support using delimiters
- Prefix filtering for environment variables
- Custom key/value transformations
- Type conversion capabilities

##### Basic Usage

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
