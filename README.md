# Go Config

Layered application configuration through a Koanf container, plus independent
`cfgx` builders for packages that accept loose input and return typed settings.
Use Go 1.24.10 or newer, as required by `go.mod`.

## Packages and installation

```sh
go get github.com/goliatone/go-config/config
```

| Import suffix | Responsibility |
| --- | --- |
| `/config` | Container, providers, normalization/validation, string transformers, OptionalBool and merge helpers |
| `/cfgx` | Typed build/decode pipeline without a Koanf container dependency |
| `/koanf/providers/env` | Indexed arrays, nested environment keys and key/value conversion |
| `/koanf/solvers` | Variables, URI reads/includes, expressions, profile selection and final-solver contracts |
| `/koanf/merge` | Standalone null-ignoring Koanf merge helper |
| `/logger` | Logging interface and default implementation |

There is no Go package at the module root. Import
`github.com/goliatone/go-config/config` for `config.New` and `OptionalBool`.
No package mounts routes or manages database migrations. Hosts own loading,
publication, synchronization and the lifetime of external resources.

## Container quick start

```go
package main

import (
    "context"
    "fmt"

    "github.com/goliatone/go-config/config"
)

type AppConfig struct {
    Name string `koanf:"name"`
    Port int `koanf:"port"`
    Debug config.OptionalBool `koanf:"debug"`
}

func (c *AppConfig) Validate() error {
    if c.Name == "" || c.Port <= 0 || c.Port > 65535 {
        return fmt.Errorf("name and a valid port are required")
    }
    return nil
}

func main() {
    container := config.New(&AppConfig{}).WithProvider(
        config.DefaultValuesProvider[*AppConfig](map[string]any{
            "name": "example", "port": 8080, "debug": true,
        }),
        config.EnvProvider[*AppConfig]("APP_", "__"),
    )
    if err := container.Load(context.Background()); err != nil {
        panic(err)
    }
    current := container.Raw()
    fmt.Printf("name=%s port=%d debug=%t\n", current.Name, current.Port, current.Debug.BoolOr(false))
}
```

`APP_PORT=9090` and `APP_DEBUG=false` override the defaults. Nested paths use
`__`, for example `APP_SERVER__HOST`. Read the current value through `Raw()`
after a successful load. Do not log the full configuration when it carries secrets.

### Providers and precedence

Providers are sorted by numeric priority before loading; larger numbers load
later. Equal-priority order is not a stable override contract. Use distinct
priorities when precedence matters, including multiple file providers.

| Builder | Default priority |
| --- | ---: |
| `DefaultValuesProvider[C]` | 0 |
| `StructProvider[C]` | 10 |
| `FileProvider[C]` | 20 |
| `EnvProvider[C]` | 30 |
| `FlagsProvider[C]` | 40 |

Builders accept an optional integer priority. `WithProvider` returns the
container; construction/loading errors are returned by `Load`, not a second
return value from `WithProvider`. Explicit providers replace the implicit file
fallback; they do not automatically add environment or flag layers.

```go
container.WithProvider(
    config.DefaultValuesProvider[*AppConfig](map[string]any{"name": "example", "port": 8080}),
    config.OptionalProvider(
        config.FileProvider[*AppConfig]("config/local.yaml"),
        config.DefaultErrorFilter(os.ErrNotExist),
    ),
    config.EnvProvider[*AppConfig]("APP_", "__"),
    config.FlagsProvider[*AppConfig](flags), // *pflag.FlagSet; parse it before Load
)
```

With no explicit provider, `config.New` attempts the optional default file
`config/app.json`. `WithConfigPath` changes that fallback;
`WithConfigPath("")` disables it. Default load timeout is 30 seconds; set it
with `WithTimeout`. `LoadWithDefaults` uses `context.Background`, and the
`MustLoad` variants panic on errors.

File providers parse JSON/YAML/TOML into maps, then decode with `koanf` tags.
Use those tags for typed field mapping. `json` and `yaml` tags are needed when
serializing structs or calling those decoders directly, not for the container's
file-to-map step. There is no need to manually unmarshal a file before loading it.
See [provider source](config/providers.go) and [file parsers](config/file_parser.go).

### Optional booleans and merge behavior

