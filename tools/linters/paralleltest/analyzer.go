// Package paralleltest flags acceptance tests that use resource.Test() instead
// of resource.ParallelTest().
//
// GEMINI.md §15.1: All acceptance tests MUST use resource.ParallelTest()
// instead of resource.Test() to allow concurrent execution and reduce CI time.
package paralleltest

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for paralleltest.
var Analyzer = &analysis.Analyzer{
	Name:     "paralleltest",
	Doc:      "Flags resource.Test() in acceptance tests; use resource.ParallelTest() instead (GEMINI.md §15.1).",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.CallExpr)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		call := n.(*ast.CallExpr)

		// Match: resource.Test(t, ...)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		if sel.Sel.Name != "Test" {
			return
		}
		// Verify the package is "resource" (terraform-plugin-testing/helper/resource).
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "resource" {
			return
		}

		pass.Reportf(call.Pos(),
			"use resource.ParallelTest() instead of resource.Test() "+
				"to enable concurrent test execution (GEMINI.md §15.1)")
	})

	return nil, nil
}
