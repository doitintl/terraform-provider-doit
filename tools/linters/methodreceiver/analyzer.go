// Package methodreceiver flags mapping functions (mapXxxToModel, etc.) that are
// methods instead of free functions. Mapping functions perform pure data
// transformation from API responses to Terraform models and should not depend on
// the resource/data source receiver. Using methods creates an unnecessary
// coupling and prevents other linters from reliably resolving schemas via
// parameter types.
package methodreceiver

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for methodreceiver.
var Analyzer = &analysis.Analyzer{
	Name:     "methodreceiver",
	Doc:      "Flags mapping functions that should be free functions, not methods.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		name := fn.Name.Name
		if !isMappingFunction(name) {
			return
		}

		pass.Reportf(fn.Pos(),
			"mapping function %s should be a free function, not a method; the receiver is not needed for data transformation",
			name)
	})

	return nil, nil
}

// isMappingFunction returns true if the function name matches the Read-path
// mapping function naming convention. populateState is excluded because it
// legitimately needs the receiver to call the API client.
func isMappingFunction(name string) bool {
	if strings.HasPrefix(name, "map") && strings.Contains(name, "ToModel") {
		return true
	}
	if strings.HasPrefix(name, "map") && strings.Contains(name, "Resource") {
		return true
	}
	return false
}
