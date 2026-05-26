// Package configuretype detects the common scaffold bug where a resource's
// Configure method says "Unexpected Data Source Configure Type" instead of
// "Unexpected Resource Configure Type".
package configuretype

import (
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for configuretype.
var Analyzer = &analysis.Analyzer{
	Name:     "configuretype",
	Doc:      "Detects 'Unexpected Data Source Configure Type' in resource Configure methods (should say 'Resource').",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Name.Name != "Configure" || fn.Body == nil {
			return
		}
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		// Determine if this is a resource or data source from the file name.
		pos := pass.Fset.Position(fn.Pos())
		isResource := isResourceFile(pos.Filename)
		isDataSource := isDataSourceFile(pos.Filename)

		if !isResource && !isDataSource {
			return
		}

		// Walk the function body looking for string literals in AddError calls.
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			lit, ok := node.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}

			val := lit.Value
			if isResource && strings.Contains(val, "Data Source Configure Type") {
				pass.Reportf(lit.Pos(),
					"resource Configure method uses \"Data Source Configure Type\"; "+
						"should be \"Resource Configure Type\"")
			}
			if isDataSource && strings.Contains(val, "Resource Configure Type") &&
				!strings.Contains(val, "Data Source") {
				pass.Reportf(lit.Pos(),
					"data source Configure method uses \"Resource Configure Type\"; "+
						"should be \"Data Source Configure Type\"")
			}

			return true
		})
	})

	return nil, nil
}

// isResourceFile checks if a filename matches the resource naming pattern.
// Resources are *_resource.go but NOT *_data_source.go.
func isResourceFile(filename string) bool {
	if len(filename) < 12 {
		return false
	}
	return strings.HasSuffix(filename, "_resource.go")
}

// isDataSourceFile checks if a filename matches the data source naming pattern.
func isDataSourceFile(filename string) bool {
	return strings.HasSuffix(filename, "_data_source.go")
}
