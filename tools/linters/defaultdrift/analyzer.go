// Package defaultdrift flags PointerValue usage on fields with schema defaults
// in mapToModel/populateState functions. When a field has a Default (e.g.,
// stringdefault.StaticString("cost")), using PointerValue maps nil → null,
// which silently drifts against the default value causing perpetual plan changes.
//
// This analyzer uses schemaparser to know which fields have defaults and their
// values, and scopes the check to mapping functions via receiver type → schema
// association (same pattern as listnullread).
package defaultdrift

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for defaultdrift.
var Analyzer = &analysis.Analyzer{
	Name:     "defaultdrift",
	Doc:      "Flags PointerValue usage on fields with schema defaults in Read-path mapping functions.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

// defaultedField records a field with a schema default and its value.
type defaultedField struct {
	name         string // snake_case schema name
	defaultValue any    // extracted default value (string, float64, bool, int64)
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

	// Build per-schema maps of defaulted fields (including nested).
	// Key: schema name, Value: map of snake_case field name → default value.
	type fieldDefaults map[string]defaultedField
	perSchemaDefaults := map[string]fieldDefaults{}
	for schemaName, schemaInfo := range schemaResult.Schemas {
		defaults := fieldDefaults{}
		collectDefaultedFields(schemaInfo.Attrs, defaults, "")
		if len(defaults) > 0 {
			perSchemaDefaults[schemaName] = defaults
		}
	}

	if len(perSchemaDefaults) == 0 {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Step 1: Map receiver types → schema names via Schema() methods.
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

	// Step 2: Find mapping functions and check PointerValue calls.
	// Mapping functions may be methods (with a receiver) or free functions.
	// For methods, we use the receiver type → schema mapping.
	// For free functions, we look at the parameter types for a model type
	// (e.g., *budgetResourceModel) and derive the resource type from it.
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil {
			return
		}
		name := fn.Name.Name
		if !isMappingFunction(name) {
			return
		}

		// Try method receiver first.
		recvType := extractReceiverTypeName(fn)
		var schemaName string
		if recvType != "" {
			schemaName = receiverToSchema[recvType]
		}

		// If no receiver or no schema mapping, try free function parameter types.
		if schemaName == "" {
			schemaName = resolveSchemaFromParams(fn, receiverToSchema)
		}

		if schemaName == "" {
			return
		}

		defaults, ok := perSchemaDefaults[schemaName]
		if !ok {
			return
		}

		// Find PointerValue assignments on defaulted fields.
		findPointerValueAssignments(pass, fn, defaults)
	})

	return nil, nil
}

// resolveSchemaFromParams derives the schema name from function parameter types.
// For free functions like:
//
//	func mapBudgetToModel(ctx context.Context, resp *models.BudgetAPI, state *budgetResourceModel)
//
// it finds *budgetResourceModel → strips "Model" suffix → "budgetResource" → looks
// up in receiverToSchema → "BudgetResourceSchema".
func resolveSchemaFromParams(fn *ast.FuncDecl, receiverToSchema map[string]string) string {
	if fn.Type == nil || fn.Type.Params == nil {
		return ""
	}
	for _, field := range fn.Type.Params.List {
		typeName := extractStarTypeName(field.Type)
		if typeName == "" {
			continue
		}
		// Check if it's a model type: xxxResourceModel or xxxModel.
		resourceType := ""
		if strings.HasSuffix(typeName, "ResourceModel") {
			resourceType = strings.TrimSuffix(typeName, "Model")
		} else if strings.HasSuffix(typeName, "Model") {
			// e.g., DriftTestModel → DriftTest → look for driftTestResource
			base := strings.TrimSuffix(typeName, "Model")
			resourceType = base + "Resource"
			// Try lowercase first letter for private types.
			if len(resourceType) > 0 {
				resourceType = strings.ToLower(resourceType[:1]) + resourceType[1:]
			}
		}
		if resourceType == "" {
			continue
		}
		if schema, ok := receiverToSchema[resourceType]; ok {
			return schema
		}
	}
	return ""
}

// extractStarTypeName extracts the type name from *T (pointer to named type).
func extractStarTypeName(expr ast.Expr) string {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		return ""
	}
	if ident, ok := star.X.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// collectDefaultedFields recursively collects fields with HasDefault into the map.
// The prefix is used for nested attributes (e.g., "config." for config.enabled).
// For the top level and for each nested level, field names are stored WITHOUT
// the prefix so that they match the Go struct field names in assignments.
func collectDefaultedFields(attrs map[string]*schemaparser.AttrInfo, out map[string]defaultedField, prefix string) {
	for name, info := range attrs {
		fullName := prefix + name
		if info.HasDefault {
			out[fullName] = defaultedField{
				name:         fullName,
				defaultValue: info.DefaultValue,
			}
		}
		if info.NestedAttrs != nil {
			// For nested attrs, also register the nested fields directly
			// (without prefix) so they match in nested mapping functions.
			collectDefaultedFields(info.NestedAttrs, out, "")
		}
	}
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
func findSchemaCall(fn *ast.FuncDecl) string {
	var schemaName string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if schemaName != "" {
			return false
		}
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		var name string
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			name = f.Sel.Name
		case *ast.Ident:
			name = f.Name
		default:
			return true
		}
		if strings.HasSuffix(name, "ResourceSchema") || strings.HasSuffix(name, "DataSourceSchema") {
			schemaName = name
		}
		return true
	})
	return schemaName
}

// pointerValueFuncs maps PointerValue function names to their type description.
var pointerValueFuncs = map[string]bool{
	"StringPointerValue":  true,
	"Float64PointerValue": true,
	"BoolPointerValue":    true,
	"Int64PointerValue":   true,
}

// findPointerValueAssignments scans a function body for assignments like:
//
//	state.X = types.StringPointerValue(resp.X)
//
// and flags them if X maps to a field with a schema default. Each finding is
// reported at the assignment position to avoid golangci-lint --uniq-by-line
// deduplication.
func findPointerValueAssignments(pass *analysis.Pass, fn *ast.FuncDecl, defaults map[string]defaultedField) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, rhs := range assign.Rhs {
			if !isPointerValueCall(rhs) {
				continue
			}
			if i >= len(assign.Lhs) {
				continue
			}

			// Check if the LHS is state.FieldName.
			fieldName := extractFieldName(assign.Lhs[i])
			if fieldName == "" {
				continue
			}

			// Convert PascalCase to snake_case for schema lookup.
			snakeName := toSnakeCase(fieldName)
			if df, ok := defaults[snakeName]; ok {
				pass.Reportf(assign.Pos(),
					"PointerValue on field %q with schema default %v; use nil-fallback pattern",
					snakeName, formatDefault(df.defaultValue))
			}
		}

		return true
	})
}

// isPointerValueCall checks if an expression is types.XxxPointerValue(...).
func isPointerValueCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if !pointerValueFuncs[sel.Sel.Name] {
		return false
	}
	// Verify it's the types package.
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

// formatDefault formats a default value for display in diagnostics.
func formatDefault(v any) string {
	if v == nil {
		return "<unknown>"
	}
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%g", val)
		}
		return fmt.Sprintf("%g", val)
	case int64:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}
