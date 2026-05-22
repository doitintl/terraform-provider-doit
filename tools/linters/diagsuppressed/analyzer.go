// Package diagsuppressed detects suppressed diagnostics in Terraform provider code.
//
// When a function returns diag.Diagnostics as its last return value, assigning
// it to the blank identifier (_) silently swallows errors. This analyzer flags
// all such assignments.
//
// Bad:
//
//	myList, _ = types.ListValue(types.StringType, []attr.Value{})
//
// Good:
//
//	var listDiags diag.Diagnostics
//	myList, listDiags = types.ListValue(types.StringType, []attr.Value{})
//	diags.Append(listDiags...)
package diagsuppressed

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for diagsuppressed.
var Analyzer = &analysis.Analyzer{
	Name:     "diagsuppressed",
	Doc:      "Detects suppressed diag.Diagnostics return values assigned to blank identifier (_).",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

// diagType is the fully qualified type name for diag.Diagnostics.
const diagType = "github.com/hashicorp/terraform-plugin-framework/diag.Diagnostics"

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.AssignStmt)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		assign := n.(*ast.AssignStmt)

		// We need at least 2 LHS values (e.g., val, diags := ...).
		if len(assign.Lhs) < 2 {
			return
		}

		// Check each LHS for blank identifiers.
		for i, lhs := range assign.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok || ident.Name != "_" {
				continue
			}

			// Get the type of this position by looking at the RHS.
			// For multi-return calls, we need the type of the i-th return value.
			typ := typeOfPosition(pass, assign, i)
			if typ == nil {
				continue
			}

			if isDiagnosticsType(typ) {
				pass.Reportf(ident.Pos(),
					"diag.Diagnostics return value must not be suppressed with blank identifier; "+
						"assign to a variable and append to the function's diagnostics")
			}
		}
	})

	return nil, nil
}

// typeOfPosition returns the type of the i-th element in a multi-value assignment.
func typeOfPosition(pass *analysis.Pass, assign *ast.AssignStmt, pos int) types.Type {
	// For single RHS with multi-return (e.g., val, diags := someFunc()):
	if len(assign.Rhs) == 1 {
		rhsType := pass.TypesInfo.TypeOf(assign.Rhs[0])
		if rhsType == nil {
			return nil
		}
		// Multi-value return is represented as a Tuple.
		tuple, ok := rhsType.(*types.Tuple)
		if !ok {
			return nil
		}
		if pos >= tuple.Len() {
			return nil
		}
		return tuple.At(pos).Type()
	}

	// For multi-RHS (e.g., a, _ = x, y): each RHS maps to corresponding LHS.
	if pos < len(assign.Rhs) {
		return pass.TypesInfo.TypeOf(assign.Rhs[pos])
	}

	return nil
}

// isDiagnosticsType checks if a type is diag.Diagnostics.
func isDiagnosticsType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path()+"."+obj.Name() == diagType
}
