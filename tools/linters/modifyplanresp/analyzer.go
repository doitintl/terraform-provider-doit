// Package modifyplanresp flags a resource.ModifyPlanResponse built in a test
// without a Plan.
//
// The framework always hands ModifyPlan a response whose Plan is the request
// plan (fwserver builds it as ModifyPlanResponse{Plan: modifyPlanReq.Plan, ...}).
// A unit test that passes &resource.ModifyPlanResponse{} hands it an empty plan
// instead: no schema and a nil Raw. That works only while ModifyPlan neither
// reads nor writes resp.Plan, and it panics with a nil pointer dereference as
// soon as it does — which is how a ModifyPlan change in an unrelated attribute
// broke TestBudgetPeriodModifyPlan.
//
// Three shapes are flagged:
//
//   - a keyed composite literal with no Plan key, including the empty one;
//   - new(resource.ModifyPlanResponse);
//   - var x resource.ModifyPlanResponse without an initializer.
//
// Every such value is flagged, not only one seen flowing into a ModifyPlan call
// or a helper taking *resource.ModifyPlanResponse: a ModifyPlanResponse has no
// other use, so tracking the data flow would add complexity and catch nothing
// more. An unkeyed literal sets every field and is skipped, as is an explicit
// Plan: tfsdk.Plan{}, which is deliberate.
//
// The check applies to test files only — production code never builds a
// ModifyPlanResponse, the framework does. Note this linter is meaningless if
// golangci-lint's run.tests is ever disabled, and would then fail silently
// rather than loudly.
package modifyplanresp

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	// resourcePkg is matched by full package path so that an unrelated type of
	// the same name is not flagged.
	resourcePkg = "github.com/hashicorp/terraform-plugin-framework/resource"

	responseType = "ModifyPlanResponse"

	planField = "Plan"
)

// Analyzer is the go/analysis Analyzer for modifyplanresp.
var Analyzer = &analysis.Analyzer{
	Name:     "modifyplanresp",
	Doc:      "Flags resource.ModifyPlanResponse values in tests that are built without a Plan.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.CompositeLit)(nil),
		(*ast.CallExpr)(nil),
		(*ast.ValueSpec)(nil),
	}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		if !strings.HasSuffix(pass.Fset.Position(n.Pos()).Filename, "_test.go") {
			return
		}

		switch node := n.(type) {
		case *ast.CompositeLit:
			checkLiteral(pass, node)
		case *ast.CallExpr:
			checkNew(pass, node)
		case *ast.ValueSpec:
			checkVar(pass, node)
		}
	})

	return nil, nil
}

// checkLiteral reports a keyed ModifyPlanResponse literal without a Plan key.
//
// The literal inside &resource.ModifyPlanResponse{} is itself a CompositeLit,
// so the pointer form needs no separate case.
func checkLiteral(pass *analysis.Pass, lit *ast.CompositeLit) {
	if !isModifyPlanResponse(pass.TypesInfo.TypeOf(lit)) {
		return
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			// Unkeyed: every field is set, Plan included.
			return
		}
		if key, ok := kv.Key.(*ast.Ident); ok && key.Name == planField {
			return
		}
	}

	report(pass, lit.Pos(), "without "+planField)
}

// checkNew reports new(resource.ModifyPlanResponse).
func checkNew(pass *analysis.Pass, call *ast.CallExpr) {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok || len(call.Args) != 1 {
		return
	}
	builtin, ok := pass.TypesInfo.Uses[ident].(*types.Builtin)
	if !ok || builtin.Name() != "new" {
		return
	}
	if !isModifyPlanResponse(pass.TypesInfo.TypeOf(call.Args[0])) {
		return
	}

	report(pass, call.Pos(), "from new()")
}

// checkVar reports var x resource.ModifyPlanResponse without an initializer.
func checkVar(pass *analysis.Pass, spec *ast.ValueSpec) {
	if spec.Type == nil || len(spec.Values) > 0 {
		return
	}
	if !isModifyPlanResponse(pass.TypesInfo.TypeOf(spec.Type)) {
		return
	}

	report(pass, spec.Pos(), "declared as a zero value")
}

func report(pass *analysis.Pass, pos token.Pos, what string) {
	pass.Reportf(pos,
		"resource.%s %s hands ModifyPlan an empty plan, which panics once it "+
			"reads or writes resp.Plan; the framework seeds the response with the "+
			"request plan, so set %s: req.Plan (or tfsdk.Plan{Schema: s, Raw: planRaw.Copy()})",
		responseType, what, planField)
}

// isModifyPlanResponse reports whether t is resource.ModifyPlanResponse.
func isModifyPlanResponse(t types.Type) bool {
	if t == nil {
		return false
	}
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == resourcePkg && obj.Name() == responseType
}