`config.OptionalBool` distinguishes unset, explicitly false and explicitly true.
Use `Set`, `Unset`, `IsSet`, `ValueOK` and `BoolOr` to preserve presence semantics.
`NewOptionalBool(value)` returns a set pointer; `NewOptionalBoolUnset` returns
an unset pointer. The shared decode hook is registered by importing `/config`.

`MergeWithBooleanPrecedence` preserves explicit false for ordinary bools too;
OptionalBool is useful for presence, rather than a requirement for false overrides.
The merge skips nil and empty strings/slices, recursively merges maps and copies
only explicitly set OptionalBool values. Do not assume every zero value clears
an earlier layer. [Boolean merge implementation](config/merge_boolean.go).

### Load and validation pipeline

1. Create and validate providers, sort by priority, load/merge values.
2. Run ordinary solvers in order, with capped passes.
3. Run final solvers once on selected input, when configured.
4. Decode through `cfgx.Build` using `koanf` tags.
5. Apply global string transformers, then exact-path transformers.
6. In semantic mode, run normalizers, custom validators, then `Validate()`.

Defaults: semantic validation, base `Validate()` enabled, fail-fast enabled,
strict decode disabled, global `TrimSpace` transformer enabled. The config type
must implement `Validate()` even when semantic validation is disabled.
`ValidationNone` skips normalizers/validators/base validation but still transforms.
`WithValidation(bool)` is a compatibility alias for `WithValidationMode`;
the last call wins.

```go
container.WithStrictDecode(true).
    WithFailFast(false).
    WithStringTransformerForKey("name", config.ToLower).
    WithNormalizer(func(c *AppConfig) error { return nil }).
    WithValidator(func(c *AppConfig) error { return nil })
```

Disable global trimming with `WithDefaultTransformers(false)`.
`WithBaseValidate(false)` keeps custom validators while skipping `Validate()`.
Unknown keys fail during decode, not semantic validation. Inspect ordinary
validation reports with `errors.As(err, &report)` where
`report` is `*config.ValidationReport`; inspect decode stages through
`*cfgx.StageError` and `errors.Is(err, cfgx.ErrDecode)`. Protected terminal/opaque
loads use safe diagnostics as described below.

## cfgx Builder

`cfgx.Build[T]` clones defaults, applies preprocessors in registration order,
decodes through mapstructure and shared hooks, then runs the configured validator.
It does not load files/environment or run the container's string transformers.

```go
package main

import (
    "fmt"
    "time"

    "github.com/goliatone/go-config/cfgx"
)

type ClientConfig struct {
    Address string `mapstructure:"address"`
    Timeout time.Duration `mapstructure:"timeout"`
}

func (c *ClientConfig) Validate() error {
    if c.Address == "" || c.Timeout <= 0 { return fmt.Errorf("address and timeout required") }
    return nil
}

func main() {
    current, err := cfgx.Build[ClientConfig](map[string]any{"timeout": "2s"},
        cfgx.WithDefaults(ClientConfig{Address: "localhost:6379", Timeout: time.Second}),
        cfgx.WithValidator((*ClientConfig).Validate),
    )
    if err != nil { panic(err) }
    fmt.Println(current.Address, current.Timeout)
}
```

Options include defaults (`WithDefaults`, `WithDefaultFunc`), preprocessors
(`WithPreprocess`, `WithPreprocessFunc`, `WithPreprocessEvalFuncs`, `WithMerge`),
decoder configuration (`WithDecoder`, `WithDecoderConfig`, `WithDecodeHooks`,
`WithStrictKeys`, `WithWeakTyping`, `WithTagName`) and one validator
(`WithValidator`/`WithValidatorFunc`). Specify `[T]` explicitly for generic
options when their arguments cannot infer the type. Duplicate validators produce
an option error. Default hooks handle durations and text unmarshaling;
OptionalBool support requires importing `/config` or registering a compatible type.

See [package documentation](cfgx/doc.go), [builder example](examples/cfgx/basic/main.go)
and [container example](examples/basic/main.go). Ownership options are covered below.

## Solvers

Default order is variables → URI → expression → select, with one pass.
`WithSolver` appends; `WithSolvers` replaces the list; `WithSolverPasses(n)` caps
recursive passes and stops early when the ordinary configuration is unchanged.
When replacing defaults, include `NewSelectSolver` if `$select` is needed.

