// Package newexpr detects the "temp variable then take address" anti-pattern
// and suggests using new(expr) instead (Go 1.26+).
//
// Enforces new(expr) for pointer conversions (Go 1.26+):
//
//	// OLD - verbose
//	filter := data.Filter.ValueString()
//	params.Filter = &filter
//
//	// NEW - concise
//	params.Filter = new(data.Filter.ValueString())
//
// The analyzer finds variables that are:
//  1. Declared with := and a single LHS/RHS
//  2. Only used once after declaration, in a &x expression
//  3. NOT composite literals (&type{} — those are constructors, excluded per §6)
package newexpr

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for newexpr.
var Analyzer = &analysis.Analyzer{
	Name:     "newexpr",
	Doc:      "Suggests new(expr) instead of temp-variable-then-address pattern (Go 1.26+).",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		var body *ast.BlockStmt
		switch fn := n.(type) {
		case *ast.FuncDecl:
			body = fn.Body
		case *ast.FuncLit:
			body = fn.Body
		}
		if body == nil {
			return
		}
		checkBlock(pass, body)
	})

	return nil, nil
}

// checkBlock scans a block's statements for the temp-var-then-address pattern.
func checkBlock(pass *analysis.Pass, block *ast.BlockStmt) {
	for i := 0; i < len(block.List); i++ {
		// Match: x := expr (short var decl, single LHS, single RHS)
		assign, ok := block.List[i].(*ast.AssignStmt)
		if !ok || assign.Tok != token.DEFINE {
			continue
		}
		if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || ident.Obj == nil {
			continue
		}

		// Skip if RHS is a composite literal — &type{} is a constructor pattern (§6).
		if _, isCompLit := assign.Rhs[0].(*ast.CompositeLit); isCompLit {
			continue
		}

		// Count references to this variable in subsequent statements.
		// We distinguish between &x references and other references.
		varObj := ident.Obj
		addrOfCount := 0
		otherRefCount := 0

		for j := i + 1; j < len(block.List); j++ {
			ast.Inspect(block.List[j], func(n ast.Node) bool {
				// Check for &x — the address-of pattern we want to replace.
				if unary, ok := n.(*ast.UnaryExpr); ok && unary.Op == token.AND {
					if id, ok := unary.X.(*ast.Ident); ok && id.Obj == varObj {
						addrOfCount++
						return false // don't count the ident inside &x separately
					}
				}
				// Count any other reference to x.
				if id, ok := n.(*ast.Ident); ok && id.Obj == varObj {
					otherRefCount++
				}
				return true
			})
		}

		// Flag only if the variable is used exactly once (in &x) and nowhere else.
		if addrOfCount == 1 && otherRefCount == 0 {
			pass.Reportf(assign.Pos(),
				"use new(%s) instead of temp variable %q then &%s (Go 1.26+)",
				exprString(assign.Rhs[0]), ident.Name, ident.Name)
		}
	}
}

// exprString returns a short human-readable representation of an expression.
func exprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.CallExpr:
		return exprString(e.Fun) + "(...)"
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	case *ast.Ident:
		return e.Name
	default:
		return "expr"
	}
}
