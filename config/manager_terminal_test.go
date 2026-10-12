package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/goliatone/go-config/koanf/solvers"
	gerrors "github.com/goliatone/go-errors"
	"github.com/knadh/koanf/v2"
)

type terminalConfig struct {
	Value string `koanf:"value"`
	Mode  string `koanf:"mode"`
}

func (*terminalConfig) Validate() error { return nil }

func TestTerminalLoadOrderingAndReservation(t *testing.T) {
	cfg := &terminalConfig{}
	calls := 0
	uri := solvers.NewURISolverWithOptions("@", "://", solvers.WithURIOnErrorRemove())
	container := New(cfg).WithConfigPath("").WithSolverPasses(3).
		WithSolvers(solvers.NewVariablesSolver("${", "}"), uri, solvers.NewExpressionSolver("{{", "}}"), solvers.NewSelectSolver("$select", "$default")).
		WithReservedDirectives("@secret://").
		WithProvider(DefaultValuesProvider[*terminalConfig](map[string]any{
			"mode":  `{{ "active" }}`,
			"value": map[string]any{"$select": "${mode}", "$default": "active", "active": "@secret://selected", "discarded": "@secret://discarded"},
		})).WithFinalSolver(solvers.FinalSolverFunc(func(ctx context.Context, input map[string]any) error {
		calls++
		if input["value"] != "@secret://selected" || input["mode"] != "active" {
			return fmt.Errorf("unexpected selected input")
		}
		if _, ok := ctx.Deadline(); !ok {
			return fmt.Errorf("missing deadline")
		}
		input["value"] = "terminal"
		return nil
	}))
	if err := container.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || container.Raw().Value != "terminal" || container.Raw() == cfg || cfg.Value != "" || container.K.String("value") != "@secret://selected" {
		t.Fatal("terminal order or publication mismatch")
	}
}

func TestTerminalLoadCancellationAndSafeErrors(t *testing.T) {
	for _, mode := range []string{"cancel", "timeout", "error"} {
		t.Run(mode, func(t *testing.T) {
			cfg := &terminalConfig{Value: "prior"}
			container := New(cfg).WithConfigPath("").WithTimeout(10 * time.Millisecond)
			prior := container.K
			_ = prior.Set("value", "prior")
			ctx := context.Background()
			calls := 0
			var expected error
			if mode == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				expected = context.Canceled
			}
			if mode == "timeout" {
				expected = context.DeadlineExceeded
			}
			container.WithFinalSolver(solvers.FinalSolverFunc(func(ctx context.Context, input map[string]any) error {
				calls++
				input["value"] = "partial"
				if mode == "timeout" {
					<-ctx.Done()
					return fmt.Errorf("sensitive-data: %w", ctx.Err())
				}
				return errors.New("sensitive-data")
			}))
			err := container.Load(ctx)
			if err == nil || (expected != nil && !errors.Is(err, expected)) {
				t.Fatalf("wrong terminal error: %v", err)
			}
			var structured *gerrors.Error
			if !errors.As(err, &structured) || structured.TextCode != "CONFIG_FINAL_SOLVER_FAILED" {
				t.Fatal("wrong terminal classification")
			}
			data, _ := json.Marshal(err)
			if strings.Contains(err.Error(), "sensitive-data") || strings.Contains(string(data), "sensitive-data") {
				t.Fatal("unsafe terminal error")
			}
			if cfg.Value != "prior" || container.K != prior || prior.String("value") != "prior" {
				t.Fatal("failed load published state")
			}
			if mode == "cancel" && calls != 0 {
				t.Fatal("canceled load ran terminal solver")
			}
		})
	}
}

type reportedSolver struct{}

func (reportedSolver) Solve(k *koanf.Koanf) *koanf.Koanf { return k }
func (reportedSolver) Err() error                        { return errors.New("sensitive-data") }
func TestTerminalGenericSolverClassification(t *testing.T) {
	err := New(&terminalConfig{}).WithConfigPath("").WithSolvers(reportedSolver{}).Load(context.Background())
	var structured *gerrors.Error
	if !errors.As(err, &structured) || structured.TextCode != "CONFIG_SOLVER_FAILED" || strings.Contains(err.Error(), "sensitive-data") {
		t.Fatalf("wrong solver error: %v", err)
	}
}
