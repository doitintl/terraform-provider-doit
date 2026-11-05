// Package volatileplan ensures that every resource with an etag and a clearing
// plan modifier marks the etag unknown on updates.
//
// The framework marks computed values unknown only when Terraform's proposed
// state differs from the prior state, and attribute plan modifiers run
// afterwards. Removing an Optional+Computed attribute changes nothing in the
// proposed state, because Terraform proposes the stored value, so a clearing
// modifier plans an update with the stored etag still known. State then keeps
// the old etag after apply, and the next update without a refresh sends a
// stale If-Match and fails with 412. The resource's ModifyPlan method must
// call planVolatileAttributesUnknownOnUpdate.
//
// Only etag is checked: a stale timestamp such as update_time is replaced by
// the next Read, so it does not break the next update.
package volatileplan

import (
	"go/ast"
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
	Doc:      "Ensures resources with an etag and a clearing plan modifier call " + helperName + " in ModifyPlan.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	facts := pass.ResultOf[schemaparser.Analyzer].(*schemaparser.SchemaFacts)

	type schemaMethod struct {
		fn         *ast.FuncDecl
		schemaName string
	}
	var schemaMethods []schemaMethod
	planned := map[string]bool{} // receiver type → ModifyPlan calls the helper

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}
		switch fn.Name.Name {
		case "Schema":
			if name := baseResourceSchema(fn.Body); name != "" && usesClearingModifier(fn.Body) {
				schemaMethods = append(schemaMethods, schemaMethod{fn: fn, schemaName: name})
			}
		case "ModifyPlan":
			if callsHelper(fn.Body) {
				planned[receiverTypeName(fn)] = true
			}
		}
	})

	for _, m := range schemaMethods {
		info, ok := facts.Schemas[m.schemaName]
		if !ok {
			continue
		}
		etag := info.Attrs["etag"]
		if etag == nil || etag.Class != schemaparser.ComputedOnly || planned[receiverTypeName(m.fn)] {
			continue
		}
		pass.Reportf(m.fn.Pos(),
			"resource has a clearing plan modifier and a Computed etag: "+
				"ModifyPlan must call %s(ctx, req, resp, path.Root(\"etag\"), ...), "+
				"or an update that only clears an attribute keeps the stale etag and the next update fails with 412",
			helperName)
	}
	return nil, nil
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

// callsHelper reports whether a method body calls planVolatileAttributesUnknownOnUpdate.
func callsHelper(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == helperName {
				found = true
			}
		}
		return !found
	})
	return found
}

// receiverTypeName returns the receiver type name of a method, unwrapping a
// pointer receiver.
func receiverTypeName(fn *ast.FuncDecl) string {
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}
