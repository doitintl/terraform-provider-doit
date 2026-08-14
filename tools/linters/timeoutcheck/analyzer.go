// Package timeoutcheck enforces two things about timeout handling in CRUD
// methods.
//
// First, all CRUD methods on resources and Read methods on data sources must
// wrap their context with context.WithTimeout. Methods are exempt if they
// contain no API calls (no StatusCode() or WithResponse call), such as no-op
// Delete methods.
//
// Second, the default passed to Timeouts.Create/Read/Update/Delete must be a
// named constant from internal/provider/timeouts.go rather than a literal
// duration. Literals had drifted into 136 call sites across 84 files, which made
// the defaults impossible to change in one place and let the timeout ordering
// invariant documented in timeouts.go be violated silently.
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

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	checkWithTimeout(pass, insp)
	checkTimeoutDefaults(pass, insp)

	return nil, nil
}

// checkTimeoutDefaults reports literal durations passed as the default to
// Timeouts.Create/Read/Update/Delete. The default must be a named constant so
// that internal/provider/timeouts.go stays the single source of truth.
func checkTimeoutDefaults(pass *analysis.Pass, insp *inspector.Inspector) {
	nodeFilter := []ast.Node{(*ast.CallExpr)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		call := n.(*ast.CallExpr)

		// Match: <expr>.Timeouts.<Op>(ctx, <default>)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !crudMethods[sel.Sel.Name] {
			return
		}
		recv, ok := sel.X.(*ast.SelectorExpr)
		if !ok || recv.Sel.Name != "Timeouts" {
			return
		}
		if len(call.Args) != 2 {
			return
		}

		// A named constant is a bare identifier. Anything else — a BasicLit or a
		// BinaryExpr such as 2*time.Minute — is a literal duration.
		if _, ok := call.Args[1].(*ast.Ident); ok {
			return
		}

		pass.Reportf(call.Args[1].Pos(),
			"Timeouts.%s must use a named default timeout constant "+
				"(e.g. Default%sTimeout from timeouts.go), not a literal duration",
			sel.Sel.Name, sel.Sel.Name)
	})
}

// checkWithTimeout reports CRUD methods that make API calls without wrapping
// their context with context.WithTimeout.
func checkWithTimeout(pass *analysis.Pass, insp *inspector.Inspector) {
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
					"using the Timeouts field (add timeout support)",
				fn.Name.Name)
		}
	})
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
