# Configuration solvers

Ordinary `ConfigSolver.Solve(*koanf.Koanf)` solvers run in container order;
`ErrorReporter.Err()` is an optional error channel inspected after each solve.
Defaults are variables, URI, expression and select, with one pass. Containers
can replace that order or cap repeated passes. Final solvers run after these
passes and before typed decode. See the [container guide](../../README.md#solvers).

| Source | Contract |
| --- | --- |
| [variables.go](variables.go) | `${path}` lookup against configuration |
| [uri.go](uri.go) | File/base64/storage/include resolution, reserved prefixes and error strategy |
| [expression.go](expression.go) | Whole-value expressions, evaluator injection and error handlers |
| [select.go](select.go) | `$select`/`$default` profile projection; recoverable resolution errors |
| [solver.go](solver.go) | `ConfigSolver`, `ErrorReporter`, context-aware `FinalSolver`/`FinalSolverFunc` |

URI reads use the configured filesystem/storage resolver. Default URI errors
leave the directive unchanged; `URIErrorRemoveKey` opts into removal. Reserved
terminal prefixes are preserved in either mode. Expression errors likewise use
a configured handler; default behavior leaves the expression unchanged.

`FinalSolver.SolveContext` receives private selected input and the load deadline.
It runs once per Load, may replace terminal directives, and must not retain the
map. The host adapter owns external acquisition scopes and cleanup. Reservation
alone performs no resolution. See [terminal and opaque-value ownership](../../README.md#terminal-solving-and-caller-owned-values).
