// Package unknownguard ensures that data source Read() methods check for
// unknown inputs before making API calls.
//
// GEMINI.md §10.5 (Unknown Input Handling in Data Sources):
//
// If any input attribute is Unknown during plan (e.g., depends on an unresolved
// resource), the data source must NOT make API calls. Instead, it should set all
// computed outputs to Unknown and return early.
//
// Detection strategy:
//  1. Find data source Read() methods (parameter type contains "datasource.ReadRequest")
//  2. Look up the data source's schema via schemaparser facts
//  3. If the schema has any Required or Optional input attributes, verify the
//     Read() body contains IsUnknown() or IsFullyKnown() checks
//  4. Data sources with only Computed attributes are excluded (no user inputs to check)
package unknownguard

import (
	"go/ast"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for unknownguard.
var Analyzer = &analysis.Analyzer{
	Name:     "unknownguard",
	Doc:      "Ensures data source Read() checks for unknown inputs before API calls (GEMINI.md §10.5).",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	result := pass.ResultOf[schemaparser.Analyzer]
	if result == nil {
		return nil, nil
	}
	schemaFacts, ok := result.(*schemaparser.SchemaFacts)
	if !ok || schemaFacts == nil {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Build a map of data source schema names → whether they have user inputs.
	dsHasInputs := map[string]bool{}
	for name, info := range schemaFacts.Schemas {
		if !strings.HasSuffix(name, "DataSourceSchema") {
			continue
		}
		for _, attr := range info.Attrs {
			if attr.Class == schemaparser.Required ||
				attr.Class == schemaparser.Optional ||
				attr.Class == schemaparser.OptionalComputed {
				dsHasInputs[name] = true
				break
			}
		}
	}

	// Find each data source's Schema() method to map receiver types to schema names.
	// Then find Read() methods on the same receiver types.
	type dsInfo struct {
		schemaName string
		receiverType string
	}

	// Step 1: Map receiver types to their schema names via Schema() methods.
	receiverToSchema := map[string]string{}
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Name.Name != "Schema" || fn.Body == nil {
			return
		}
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		recvType := extractReceiverTypeName(fn)
		if recvType == "" {
			return
		}

		// Find the schema function call: s := datasource_xxx.XxxDataSourceSchema(ctx)
		schemaName := findDataSourceSchemaCall(fn)
		if schemaName == "" {
			return
		}

		receiverToSchema[recvType] = schemaName
	})

	// Step 2: Find Read() methods on data source types and check for unknown guards.
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Name.Name != "Read" || fn.Body == nil {
			return
		}
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		// Verify this is a data source Read (has datasource.ReadRequest parameter).
		if !isDataSourceRead(fn) {
			return
		}

		recvType := extractReceiverTypeName(fn)
		if recvType == "" {
			return
		}

		schemaName, ok := receiverToSchema[recvType]
		if !ok {
			return
		}

		// Skip data sources with no user inputs.
		if !dsHasInputs[schemaName] {
			return
		}

		// Check if the Read() body contains an unknown guard.
		if hasUnknownGuard(fn.Body) {
			return
		}

		pass.Reportf(fn.Name.Pos(),
			"data source Read() must check for unknown inputs before API calls "+
				"(IsUnknown/IsFullyKnown); inputs may be unknown during plan (GEMINI.md §10.5)")
	})

	return nil, nil
}

// isDataSourceRead checks if a Read method has datasource.ReadRequest parameter.
func isDataSourceRead(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}
	for _, param := range fn.Type.Params.List {
		if containsSelector(param.Type, "datasource", "ReadRequest") {
			return true
		}
	}
	return false
}

// containsSelector checks if an expression contains a pkg.Name selector.
func containsSelector(expr ast.Expr, pkg, name string) bool {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name == pkg && t.Sel.Name == name
		}
	}
	return false
}

// extractReceiverTypeName gets the base type name from a method receiver.
func extractReceiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	recv := fn.Recv.List[0].Type
	// Unwrap *T to T.
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}
	if ident, ok := recv.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// findDataSourceSchemaCall finds the generated schema function call in a Schema() body.
// Matches: s := datasource_xxx.XxxDataSourceSchema(ctx)
func findDataSourceSchemaCall(fn *ast.FuncDecl) string {
	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		var name string
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			name = f.Sel.Name
		case *ast.Ident:
			name = f.Name
		default:
			continue
		}
		if strings.HasSuffix(name, "DataSourceSchema") {
			return name
		}
	}
	return ""
}

// hasUnknownGuard checks if a function body contains IsUnknown() or IsFullyKnown() calls.
func hasUnknownGuard(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch sel.Sel.Name {
		case "IsUnknown", "IsFullyKnown":
			found = true
			return false
		}
		return true
	})
	return found
}
