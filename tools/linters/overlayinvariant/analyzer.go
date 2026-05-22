// Package overlayinvariant enforces the invariant that Create/Update methods
// must call an overlay function (not mapResourceToModel/populateState directly),
// and Read/ImportState must call mapResourceToModel/populateState (not an overlay).
//
// Detection strategy: Instead of relying on function naming, this analyzer
// identifies overlay functions by their usage context — functions called from
// Create/Update that are NOT the mapping functions. It then flags:
//   - mapResourceToModel/populateState calls inside Create/Update methods
//   - overlay* calls inside Read/ImportState methods
package overlayinvariant

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for overlayinvariant.
var Analyzer = &analysis.Analyzer{
	Name:     "overlayinvariant",
	Doc:      "Ensures Create/Update use overlay functions and Read/ImportState use mapping functions, not vice versa.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

// mappingFuncNames are the names of functions that should only be called
// from Read/ImportState, never from Create/Update.
var mappingFuncNames = map[string]bool{
	"mapResourceToModel": true,
	"populateState":      true,
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil {
			return
		}
		// Only check methods on resource types (have a receiver).
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		methodName := fn.Name.Name
		isCreateUpdate := methodName == "Create" || methodName == "Update"
		isReadImport := methodName == "Read" || methodName == "ImportState"

		if !isCreateUpdate && !isReadImport {
			return
		}

		// Walk the function body looking for function calls.
		overlayCallFound := false
		setsState := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			calledName := calledFuncName(call)
			if calledName == "" {
				return true
			}

			if isOverlayFunc(calledName) {
				overlayCallFound = true
			}

			// Detect resp.State.Set(...) calls — indicates state population.
			if calledName == "Set" {
				setsState = true
			}

			if isCreateUpdate && isMappingFunc(calledName) {
				pass.Reportf(call.Pos(),
					"%s must not call %s directly; use the overlay function (e.g., overlay*ComputedFields) instead",
					methodName, calledName)
			}

			if isReadImport && isOverlayFunc(calledName) {
				pass.Reportf(call.Pos(),
					"%s must not call %s; use mapResourceToModel or populateState instead",
					methodName, calledName)
			}

			return true
		})

		// Create/Update must call an overlay function to populate state,
		// but only if the method actually sets state (calls resp.State.Set).
		// Skip no-op methods (e.g., import-only resources returning an error)
		// and methods that set state purely from plan data without API-derived fields.
		if isCreateUpdate && !overlayCallFound && setsState {
			pass.Reportf(fn.Pos(),
				"%s must call an overlay function (overlay*ComputedFields) to populate state; "+
					"do not assign plan fields inline or use ad-hoc helpers",
				methodName)
		}
	})

	return nil, nil
}

// calledFuncName extracts the function name from a call expression.
// Handles both direct calls (populateState(...)) and method calls (r.populateState(...)).
func calledFuncName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

// isMappingFunc checks if a function name matches a mapping function pattern.
func isMappingFunc(name string) bool {
	if mappingFuncNames[name] {
		return true
	}
	// Also match mapXxxToModel pattern.
	if strings.HasPrefix(name, "map") && strings.HasSuffix(name, "ToModel") {
		return true
	}
	return false
}

// isOverlayFunc checks if a function name matches an overlay function pattern.
func isOverlayFunc(name string) bool {
	return strings.HasPrefix(name, "overlay")
}
