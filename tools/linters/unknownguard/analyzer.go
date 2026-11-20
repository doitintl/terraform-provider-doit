// Package unknownguard ensures that data source Read() methods defensively
// guard unknown configuration before making API calls.
//
// Terraform Core normally defers data sources with unknown configuration and
// marks their complete results unknown. The plugin protocol still permits
// clients to pass unknown configuration to ReadDataSource, so providers retain
// a defensive guard to prevent premature request construction and API calls.
//
// This analyzer intentionally checks only for guard presence. Schema classes
// cannot identify every input or determine the correct fallback assignment:
// Optional+Computed may be an API-derived output or configuration that must be
// preserved.
//
// Detection strategy:
//  1. Find data source Read() methods (parameter type contains "datasource.ReadRequest")
//  2. Look up the data source's schema via schemaparser facts
//  3. If the schema has any Required or Optional input attributes, verify the
//     Read() body contains IsUnknown() or IsFullyKnown() checks
//  4. Data sources with only Computed attributes are excluded (no user inputs to check)
//
// List data sources (max_results, page_token and row_count in the schema) get
// one stricter check, because their fallback is fully determined: a read
// replaces page_token with the next page's token or null in every mode, and
// row_count is a result. Their Read() must guard with a condition that holds
// for every configuration that is not fully known (!x.IsFullyKnown(), alone or
// ORed with other conditions; an inverted or ANDed check lets unknown
// configuration reach the API), and the guard's body must assign
// PageToken = types.StringUnknown() and
// RowCount = types.Int64Unknown() as top-level statements, not under a
// condition such as "only when page_token is null".
package unknownguard

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for unknownguard.
var Analyzer = &analysis.Analyzer{
	Name:     "unknownguard",
	Doc:      "Ensures data source Read() defensively guards unknown protocol configuration before API calls.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	result := pass.ResultOf[schemaparser.Analyzer]
	if result == nil {
		return nil, nil
	}
	schemaFacts, ok := result.(*schemaparser.SchemaFacts)
	if !ok || schemaFacts == nil {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Build a map of data source schema names → whether they have user inputs,
	// and which of them are paged list data sources.
	dsHasInputs := map[string]bool{}
	dsIsList := map[string]bool{}
	for name, info := range schemaFacts.Schemas {
		if !strings.HasSuffix(name, "DataSourceSchema") {
			continue
		}
		_, hasMaxResults := info.Attrs["max_results"]
		_, hasPageToken := info.Attrs["page_token"]
		_, hasRowCount := info.Attrs["row_count"]
		dsIsList[name] = hasMaxResults && hasPageToken && hasRowCount
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

		if dsIsList[schemaName] {
			checkListGuard(pass, fn)
			return
		}

		// Check if the Read() body contains an unknown guard.
		if hasUnknownGuard(fn.Body) {
			return
		}

		pass.Reportf(fn.Name.Pos(),
			"data source Read() must defensively guard unknown protocol configuration before API calls "+
				"(IsUnknown/IsFullyKnown)")
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

// checkListGuard reports a list data source Read() whose guard is not an
// IsFullyKnown() check that sets PageToken and RowCount unknown unconditionally.
func checkListGuard(pass *analysis.Pass, fn *ast.FuncDecl) {
	var guard *ast.IfStmt
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if guard != nil {
			return false
		}
		ifStmt, ok := n.(*ast.IfStmt)
		if ok && coversNotFullyKnown(ifStmt.Cond) {
			guard = ifStmt
			return false
		}
		return true
	})
	if guard == nil {
		pass.Reportf(fn.Name.Pos(),
			"list data source Read() must guard unknown protocol configuration with req.Config.Raw.IsFullyKnown()")
		return
	}

	var setsPageToken, setsRowCount bool
	for _, stmt := range guard.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		field, ok := assign.Lhs[0].(*ast.SelectorExpr)
		if !ok {
			continue
		}
		switch {
		case field.Sel.Name == "PageToken" && isCallTo(assign.Rhs[0], "StringUnknown"):
			setsPageToken = true
		case field.Sel.Name == "RowCount" && isCallTo(assign.Rhs[0], "Int64Unknown"):
			setsRowCount = true
		}
	}
	if !setsPageToken || !setsRowCount {
		pass.Reportf(guard.Pos(),
			"list data source unknown guard must set PageToken = types.StringUnknown() and RowCount = types.Int64Unknown() "+
				"unconditionally: a read replaces page_token in every mode")
	}
}

// coversNotFullyKnown reports whether cond holds whenever the configuration is
// not fully known: it is !x.IsFullyKnown(), possibly ORed with other conditions.
func coversNotFullyKnown(cond ast.Expr) bool {
	switch e := ast.Unparen(cond).(type) {
	case *ast.UnaryExpr:
		return e.Op == token.NOT && isCallTo(ast.Unparen(e.X), "IsFullyKnown")
	case *ast.BinaryExpr:
		return e.Op == token.LOR && (coversNotFullyKnown(e.X) || coversNotFullyKnown(e.Y))
	}
	return false
}

// isCallTo reports whether expr is a call of a function or method named name.
func isCallTo(expr ast.Expr, name string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		return f.Sel.Name == name
	case *ast.Ident:
		return f.Name == name
	}
	return false
}
