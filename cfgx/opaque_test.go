package cfgx

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/go-viper/mapstructure/v2"
)

type opaqueFake struct{ Text string }

func (*opaqueFake) String() string { return "sensitive-data" }
func (*opaqueFake) Read()          {}

type opaqueReader interface{ Read() }

func opaqueFakePolicy(t reflect.Type) bool { return t == reflect.TypeOf((*opaqueFake)(nil)) }

func TestOpaqueBuildIdentityAndRejection(t *testing.T) {
	v := &opaqueFake{Text: "payload"}
	hooks := 0
	type supported struct {
		Direct *opaqueFake
		Reader opaqueReader
		List   []*opaqueFake
		Map    map[string]*opaqueFake
	}
	cfg, err := Build[supported](map[string]any{"Direct": v, "Reader": v, "List": []any{v}, "Map": map[string]any{"value": v}}, WithOpaqueValues[supported](opaqueFakePolicy), WithStrictKeys[supported](), WithDecodeHooks[supported](mapstructure.DecodeHookFuncType(func(from, to reflect.Type, data any) (any, error) {
		if opaqueFakePolicy(from) {
			hooks++
		}
		return data, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Direct != v || cfg.Reader != v || cfg.List[0] != v || cfg.Map["value"] != v || hooks != 0 {
		t.Fatal("owned values traversed decode hooks or lost identity")
	}
	type unsupported struct {
		Text    string
		Generic any
		Value   opaqueFake
	}
	for _, key := range []string{"Text", "Generic", "Value"} {
		_, err := Build[unsupported](map[string]any{key: v}, WithOpaqueValues[unsupported](opaqueFakePolicy))
		if !errors.Is(err, ErrDecode) || !errors.Is(err, ErrOpaqueDestination) {
			t.Fatalf("wrong opaque rejection: %v", err)
		}
		encoded, _ := json.Marshal(err)
		if strings.Contains(err.Error(), "sensitive-data") || strings.Contains(string(encoded), "sensitive-data") {
			t.Fatal("decoder retained payload")
		}
	}
	_, err = Build[supported](map[string]any{"Direct": "sensitive-data"}, WithOpaqueValues[supported](opaqueFakePolicy))
	if !errors.Is(err, ErrOpaqueDestination) {
		t.Fatal("ordinary data constructed an opaque type")
	}
}

func TestOpaqueBuildSafeCallbackErrors(t *testing.T) {
	_, err := Build[struct{}](nil, WithOpaqueValues[struct{}](opaqueFakePolicy), WithValidatorFunc(func(struct{}) error { return errors.New("sensitive-data") }))
	if !errors.Is(err, ErrValidate) || strings.Contains(err.Error(), "sensitive-data") {
		t.Fatal("unsafe validation error")
	}
	_, err = Build[struct{}](nil, WithOpaqueValues[struct{}](opaqueFakePolicy), WithPreprocessEvalFuncs[struct{}]())
	if !errors.Is(err, ErrOption) {
		t.Fatal("opaque input allowed generic preprocessing")
	}
}
