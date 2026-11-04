// Package setnestedplan ensures that every resource whose schema has a
// SetNestedAttribute with Computed attributes in its elements plans that set
// from the prior state.
//
// Terraform Core does not match the elements of a nested-attribute set to the
// prior state, and the framework cannot either: it marks every Computed (and
// every omitted Optional+Computed) attribute of every element unknown, so the
// set shows a replaced element on every plan. The resource's ModifyPlan method
// must call planSetNestedAttributesFromState, which matches elements by their
// configured values.
//
// Plan modifiers inside a set element cannot fix this: the framework passes
// them the configuration and prior state of the element at the same index,
// which is generally a different element. The linter therefore also reports
// UseStateForUnknown on any attribute inside a set element, where it can copy
// another element's value into the plan.
package setnestedplan

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

// helperName is the ModifyPlan helper that plans sets from the prior state.
const helperName = "planSetNestedAttributesFromState"

// Analyzer is the go/analysis Analyzer for setnestedplan.
var Analyzer = &analysis.Analyzer{
	Name:     "setnestedplan",
	Doc:      "Ensures resources with Computed attributes inside a SetNestedAttribute call " + helperName + " in ModifyPlan.",
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
			if name := baseResourceSchema(fn.Body); name != "" {
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
		if path := stateForUnknownInSet(info.Attrs, "", false); path != "" {
			pass.Reportf(m.fn.Body.Lbrace,
				"%q is inside a set element and has UseStateForUnknown: the framework passes set element "+
					"plan modifiers the prior state of the element at the same index, so it can copy another element's value",
				path)
		}
		if planned[receiverTypeName(m.fn)] {
			continue
		}
		if path := computedSetNested(info.Attrs, ""); path != "" {
			pass.Reportf(m.fn.Pos(),
				"SetNestedAttribute %q has Computed attributes in its elements: "+
					"ModifyPlan must call %s(ctx, req, resp), or every plan shows its elements as replaced",
				path, helperName)
		}
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

// callsHelper reports whether a method body calls planSetNestedAttributesFromState.
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

// computedSetNested returns the path of the first set-nested attribute, at any
// depth, whose elements have a Computed attribute without a default. Paths use
// the [*] element notation of acknowledgeNotClearable.
func computedSetNested(attrs map[string]*schemaparser.AttrInfo, prefix string) string {
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		a := attrs[name]
		if a == nil || a.NestedAttrs == nil {
			continue
		}
		path := prefix + name
		if a.IsSet && hasComputed(a.NestedAttrs) {
			return path
		}
		childPrefix := path + "."
		if a.IsList || a.IsSet {
			childPrefix = path + "[*]."
		}
		if p := computedSetNested(a.NestedAttrs, childPrefix); p != "" {
			return p
		}
	}
	return ""
}

// stateForUnknownInSet returns the path of the first attribute inside a set
// element, at any depth, that has the UseStateForUnknown plan modifier.
func stateForUnknownInSet(attrs map[string]*schemaparser.AttrInfo, prefix string, inSet bool) string {
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		a := attrs[name]
		if a == nil {
			continue
		}
		path := prefix + name
		if inSet && slices.Contains(a.PlanModifiers, "UseStateForUnknown") {
			return path
		}
		childPrefix := path + "."
		if a.IsList || a.IsSet {
			childPrefix = path + "[*]."
		}
		if p := stateForUnknownInSet(a.NestedAttrs, childPrefix, inSet || a.IsSet); p != "" {
			return p
		}
	}
	return ""
}

// hasComputed reports whether any attribute, at any depth, is Computed-only or
// Optional+Computed without a default: those are planned unknown.
func hasComputed(attrs map[string]*schemaparser.AttrInfo) bool {
	for _, a := range attrs {
		if a == nil {
			continue
		}
		if (a.Class == schemaparser.ComputedOnly || a.Class == schemaparser.OptionalComputed) && !a.HasDefault {
			return true
		}
		if hasComputed(a.NestedAttrs) {
			return true
		}
	}
	return false
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
