package solvers

import (
	"log"
	"strings"

	opts "github.com/goliatone/go-options"
	"github.com/knadh/koanf/v2"
)

const (
	defaultExpressionStart = "{{"
	defaultExpressionEnd   = "}}"
)

// EvalErrorHandler allows custom handling of expression evaluation errors.
// Return true to mark the error as handled.
type EvalErrorHandler func(key string, expr string, err error, cfg *koanf.Koanf) bool

type expression struct {
	delimiters *delimiters
	evaluator  opts.Evaluator
	onError    EvalErrorHandler
}

// NewExpressionSolver evaluates expressions wrapped by delimiters (default {{ }})
// using the default expr evaluator.
func NewExpressionSolver(start, end string) ConfigSolver {
	return NewExpressionSolverWithEvaluator(start, end, nil, nil)
}

// NewExpressionSolverWithEvaluator allows custom evaluator and error handler.
func NewExpressionSolverWithEvaluator(start, end string, eval opts.Evaluator, onErr EvalErrorHandler) ConfigSolver {
	if eval == nil {
		eval = opts.NewExprEvaluator()
	}
	if onErr == nil {
		onErr = OnEvalLeaveUnchanged()
	}
	start, end = normalizeExpressionDelimiters(start, end)

	return &expression{
		delimiters: &delimiters{Start: start, End: end},
		evaluator:  eval,
		onError:    onErr,
	}
}

// Solve will transform a configuration object.
func (s expression) Solve(config *koanf.Koanf) *koanf.Koanf {
	if config == nil {
		return config
	}

	c := config.All()
	for key, val := range c {
		v2, ok := val.(string)
		if !ok {
			continue
		}
		expr, ok := s.fullMatch(v2)
		if !ok {
			continue
		}

		expr = strings.TrimSpace(expr)
		result, err := s.evaluator.Evaluate(opts.RuleContext{Snapshot: config.Raw()}, expr)
		if err != nil {
			if s.onError != nil && s.onError(key, expr, err, config) {
				continue
			}
			continue
		}

		config.Set(key, result)
	}

	return config
}

func (s expression) fullMatch(input string) (string, bool) {
	if s.delimiters == nil {
		return "", false
	}
	if !strings.HasPrefix(input, s.delimiters.Start) || !strings.HasSuffix(input, s.delimiters.End) {
		return "", false
	}

	start := len(s.delimiters.Start)
	end := len(input) - len(s.delimiters.End)
	if end < start {
		return "", false
	}
	return input[start:end], true
}

func normalizeExpressionDelimiters(start, end string) (string, string) {
	if start == "" {
		start = defaultExpressionStart
	}
	if end == "" {
		end = defaultExpressionEnd
	}
	return start, end
}

// OnEvalLogAndPanic logs the error then panics.
func OnEvalLogAndPanic(logger *log.Logger) EvalErrorHandler {
	return func(key string, expr string, err error, _ *koanf.Koanf) bool {
		logWriter := logger
		if logWriter == nil {
			logWriter = log.Default()
		}
		logWriter.Printf("expression evaluation failed for %s: %s (%v)", key, expr, err)
		panic(err)
	}
}

// OnEvalLeaveUnchanged keeps the original value.
func OnEvalLeaveUnchanged() EvalErrorHandler {
	return func(_ string, _ string, _ error, _ *koanf.Koanf) bool {
		return true
	}
}

// OnEvalRemove deletes the key from the config.
func OnEvalRemove() EvalErrorHandler {
	return func(key string, _ string, _ error, cfg *koanf.Koanf) bool {
		if cfg != nil {
			cfg.Delete(key)
		}
		return true
	}
}
