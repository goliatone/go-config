package solvers

import (
	"fmt"
	"reflect"

	"github.com/knadh/koanf/v2"
)

type ConfigSolver interface {
	Solve(config *koanf.Koanf) *koanf.Koanf
}

func ToString(v any) string {
	return fmt.Sprintf("%v", reflect.ValueOf(v))
}

type delimiters struct {
	Start string
	End   string
}
