package cfgx

import (
	"errors"
	"testing"
)

func TestOpaqueAliasedSliceDefaults(t *testing.T) {
	type item struct{ Value opaqueReader }
	type cfg struct{ Short, Full []item }
	value := &opaqueFake{Text: "synthetic"}
	backing := []item{{}, {Value: value}}
	out, err := Build[cfg](map[string]any{}, WithDefaults(cfg{Short: backing[:1], Full: backing[:2]}), WithOpaqueValues[cfg](opaqueFakePolicy))
	if err != nil {
		t.Fatal(err)
	}
	if out.Full[1].Value != value {
		t.Fatal("aliased slice suffix copied an owned pointer")
	}
	out.Short[0].Value = value
	if backing[0].Value != nil {
		t.Fatal("ordinary slice backing state was not isolated")
	}
}

func TestOpaqueAggregateDestinations(t *testing.T) {
	type cfg struct{ Value any }
	value := &opaqueFake{Text: "synthetic"}
	for _, source := range []string{"defaults", "typed_input", "cloner"} {
		t.Run(source, func(t *testing.T) {
			options := []Option[cfg]{WithOpaqueValues[cfg](opaqueFakePolicy)}
			var input any = map[string]any{}
			expected := ErrDecode
			switch source {
			case "defaults":
				options = append(options, WithDefaults(cfg{Value: value}))
				expected = ErrDefaults
			case "typed_input":
				input = cfg{Value: value}
			case "cloner":
				options = append(options, WithDefaults(cfg{}), WithDefaultsCloner(func(cfg) (cfg, error) { return cfg{Value: value}, nil }))
				expected = ErrDefaults
			}
			_, err := Build[cfg](input, options...)
			if !errors.Is(err, expected) || !errors.Is(err, ErrOpaqueDestination) {
				t.Fatalf("aggregate destination escaped restrictions: %v", err)
			}
		})
	}
	_, err := Build[cfg](map[string]any{}, WithOpaqueValues[cfg](opaqueFakePolicy), WithValidator(func(c *cfg) error { c.Value = value; return nil }))
	if !errors.Is(err, ErrValidate) || !errors.Is(err, ErrOpaqueDestination) {
		t.Fatalf("validator bypassed destination checks: %v", err)
	}
}

func TestCloneDefaultsCyclesAndNil(t *testing.T) {
	var empty any
	out, err := CloneDefaults(empty, opaqueFakePolicy)
	if err != nil || out != nil {
		t.Fatalf("nil defaults failed: %v", err)
	}
	type node struct {
		Value opaqueReader
		Next  *node
	}
	value := &opaqueFake{}
	root := &node{Value: value}
	root.Next = root
	clone, err := CloneDefaults(root, opaqueFakePolicy)
	if err != nil {
		t.Fatal(err)
	}
	if clone == root || clone.Next != clone || clone.Value != value {
		t.Fatal("isolated cyclic defaults lost ownership or structure")
	}
}
