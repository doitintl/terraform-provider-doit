package validatordefer

import "testing"

func TestSelectorPathContains(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		parent string
		child  string
		want   bool
	}{
		"root contains selector":        {child: ".ownerCount", want: true},
		"selector contains nested":      {parent: ".state", child: ".state.ownerCount", want: true},
		"selector contains index":       {parent: ".states", child: ".states[constant:0]", want: true},
		"same selector handled exactly": {parent: ".ownerCount", child: ".ownerCount"},
		"selector prefix is not parent": {parent: ".owner", child: ".ownerCount"},
		"child cannot contain parent":   {parent: ".state.ownerCount", child: ".state"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := selectorPathContains(test.parent, test.child); got != test.want {
				t.Fatalf("selectorPathContains(%q, %q) = %v, want %v", test.parent, test.child, got, test.want)
			}
		})
	}
}
