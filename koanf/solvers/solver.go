package solvers

import (
	"fmt"

	"github.com/knadh/koanf/v2"
)

type ConfigSolver interface {
	Solve(config *koanf.Koanf) *koanf.Koanf
}

func ToString(v any) string {
	return fmt.Sprint(v)
}

type delimiters struct {
	Start string
	End   string
}
