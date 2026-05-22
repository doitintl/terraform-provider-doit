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
// their classification.
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

func run(pass *analysis.Pass) (interface{}, error) {
	result := pass.ResultOf[schemaparser.Analyzer]
	if result == nil {
		return nil, nil
	}
	schemaResult, ok := result.(*schemaparser.SchemaFacts)
	if !ok || schemaResult == nil {
		return nil, nil
	}

	// Build a set of list fields that are Optional or Optional+Computed.
	userConfigurableLists := map[string]bool{}
	for _, schemaInfo := range schemaResult.Schemas {
		for fieldName, info := range schemaInfo.Attrs {
			if info.IsList && (info.Class == schemaparser.Optional || info.Class == schemaparser.OptionalComputed) {
				userConfigurableLists[fieldName] = true
			}
		}
	}

	if len(userConfigurableLists) == 0 {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Find mapping functions (populateState, mapXxxToModel).
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil {
			return
		}
		name := fn.Name.Name
		if !isMappingFunction(name) {
			return
		}

		// Find ListNull calls in the function body and check if they're
		// assigned to user-configurable list fields.
		findListNullAssignments(pass, fn.Body, userConfigurableLists)
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

