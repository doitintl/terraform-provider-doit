// Package errformat ensures that error messages for HTTP response errors
// include both the status code and response body for debugging.
// GEMINI.md §6 (Error Messages).
//
// Scope: Only flags AddError calls inside HTTP status code conditionals
// (e.g. `if resp.StatusCode() != 200`). Non-HTTP errors (e.g., connection
// failures) are excluded.
//
// For generic error paths (`StatusCode() != NNN`), both StatusCode() and Body
// must appear in the error message arguments.
//
// For specific status checks (`StatusCode() == 404`), we only flag if
// *neither* StatusCode() nor Body appears — a semantic message like
// "resource not found" is acceptable for known status codes.
package errformat

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for errformat.
var Analyzer = &analysis.Analyzer{
	Name:     "errformat",
	Doc:      "Ensures HTTP error messages include status code and response body.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.IfStmt)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		ifStmt := n.(*ast.IfStmt)

		// Check if the if-condition contains a StatusCode() comparison.
		cmpInfo := findStatusCodeComparison(ifStmt.Cond)
		if cmpInfo == nil {
			return
		}

		// Find AddError calls in the if body.
		for _, addErr := range findAddErrorCalls(ifStmt.Body) {
			checkAddErrorArgs(pass, addErr, cmpInfo)
		}
	})

	return nil, nil
}

// comparisonInfo describes a StatusCode() comparison.
type comparisonInfo struct {
	op token.Token // NEQ or EQL
	// receiverName is the object whose StatusCode() is called (e.g., "deleteResp").
	receiverName string
}

// findStatusCodeComparison inspects an if-condition for a StatusCode() comparison.
// For compound conditions (e.g., `a != 200 && a != 404`), returns info about
// the first comparison found.
func findStatusCodeComparison(expr ast.Expr) *comparisonInfo {
	var result *comparisonInfo
	ast.Inspect(expr, func(n ast.Node) bool {
		if result != nil {
			return false
		}
		binExpr, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		if binExpr.Op != token.NEQ && binExpr.Op != token.EQL {
			return true
		}

		// Check if either side is a StatusCode() call.
		recvName := statusCodeReceiver(binExpr.X)
		if recvName == "" {
			recvName = statusCodeReceiver(binExpr.Y)
		}
		if recvName != "" {
			result = &comparisonInfo{op: binExpr.Op, receiverName: recvName}
			return false
		}
		return true
	})
	return result
}

// statusCodeReceiver returns the receiver name from a xxx.StatusCode() call,
// or "" if the expression is not such a call.
func statusCodeReceiver(expr ast.Expr) string {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "StatusCode" {
		return ""
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

// findAddErrorCalls returns all AddError call expressions in a block.
func findAddErrorCalls(body *ast.BlockStmt) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "AddError" {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

// checkAddErrorArgs checks whether an AddError call's arguments reference
// StatusCode() and Body for the HTTP response variable.
func checkAddErrorArgs(pass *analysis.Pass, call *ast.CallExpr, cmp *comparisonInfo) {
	// AddError takes 2 args: summary and detail. The detail (2nd arg)
	// should contain the status and body references.
	if len(call.Args) < 2 {
		return
	}

	detail := call.Args[1]
	hasStatus := containsMethodCall(detail, cmp.receiverName, "StatusCode")
	hasBody := containsFieldAccess(detail, cmp.receiverName, "Body")

	if cmp.op == token.NEQ {
		// Generic error path: MUST include both status and body.
		if !hasStatus && !hasBody {
			pass.Reportf(call.Pos(),
				"HTTP error message must include both %s.StatusCode() and %s.Body "+
					"(see GEMINI.md §6 Error Messages)",
				cmp.receiverName, cmp.receiverName)
		} else if !hasStatus {
			pass.Reportf(call.Pos(),
				"HTTP error message includes body but is missing %s.StatusCode() "+
					"(see GEMINI.md §6 Error Messages)",
				cmp.receiverName)
		} else if !hasBody {
			pass.Reportf(call.Pos(),
				"HTTP error message includes status but is missing %s.Body "+
					"(see GEMINI.md §6 Error Messages)",
				cmp.receiverName)
		}
	}
	// For EQL (e.g., == 404): we don't flag — the status is already known
	// context from the if-condition. A semantic message is acceptable.
}

// containsMethodCall checks if an expression tree contains receiver.method().
func containsMethodCall(expr ast.Expr, receiver, method string) bool {
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
		if sel.Sel.Name != method {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == receiver {
			found = true
			return false
		}
		return true
	})
	return found
}

// containsFieldAccess checks if an expression tree contains receiver.field.
func containsFieldAccess(expr ast.Expr, receiver, field string) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != field {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == receiver {
			found = true
			return false
		}
		return true
	})
	return found
}
