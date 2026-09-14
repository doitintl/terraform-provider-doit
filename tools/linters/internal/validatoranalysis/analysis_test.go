package validatoranalysis

import (
	"go/token"
	"go/types"
	"testing"
)

func TestNamedType(t *testing.T) {
	t.Parallel()

	named := types.NewNamed(
		types.NewTypeName(token.NoPos, types.NewPackage("example.test/value", "value"), "Value", nil),
		types.NewStruct(nil, nil),
		nil,
	)
	tests := []struct {
		name  string
		value types.Type
		want  *types.Named
	}{
		{name: "named", value: named, want: named},
		{name: "pointer", value: types.NewPointer(named), want: named},
		{name: "basic", value: types.Typ[types.String]},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := NamedType(test.value); got != test.want {
				t.Fatalf("NamedType() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestValueKeyIncludesObjectIdentityAndSelectorPath(t *testing.T) {
	t.Parallel()

	first := types.NewVar(token.NoPos, nil, "value", types.Typ[types.String])
	shadow := types.NewVar(token.NoPos, nil, "value", types.Typ[types.String])
	tests := []struct {
		name  string
		left  ValueKey
		right ValueKey
		want  bool
	}{
		{name: "same object and path", left: ValueKey{Root: first, Path: ".Role"}, right: ValueKey{Root: first, Path: ".Role"}, want: true},
		{name: "shadowed object", left: ValueKey{Root: first}, right: ValueKey{Root: shadow}},
		{name: "different selector", left: ValueKey{Root: first, Path: ".Role"}, right: ValueKey{Root: first, Path: ".Email"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.left == test.right; got != test.want {
				t.Fatalf("ValueKey equality = %v, want %v", got, test.want)
			}
		})
	}
}
