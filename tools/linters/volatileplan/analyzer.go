// Package volatileplan ensures that every resource with a clearing plan
// modifier marks its volatile Computed-only attributes unknown on updates.
//
// The framework marks computed values unknown only when Terraform's proposed
// state differs from the prior state, and attribute plan modifiers run
// afterwards. Removing an Optional+Computed attribute changes nothing in the
// proposed state, because Terraform proposes the stored value, so a clearing
// modifier plans an update with the stored values of all Computed-only
// attributes still known. An overlay that guards such an attribute with
// IsUnknown() then keeps the stale value in state (a stale etag makes the next
// update fail with 412), and an unguarded overlay fails with "Provider produced
// inconsistent result" when the API returns a new value.
//
// Every top-level Computed-only attribute of such a resource must either have
// UseStateForUnknown (the API never changes it) or be passed to
// planVolatileAttributesUnknownOnUpdate in the resource's ModifyPlan method.
package volatileplan

import (
	"go/ast"
	"slices"
	"sort"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// helperName is the ModifyPlan helper that marks volatile attributes unknown.
const helperName = "planVolatileAttributesUnknownOnUpdate"

// Analyzer is the go/analysis Analyzer for volatileplan.
var Analyzer = &analysis.Analyzer{
	Name:     "volatileplan",
	Doc:      "Ensures resources with a clearing plan modifier pass their volatile Computed-only attributes to " + helperName + " in ModifyPlan.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	facts := pass.ResultOf[schemaparser.Analyzer].(*schemaparser.SchemaFacts)

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Name.Name != "Schema" || fn.Body == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}
		name := baseResourceSchema(fn.Body)
		if name == "" || !usesClearingModifier(fn.Body) {
			return
		}
		info, ok := facts.Schemas[name]
		if !ok {
			return
		}
		if missing := unplannedVolatileAttributes(info); len(missing) > 0 {
			pass.Reportf(fn.Pos(),
				"resource has a clearing plan modifier, and Computed-only attribute(s) %s have no UseStateForUnknown: "+
					"ModifyPlan must pass them to %s(ctx, req, resp, path.Root(...), ...), "+
					"or an update that only clears an attribute plans their stored values "+
					"(stale state, or an inconsistent result when the API changes them); "+
					"add UseStateForUnknown instead to an attribute the API never changes",
				strings.Join(missing, ", "), helperName)
		}
	})
	return nil, nil
}

// unplannedVolatileAttributes returns the sorted top-level Computed-only
// attributes that neither have UseStateForUnknown nor are passed to the helper.
func unplannedVolatileAttributes(info *schemaparser.SchemaInfo) []string {
	var missing []string
	for name, attr := range info.Attrs {
		if name == "timeouts" || attr.Class != schemaparser.ComputedOnly || attr.VolatileOnUpdate ||
			slices.Contains(attr.PlanModifiers, "UseStateForUnknown") {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return missing
}

// baseResourceSchema returns the generated resource schema function a Schema()
// method body calls (s := resource_xxx.XxxResourceSchema(ctx)).
func baseResourceSchema(body *ast.BlockStmt) string {
	name := ""
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || name != "" {
			return name == ""
		}
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			if strings.HasSuffix(f.Sel.Name, "ResourceSchema") {
				name = f.Sel.Name
			}
		case *ast.Ident:
			if strings.HasSuffix(f.Name, "ResourceSchema") {
				name = f.Name
			}
		}
		return name == ""
	})
	return name
}

// usesClearingModifier reports whether a Schema() body calls one of the
// clearing plan modifier constructors (useEmptyFor..., useNullFor...).
func usesClearingModifier(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok &&
				(strings.HasPrefix(ident.Name, "useEmptyFor") || strings.HasPrefix(ident.Name, "useNullFor")) {
				found = true
			}
		}
		return !found
	})
	return found
}
