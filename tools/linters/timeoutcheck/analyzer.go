// Package timeoutcheck ensures that all CRUD methods on resources and Read
// methods on data sources wrap their context with context.WithTimeout.
// GEMINI.md §21 (Timeout Support).
//
// Methods are exempt if they contain no API calls (no StatusCode() or
// WithResponse call), such as no-op Delete methods.
package timeoutcheck

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for timeoutcheck.
var Analyzer = &analysis.Analyzer{
	Name:     "timeoutcheck",
	Doc:      "Ensures CRUD methods use context.WithTimeout for operation timeouts.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

// crudMethods are the methods that require timeout wrapping.
var crudMethods = map[string]bool{
	"Create": true,
	"Read":   true,
	"Update": true,
	"Delete": true,
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil {
			return
		}
		if !crudMethods[fn.Name.Name] {
			return
		}
		// Only check methods (have a receiver).
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		// Check if the method makes API calls. If not, it's exempt (no-op).
		if !hasAPICalls(fn.Body) {
			return
		}

		// Check if context.WithTimeout is called.
		if !hasWithTimeout(fn.Body) {
			pass.Reportf(fn.Pos(),
				"%s method must wrap context with context.WithTimeout "+
					"using the Timeouts field (see GEMINI.md §21)",
				fn.Name.Name)
		}
	})

	return nil, nil
}

// hasAPICalls checks whether the function body contains API-related calls.
// We detect this by looking for calls to methods ending in "WithResponse"
// or calls to "StatusCode()".
func hasAPICalls(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name := sel.Sel.Name
		if name == "StatusCode" || len(name) > 12 && name[len(name)-12:] == "WithResponse" {
			found = true
			return false
		}
		return true
	})
	return found
}

// hasWithTimeout checks whether the function body calls context.WithTimeout.
func hasWithTimeout(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "WithTimeout" {
			found = true
			return false
		}
		return true
	})
	return found
}
