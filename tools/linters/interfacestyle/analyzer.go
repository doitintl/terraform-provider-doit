// Package interfacestyle flags interface satisfaction checks that use &type{}
// instead of the preferred (*type)(nil) style.
//
// Enforces (*type)(nil) style for interface satisfaction checks.
// This is the convention across the entire codebase for both resources and
// data sources.
package interfacestyle

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for interfacestyle.
var Analyzer = &analysis.Analyzer{
	Name:     "interfacestyle",
	Doc:      "Flags &type{} in interface satisfaction checks; use (*type)(nil) instead.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Find: var _ SomeInterface = &myType{}
	nodeFilter := []ast.Node{(*ast.GenDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		genDecl := n.(*ast.GenDecl)
		if genDecl.Tok != token.VAR {
			return
		}

		for _, spec := range genDecl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			// Check for blank identifier: var _ = ...
			if len(vs.Names) != 1 || vs.Names[0].Name != "_" {
				continue
			}

			// Must have an explicit type (the interface being checked).
			if vs.Type == nil {
				continue
			}

			// Check each value: look for &myType{}
			for _, val := range vs.Values {
				unary, ok := val.(*ast.UnaryExpr)
				if !ok || unary.Op != token.AND {
					continue
				}
				// Must be &Type{} — a composite literal.
				comp, ok := unary.X.(*ast.CompositeLit)
				if !ok {
					continue
				}
				// Extract the type name for the message.
				typeName := ""
				switch t := comp.Type.(type) {
				case *ast.Ident:
					typeName = t.Name
				case *ast.SelectorExpr:
					if ident, ok := t.X.(*ast.Ident); ok {
						typeName = ident.Name + "." + t.Sel.Name
					}
				}
				if typeName == "" {
					typeName = "type"
				}

				pass.Reportf(val.Pos(),
					"use (*%s)(nil) instead of &%s{} for interface satisfaction checks",
					typeName, typeName)
			}
		}
	})

	return nil, nil
}
