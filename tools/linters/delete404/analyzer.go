// Package delete404 ensures that Delete methods treat 404 responses as success.
// When a resource has already been deleted outside of Terraform, the Delete
// method should not return an error — the desired state (resource gone) already
// exists. GEMINI.md §6 (Delete 404 Handling).
package delete404

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for delete404.
var Analyzer = &analysis.Analyzer{
	Name:     "delete404",
	Doc:      "Ensures Delete methods treat HTTP 404 as success (resource already gone).",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Name.Name != "Delete" || fn.Body == nil {
			return
		}
		// Only check methods (have a receiver).
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		// Check if there's an API call (StatusCode check) in this Delete method.
		// If not, it's a no-op Delete — skip.
		statusCheckNode := findStatusCodeCheck(fn.Body)
		if statusCheckNode == nil {
			return
		}

		// Check if 404 is allowed (excluded from the error path).
		if !allows404(fn.Body) {
			pass.Reportf(statusCheckNode.Pos(),
				"Delete method must treat 404 as success (resource already gone); "+
					"add '&& deleteResp.StatusCode() != 404' to the error condition")
		}
	})

	return nil, nil
}

// findStatusCodeCheck looks for a call to StatusCode() in an if-condition
// within the function body. Returns the first such if-statement, or nil.
func findStatusCodeCheck(body *ast.BlockStmt) *ast.IfStmt {
	var result *ast.IfStmt
	ast.Inspect(body, func(n ast.Node) bool {
		if result != nil {
			return false
		}
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		if containsStatusCodeCall(ifStmt.Cond) {
			result = ifStmt
			return false
		}
		return true
	})
	return result
}

// containsStatusCodeCall checks if an expression tree contains a call to StatusCode().
func containsStatusCodeCall(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
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
		if sel.Sel.Name == "StatusCode" {
			found = true
			return false
		}
		return true
	})
	return found
}

// allows404 checks whether the function body contains a comparison with 404
// that excludes it from the error path. We look for either:
//   - `!= 404` in an if-condition (allowing 404 through)
//   - `== 404` in an if-condition that returns early (treating 404 as success)
func allows404(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		// Look for binary expressions comparing to 404.
		binExpr, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		if (binExpr.Op == token.NEQ || binExpr.Op == token.EQL) && is404Literal(binExpr.X, binExpr.Y) {
			found = true
			return false
		}
		return true
	})
	return found
}

// is404Literal checks if either x or y is the integer literal 404.
func is404Literal(x, y ast.Expr) bool {
	return isIntLiteral(x, "404") || isIntLiteral(y, "404")
}

// isIntLiteral checks if an expression is a specific integer literal.
func isIntLiteral(expr ast.Expr, value string) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		return false
	}
	return lit.Kind == token.INT && lit.Value == value
}
