// Package constructor flags NewXxxResource() and NewXxxDataSource()
// constructor functions that don't use the &type{} return style.
//
// GEMINI.md §6:
//
//	// Use &type{} style (not new())
//	func NewLabelResource() resource.Resource {
//	    return &labelResource{}
//	}
package constructor

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for constructor.
var Analyzer = &analysis.Analyzer{
	Name:     "constructor",
	Doc:      "Ensures NewXxxResource/NewXxxDataSource constructors use &type{} style (GEMINI.md §6).",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil {
			return
		}

		name := fn.Name.Name
		if !isConstructorName(name) {
			return
		}

		// Walk the body looking for return statements.
		for _, stmt := range fn.Body.List {
			ret, ok := stmt.(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 {
				continue
			}

			result := ret.Results[0]

			// Check for new(type) — a call to the new builtin.
			if call, ok := result.(*ast.CallExpr); ok {
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "new" {
					pass.Reportf(ret.Pos(),
						"constructor %s should return &type{} instead of new(type) (GEMINI.md §6)",
						name)
				}
			}

			// Check for &type{} — this is correct, but verify it IS a composite literal
			// (not &variable which would be unusual for a constructor).
			if unary, ok := result.(*ast.UnaryExpr); ok && unary.Op == token.AND {
				if _, ok := unary.X.(*ast.CompositeLit); !ok {
					pass.Reportf(ret.Pos(),
						"constructor %s should return &type{} (composite literal), not &variable (GEMINI.md §6)",
						name)
				}
			}
		}
	})

	return nil, nil
}

// isConstructorName checks if a function name matches the constructor naming pattern.
func isConstructorName(name string) bool {
	if !strings.HasPrefix(name, "New") {
		return false
	}
	return strings.HasSuffix(name, "Resource") || strings.HasSuffix(name, "DataSource")
}
