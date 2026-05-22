// Package crudnaming enforces that variables populated via req.Plan.Get() or
// req.State.Get() are named consistently with their data source:
//
//   - req.Plan.Get(ctx, &x)  → x must be named "plan"
//   - req.State.Get(ctx, &x) → x must be named "state"
//   - req.State.GetAttribute(ctx, ..., &x) → no constraint (scalar extraction)
//
// This catches actual mismatches (e.g., `var plan model; req.State.Get(ctx, &plan)`)
// while allowing legitimate secondary reads like `req.State.GetAttribute(ctx,
// path.Root("id"), &stateId)` in Update methods.
//
// GEMINI.md §6 (Resource Code Conventions):
//
//	Use `plan` in Create and Update methods (data from Terraform plan)
//	Use `state` in Read and Delete methods (data from Terraform state)
package crudnaming

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for crudnaming.
var Analyzer = &analysis.Analyzer{
	Name:     "crudnaming",
	Doc:      "Enforces variable names match their data source: req.Plan.Get → plan, req.State.Get → state (GEMINI.md §6).",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Find all method calls matching req.Plan.Get(ctx, &x) or req.State.Get(ctx, &x).
	nodeFilter := []ast.Node{(*ast.CallExpr)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		call := n.(*ast.CallExpr)

		// Must be a selector call: something.Get(...)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Get" {
			return
		}

		// The receiver must be req.Plan or req.State (another selector).
		innerSel, ok := sel.X.(*ast.SelectorExpr)
		if !ok {
			return
		}

		// Determine which data source this reads from and what the wrong name would be.
		var wrongName string
		switch innerSel.Sel.Name {
		case "Plan":
			wrongName = "state" // reading from Plan but variable says "state"
		case "State":
			wrongName = "plan" // reading from State but variable says "plan"
		default:
			return
		}

		// Verify the outermost receiver is a simple ident (typically "req" or "resp").
		if _, ok := innerSel.X.(*ast.Ident); !ok {
			return
		}

		// Get must have at least 2 args: ctx, &model.
		if len(call.Args) < 2 {
			return
		}

		// Second arg must be &x (unary & expression).
		unary, ok := call.Args[1].(*ast.UnaryExpr)
		if !ok {
			return
		}
		ident, ok := unary.X.(*ast.Ident)
		if !ok {
			return
		}

		// Flag only if the variable name contains the WRONG source name.
		// "plan" from req.State.Get() → wrong (contains "plan")
		// "oldState" from req.State.Get() → fine (contains "state", not "plan")
		// "data" from req.Plan.Get() → fine (neutral name)
		lower := strings.ToLower(ident.Name)
		if strings.Contains(lower, wrongName) {
			expectedName := "plan"
			if wrongName == "plan" {
				expectedName = "state"
			}
			pass.Reportf(call.Pos(),
				"variable %q is populated from req.%s.Get() but name suggests %s; use %q instead (GEMINI.md §6)",
				ident.Name, innerSel.Sel.Name, wrongName, expectedName)
		}
	})

	return nil, nil
}