- Variables resolve `${path}` against configuration keys.
- URI solving supports `@file://`, `@base64://`, `@storage://` and `@include://`.
- Expressions evaluate a whole `{{ expression }}` value; use
  `WithExpressionFunction` to register host helpers.
- Select replaces an object with its selected profile branch. `$select` accepts
  `${path}` or dotted paths; `$default` supplies a fallback branch. Missing both
  selected and default branches returns `CONFIG_SELECT_RESOLUTION_FAILED`.

```json
{
  "env": "development",
  "limits": {
    "$select": "${env}",
    "$default": "production",
    "development": {"requests": 10},
    "production": {"requests": 100}
  }
}
```

The result contains `limits.requests` from the chosen branch; control keys and
unselected branches are removed before decode. URI/expression failure strategies
are configurable; their defaults can leave unresolved input unchanged. Validate
required results explicitly. See [solver contracts](koanf/solvers/README.md).

## Terminal solving and caller-owned values

`WithFinalSolver(solvers.FinalSolver)` runs `SolveContext(context.Context, map[string]any) error`
once after ordinary solver passes and selection, before typed decode. The context includes
the load timeout. Solvers receive private selected input and must not retain that map.
`WithReservedDirectives("@secret://")` preserves terminal prefixes during built-in URI
solving, including remove-on-error; `solvers.WithURIReservedPrefixes` supports custom
URI setup. Reservation performs no parsing or I/O.

Terminal or opaque loads publish a new validated generation through `Raw()`. The seed
passed to `New` and earlier published generations stay unchanged. Read `Raw()` after a
successful Load; do not expect the seed pointer to update in this opted-in mode. `K`
remains ordinary selected input, including unresolved terminal directives; final output
is available only in typed configuration. Failure publishes neither view. Legacy loads
without these options retain their existing assignment and diagnostic behavior.

```go
container := config.New(seed).
    WithOpaqueValues(policy).
    WithFinalSolver(terminal)
if err := container.Load(ctx); err != nil {
    return err
}
current := container.Raw()
```

`cfgx.OpaquePolicy func(reflect.Type) bool` identifies caller-owned pointer types.
`Container.WithOpaqueValues(policy)` and `cfgx.WithOpaqueValues[T](policy)` retain exact
pointer identity in concrete pointer fields or assignable nonempty typed interfaces,
including collection elements. Empty-interface destinations and opaque map keys fail
safely. `cfgx.ValidateOpaque` checks the complete candidate without traversing owned
payloads, covering defaults, direct aggregate decode and callback results. Direct opaque
assignment bypasses decoder hooks; string transformers stop at registered types.

`cfgx.CloneDefaults` isolates ordinary state and preserves scalar private fields and
owned pointers without global copy hooks. Distinct slice views are traversed completely;
reference cycles are handled by the clone. Private mutable references, locks and callable
state fail with `cfgx.ErrDefaultsClonerRequired`. Use `Container.WithDefaultsCloner` or
`cfgx.WithDefaultsCloner[T]` to supply an explicit cloner for such configurations. It must
return a distinct candidate root, isolate every mutable ordinary/private reference,
preserve validation behavior and retain owned pointer identity. Cloners are trusted code;
they must not mutate their input, acquire secrets, or retain failed candidates.

The external adapter wraps every Load in a new acquisition scope. Destroy newly acquired
values on any load/decode/validation failure; on success hand over the scope and release
the prior generation only after its consumers finish. Direct Load performs no cleanup.
Borrowed bytes must not outlive their owned values. Semantic callbacks must not mutate
retained owned state or retain failed candidates. go-config never acquires or destroys
unknown values. Concurrent load/read synchronization remains the caller's responsibility.

Opaque policies must be stable, side-effect-free type checks, for example
`func(t reflect.Type) bool { return t == reflect.TypeOf((*OwnedValue)(nil)) }`.
Owned values enter only through terminal maps or typed defaults, never ordinary providers
or solvers. Opaque cfgx builds reject preprocessors. Protected provider failures preserve
only safe stage/index metadata and cancellation sentinels, omitting raw provider causes
and source labels. Decode and semantic diagnostics likewise omit foreign causes;
`cfgx.SafeBuildError` retains known stage/ownership sentinels. Generic and terminal solver
errors have distinct CONFIG_SOLVER_FAILED/CONFIG_FINAL_SOLVER_FAILED codes; select
errors retain CONFIG_SELECT_RESOLUTION_FAILED.
