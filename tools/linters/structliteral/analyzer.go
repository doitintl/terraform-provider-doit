// Package structliteral detects direct struct literal construction of generated
// Terraform framework Value types. These types have unexported internal state
// fields that must be initialized via their generated constructors.
//
// Bad:
//
//	scopeVal := resource_budget.ScopesValue{
//	    Id: types.StringValue("foo"),
//	}
//
// Good:
//
//	scopeVal, diags := resource_budget.NewScopesValue(
//	    resource_budget.ScopesValue{}.AttributeTypes(ctx),
//	    map[string]attr.Value{"id": types.StringValue("foo")},
//	)
//
// Exception: Empty struct literals used to call methods are allowed, e.g.:
//
//	resource_budget.ScopesValue{}.AttributeTypes(ctx)
//	resource_budget.ScopesValue{}.Type(ctx)
package structliteral

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for structliteral.
var Analyzer = &analysis.Analyzer{
	Name:     "structliteral",
	Doc:      "Detects direct struct literal construction of generated Terraform Value types (use NewXxxValue instead).",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.CompositeLit)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		lit := n.(*ast.CompositeLit)
		if lit.Type == nil {
			return
		}

		// Check if the type is from a resource_* or datasource_* package
		// and ends with "Value".
		pkgName, typeName := selectorParts(lit.Type)
		if pkgName == "" || typeName == "" {
			return
		}

		if !isGeneratedValuePackage(pkgName) {
			return
		}

		if !strings.HasSuffix(typeName, "Value") {
			return
		}

		// Allow empty struct literals used as method receivers:
		// resource_budget.ScopesValue{}.AttributeTypes(ctx)
		if len(lit.Elts) == 0 {
			return
		}

		pass.Reportf(lit.Pos(),
			"do not construct %s.%s with struct literal; use New%s() or New%sMust() constructor instead",
			pkgName, typeName, typeName, typeName)
	})

	return nil, nil
}

// selectorParts extracts the package name and type name from a selector expression.
// For resource_budget.ScopesValue, returns ("resource_budget", "ScopesValue").
func selectorParts(expr ast.Expr) (string, string) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", ""
	}
	return ident.Name, sel.Sel.Name
}

// isGeneratedValuePackage checks if a package name matches the generated
// resource/datasource package naming convention.
func isGeneratedValuePackage(name string) bool {
	return strings.HasPrefix(name, "resource_") || strings.HasPrefix(name, "datasource_")
}
