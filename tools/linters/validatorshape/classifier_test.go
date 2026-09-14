package validatorshape

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestCanonicalExitBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "return", body: "return", want: true},
		{name: "continue", body: "continue", want: true},
		{name: "break", body: "break"},
		{name: "statement before return", body: "x := 1; _ = x; return"},
		{name: "empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			statement, err := parser.ParseFile(token.NewFileSet(), "shape.go", "package shape\nfunc f() { "+test.body+" }", 0)
			if err != nil {
				t.Fatalf("parse body: %v", err)
			}
			body := statement.Decls[0].(*ast.FuncDecl).Body
			if got := canonicalExitBody(body); got != test.want {
				t.Fatalf("canonicalExitBody() = %v, want %v", got, test.want)
			}
		})
	}
}
