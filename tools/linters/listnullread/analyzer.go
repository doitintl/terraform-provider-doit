// Package listnullread flags types.ListNull() usage in Read-path mapping
// functions (mapXxxToModel / populateState) for list attributes that are
// Optional or Optional+Computed.
//
// The overlay resolves unknown lists to
// empty [], so the Read path must also return [] (not null) for empty/nil
// API responses. Using ListNull() here causes state churn (null↔[] flip)
// on every apply.
//
// This analyzer uses schemaparser to know which attributes are lists and
// their classification. It scopes the check to the specific schema
// associated with the mapping function's receiver type, preventing false
// positives when the same attribute name has different classifications
// across resource and data source schemas.
package listnullread

import (
	"go/ast"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for listnullread.
var Analyzer = &analysis.Analyzer{
	Name:     "listnullread",
	Doc:      "Flags types.ListNull() in Read paths for Optional/Optional+Computed list attributes.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	result := pass.ResultOf[schemaparser.Analyzer]
	if result == nil {
		return nil, nil
	}
	schemaResult, ok := result.(*schemaparser.SchemaFacts)
	if !ok || schemaResult == nil {
		return nil, nil
	}

	// Build per-schema maps of user-configurable list attributes.
	perSchemaLists := map[string]map[string]bool{}
	for schemaName, schemaInfo := range schemaResult.Schemas {
		lists := map[string]bool{}
		for fieldName, info := range schemaInfo.Attrs {
			if info.IsList && (info.Class == schemaparser.Optional || info.Class == schemaparser.OptionalComputed) {
				lists[fieldName] = true
			}
		}
		if len(lists) > 0 {
			perSchemaLists[schemaName] = lists
		}
	}

	if len(perSchemaLists) == 0 {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

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

		schemaName := findSchemaCall(fn)
		if schemaName == "" {
			return
		}

		receiverToSchema[recvType] = schemaName
	})

	// Step 2: Find mapping functions and check ListNull calls against
	// the schema associated with the method's receiver type.
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil {
			return
		}
		name := fn.Name.Name
		if !isMappingFunction(name) {
			return
		}

		// Determine which schema this mapping function belongs to.
		recvType := extractReceiverTypeName(fn)
		if recvType == "" {
			return
		}

		schemaName, ok := receiverToSchema[recvType]
		if !ok {
			return
		}

		lists, ok := perSchemaLists[schemaName]
		if !ok {
			return
		}

		// Find ListNull calls in the function body and check if they're
		// assigned to user-configurable list fields for THIS schema.
		findListNullAssignments(pass, fn.Body, lists)
	})

	return nil, nil
}

// isMappingFunction returns true if the function name matches the Read-path
// mapping function naming convention.
func isMappingFunction(name string) bool {
	if name == "populateState" {
		return true
	}
	if strings.HasPrefix(name, "map") && strings.Contains(name, "ToModel") {
		return true
	}
	if strings.HasPrefix(name, "map") && strings.Contains(name, "Resource") {
		return true
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

// findSchemaCall finds the generated schema function call in a Schema() body.
// Matches both resource and data source schema calls:
//   - resource_xxx.XxxResourceSchema(ctx)
//   - datasource_xxx.XxxDataSourceSchema(ctx)
func findSchemaCall(fn *ast.FuncDecl) string {
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
		if strings.HasSuffix(name, "ResourceSchema") || strings.HasSuffix(name, "DataSourceSchema") {
			return name
		}
	}
	return ""
}

// findListNullAssignments finds assignments like:
//
//	state.Scopes = types.ListNull(...)
//
// and flags them if the field is a user-configurable list.
func findListNullAssignments(pass *analysis.Pass, body *ast.BlockStmt, userConfigLists map[string]bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, rhs := range assign.Rhs {
			if !isListNullCall(rhs) {
				continue
			}
			if i >= len(assign.Lhs) {
				continue
			}

			// Check if the LHS is state.FieldName
			fieldName := extractFieldName(assign.Lhs[i])
			if fieldName == "" {
				continue
			}

			// Convert PascalCase to snake_case for schema lookup.
			snakeName := toSnakeCase(fieldName)
			if userConfigLists[snakeName] {
				pass.Reportf(assign.Pos(),
					"types.ListNull() used for Optional/Optional+Computed list field %q in Read path; "+
						"use types.ListValueMust() with empty slice instead to avoid null↔[] state churn "+
						"(use empty list for null/[] consistency)",
					snakeName)
			}
		}

		return true
	})
}

// isListNullCall checks if an expression is types.ListNull(...).
func isListNullCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if sel.Sel.Name != "ListNull" {
		return false
	}
	// Verify it's the types package (or at least a package, not a method).
	if ident, ok := sel.X.(*ast.Ident); ok {
		return ident.Name == "types"
	}
	return false
}

// extractFieldName returns the field name from state.FieldName or plan.FieldName.
func extractFieldName(expr ast.Expr) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	// Verify it's a simple x.Field access.
	if _, ok := sel.X.(*ast.Ident); !ok {
		return ""
	}
	return sel.Sel.Name
}

// toSnakeCase converts PascalCase to snake_case.
func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, r+32) // toLower
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}
