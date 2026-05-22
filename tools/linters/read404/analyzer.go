// Package read404 ensures that Read methods on resources call
// resp.State.RemoveResource(ctx) to handle externally deleted resources.
// GEMINI.md §6 (Read 404 Handling).
//
// This linter checks for the presence of RemoveResource in the Read method
// body. It accepts both direct patterns (if status == 404 → RemoveResource)
// and indirect patterns (populateState + null-ID check → RemoveResource).
//
// Data source Read methods are excluded — they should return errors on 404.
package read404

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for read404.
var Analyzer = &analysis.Analyzer{
	Name:     "read404",
	Doc:      "Ensures resource Read methods call RemoveResource to handle externally deleted resources.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Name.Name != "Read" || fn.Body == nil {
			return
		}
		// Only check methods (have a receiver).
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		// Exclude data source Read methods. Data sources use ReadRequest from
		// the datasource package, while resources use ReadRequest from the
		// resource package. We detect data sources by checking if the file
		// name ends with _data_source.go.
		pos := pass.Fset.Position(fn.Pos())
		if isDataSourceFile(pos.Filename) {
			return
		}

		// Check if the method makes API calls. If it has no API calls at all,
		// it's likely an import-only resource or stub.
		if !hasAPICalls(fn.Body) {
			return
		}

		// Check if RemoveResource is called anywhere in the method body.
		if !hasRemoveResource(fn.Body) {
			pass.Reportf(fn.Pos(),
				"Read method must call resp.State.RemoveResource(ctx) to handle "+
					"externally deleted resources (404 responses)")
		}
	})

	return nil, nil
}

// isDataSourceFile checks if a filename matches the data source naming pattern.
func isDataSourceFile(filename string) bool {
	// Match files ending in _data_source.go
	if len(filename) < 15 {
		return false
	}
	suffix := filename[len(filename)-15:]
	return suffix == "_data_source.go"
}

// hasAPICalls checks whether the function body contains API-related calls.
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
		// Detect XxxWithResponse calls or StatusCode calls or populateState calls.
		if name == "StatusCode" ||
			name == "populateState" ||
			(len(name) > 12 && name[len(name)-12:] == "WithResponse") {
			found = true
			return false
		}
		return true
	})
	return found
}

// hasRemoveResource checks whether the function body calls RemoveResource.
func hasRemoveResource(body *ast.BlockStmt) bool {
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
		if sel.Sel.Name == "RemoveResource" {
			found = true
			return false
		}
		return true
	})
	return found
}
