// Package overlaycheck validates that overlay functions correctly handle all
// schema fields according to their classification:
//
//   - Computed-only fields must be unconditionally assigned from resolved
//   - Required/Optional fields must NOT be assigned in the overlay
//   - Optional+Computed fields must be guarded by IsUnknown() before assignment
//
// This analyzer depends on schemaparser.Analyzer for field classifications.
// It finds overlay functions by signature pattern: functions that accept both an
// API response type and a model pointer type, where the model type matches a
// known schema resource model.
package overlaycheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for overlaycheck.
var Analyzer = &analysis.Analyzer{
	Name:     "overlaycheck",
	Doc:      "Validates overlay functions handle all schema fields correctly per their classification.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

// fieldAssignment tracks how a field is assigned in the overlay function.
type fieldAssignment struct {
	// unconditional is true if plan.X = resolved.X (direct assignment).
	unconditional bool
	// guardedByIsUnknown is true if inside `if plan.X.IsUnknown() { ... }`.
	guardedByIsUnknown bool
	// pos is the position of the assignment for error reporting.
	pos token.Pos
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Collect schema facts from all packages (current + imported).
	// The schemaparser exports SchemaFacts as a package fact on each resource_*
	// package. We need to aggregate them all since overlay functions in
	// internal/provider reference schemas from imported resource_* packages.
	allSchemas := make(map[string]*schemaparser.SchemaInfo)

	// From current package (if it has schemas).
	if localFacts, ok := pass.ResultOf[schemaparser.Analyzer].(*schemaparser.SchemaFacts); ok && localFacts != nil {
		for name, info := range localFacts.Schemas {
			allSchemas[name] = info
		}
	}

	// From imported packages (cross-package facts).
	for _, fact := range pass.AllPackageFacts() {
		if sf, ok := fact.Fact.(*schemaparser.SchemaFacts); ok {
			for name, info := range sf.Schemas {
				allSchemas[name] = info
			}
		}
	}

	if len(allSchemas) == 0 {
		return nil, nil
	}

	// Build an aggregated facts struct for matching.
	facts := &schemaparser.SchemaFacts{Schemas: allSchemas}

	// Find overlay functions. These are functions whose name starts with "overlay"
	// and ends with "ComputedFields" (top-level overlays).
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil {
			return
		}
		name := fn.Name.Name
		if !isTopLevelOverlay(name) {
			return
		}

		// Try to match this overlay function to a known schema.
		schemaInfo := matchOverlayToSchema(name, facts)
		if schemaInfo == nil {
			return
		}

		// Determine the plan parameter name (the last pointer parameter).
		planParamName := findPlanParamName(fn)
		if planParamName == "" {
			return
		}

		// Analyze the function body for field assignments.
		assignments := analyzeOverlayBody(fn.Body, planParamName)

		// Cross-reference assignments against schema classification.
		validateOverlay(pass, fn, schemaInfo, assignments, planParamName)
	})

	return nil, nil
}

// isTopLevelOverlay checks if a function name matches the top-level overlay pattern.
func isTopLevelOverlay(name string) bool {
	return strings.HasPrefix(name, "overlay") && strings.HasSuffix(name, "ComputedFields")
}

// matchOverlayToSchema attempts to match an overlay function name to a known schema.
// E.g., "overlayBudgetComputedFields" → "BudgetResourceSchema".
func matchOverlayToSchema(overlayName string, facts *schemaparser.SchemaFacts) *schemaparser.SchemaInfo {
	// Extract the resource name: "overlayBudgetComputedFields" → "Budget"
	trimmed := strings.TrimPrefix(overlayName, "overlay")
	trimmed = strings.TrimSuffix(trimmed, "ComputedFields")

	// Try to find a matching schema.
	schemaName := trimmed + "ResourceSchema"
	if info, ok := facts.Schemas[schemaName]; ok {
		return info
	}

	// Try data source schema.
	schemaName = trimmed + "DataSourceSchema"
	if info, ok := facts.Schemas[schemaName]; ok {
		return info
	}

	return nil
}

// findPlanParamName returns the name of the plan/model parameter.
// Convention: the last parameter that is a pointer type is the plan.
func findPlanParamName(fn *ast.FuncDecl) string {
	if fn.Type.Params == nil {
		return ""
	}
	params := fn.Type.Params.List
	// Walk backward to find the last pointer parameter.
	for i := len(params) - 1; i >= 0; i-- {
		p := params[i]
		if _, ok := p.Type.(*ast.StarExpr); ok {
			if len(p.Names) > 0 {
				return p.Names[0].Name
			}
		}
	}
	return ""
}

// analyzeOverlayBody walks the overlay function body and records all
// assignments to plan.FieldName, noting whether they are unconditional
// or guarded by IsUnknown().
func analyzeOverlayBody(body *ast.BlockStmt, planParam string) map[string]*fieldAssignment {
	result := make(map[string]*fieldAssignment)

	for _, stmt := range body.List {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			// Direct assignment: plan.X = resolved.X
			for _, lhs := range s.Lhs {
				fieldName := selectorFieldOn(lhs, planParam)
				if fieldName != "" {
					result[fieldName] = &fieldAssignment{
						unconditional: true,
						pos:           s.Pos(),
					}
				}
			}

		case *ast.IfStmt:
			// Check if this is `if plan.X.IsUnknown() { ... }`
			guardedField := extractIsUnknownGuard(s.Cond, planParam)
			if guardedField != "" {
				// Look for assignments inside the if body.
				for _, innerStmt := range s.Body.List {
					if assign, ok := innerStmt.(*ast.AssignStmt); ok {
						for _, lhs := range assign.Lhs {
							fieldName := selectorFieldOn(lhs, planParam)
							if fieldName != "" {
								result[fieldName] = &fieldAssignment{
									guardedByIsUnknown: true,
									pos:                assign.Pos(),
								}
							}
						}
					}
				}
				// If no explicit assignment found in the body but the guard is
				// present, record the guarded field as IsUnknown-guarded.
				if _, exists := result[guardedField]; !exists {
					result[guardedField] = &fieldAssignment{
						guardedByIsUnknown: true,
						pos:                s.Pos(),
					}
				}
			}

			// Also handle if-else chains: `if plan.X.IsUnknown() { ... } else if !plan.X.IsNull() { ... }`
			// The else-if branch contains list overlayListElements calls — we track those too.
			if s.Else != nil {
				walkElseChainForAssignments(s.Else, planParam, result, s.Pos())
			}

		case *ast.ExprStmt:
			// Handle: diags.Append(overlayListElements(..., &plan.X, ...)...)
			// These indicate the field is being handled (for list overlay).
			handleExprForListOverlay(s.X, planParam, result, s.Pos())
		}
	}

	return result
}

// walkElseChainForAssignments handles else/else-if blocks.
func walkElseChainForAssignments(elseStmt ast.Stmt, planParam string, result map[string]*fieldAssignment, pos token.Pos) {
	switch e := elseStmt.(type) {
	case *ast.BlockStmt:
		for _, stmt := range e.List {
			if assign, ok := stmt.(*ast.AssignStmt); ok {
				for _, lhs := range assign.Lhs {
					fieldName := selectorFieldOn(lhs, planParam)
					if fieldName != "" {
						if _, exists := result[fieldName]; !exists {
							result[fieldName] = &fieldAssignment{
								guardedByIsUnknown: true,
								pos:                assign.Pos(),
							}
						}
					}
				}
			}
		}
	case *ast.IfStmt:
		if e.Body != nil {
			for _, stmt := range e.Body.List {
				if es, ok := stmt.(*ast.ExprStmt); ok {
					handleExprForListOverlay(es.X, planParam, result, pos)
				}
			}
		}
		if e.Else != nil {
			walkElseChainForAssignments(e.Else, planParam, result, pos)
		}
	}
}

// handleExprForListOverlay detects diags.Append(overlayListElements(ctx, &resolved.X, &plan.X, ...)...)
// and similar patterns where list fields are overlaid.
func handleExprForListOverlay(expr ast.Expr, planParam string, result map[string]*fieldAssignment, pos token.Pos) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return
	}
	// Look through the call for &plan.X patterns in arguments.
	ast.Inspect(call, func(n ast.Node) bool {
		unary, ok := n.(*ast.UnaryExpr)
		if !ok || unary.Op != token.AND {
			return true
		}
		fieldName := selectorFieldOn(unary.X, planParam)
		if fieldName != "" {
			if _, exists := result[fieldName]; !exists {
				result[fieldName] = &fieldAssignment{
					guardedByIsUnknown: true,
					pos:                pos,
				}
			}
		}
		return true
	})
}

// selectorFieldOn extracts the field name from an expression of the form `obj.FieldName`
// where obj matches the expected parameter name.
func selectorFieldOn(expr ast.Expr, objName string) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	if ident.Name != objName {
		return ""
	}
	return toSnakeCase(sel.Sel.Name)
}

// extractIsUnknownGuard checks if a condition is `plan.X.IsUnknown()` and
// returns the field name X.
func extractIsUnknownGuard(cond ast.Expr, planParam string) string {
	call, ok := cond.(*ast.CallExpr)
	if !ok {
		return ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "IsUnknown" {
		return ""
	}
	// sel.X should be plan.FieldName
	return selectorFieldOn(sel.X, planParam)
}

// validateOverlay checks the overlay against the schema classification.
func validateOverlay(pass *analysis.Pass, fn *ast.FuncDecl, schema *schemaparser.SchemaInfo, assignments map[string]*fieldAssignment, planParam string) {
	for attrName, attrInfo := range schema.Attrs {
		// Skip the "timeouts" attribute — it's framework-managed.
		if attrName == "timeouts" {
			continue
		}

		assignment, hasAssignment := assignments[attrName]

		switch attrInfo.Class {
		case schemaparser.ComputedOnly:
			if !hasAssignment {
				pass.Reportf(fn.Pos(),
					"%s: Computed-only field %q is not set from API response; "+
						"add unconditional assignment: %s.%s = resolved.%s",
					fn.Name.Name, attrName, planParam, toPascalCase(attrName), toPascalCase(attrName))
			} else if assignment.guardedByIsUnknown {
				pass.Reportf(assignment.pos,
					"%s: Computed-only field %q should be assigned unconditionally (not guarded by IsUnknown)",
					fn.Name.Name, attrName)
			}

		case schemaparser.Required:
			if hasAssignment {
				pass.Reportf(assignment.pos,
					"%s: Required field %q must not be assigned in overlay; "+
						"the plan value should be preserved",
					fn.Name.Name, attrName)
			}

		case schemaparser.Optional:
			if hasAssignment {
				pass.Reportf(assignment.pos,
					"%s: Optional field %q must not be assigned in overlay; "+
						"the plan value should be preserved",
					fn.Name.Name, attrName)
			}

		case schemaparser.OptionalComputed:
			if !hasAssignment {
				pass.Reportf(fn.Pos(),
					"%s: Optional+Computed field %q is not handled; "+
						"add: if %s.%s.IsUnknown() { %s.%s = resolved.%s }",
					fn.Name.Name, attrName,
					planParam, toPascalCase(attrName),
					planParam, toPascalCase(attrName), toPascalCase(attrName))
			} else if assignment.unconditional {
				pass.Reportf(assignment.pos,
					"%s: Optional+Computed field %q is assigned unconditionally; "+
						"must be guarded by IsUnknown() to preserve user-configured values",
					fn.Name.Name, attrName)
			}
		}
	}
}

// toSnakeCase converts PascalCase/camelCase to snake_case.
// e.g., "CreateTime" → "create_time", "Id" → "id".
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32) // lowercase
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// toPascalCase converts snake_case to PascalCase.
// e.g., "create_time" → "CreateTime", "id" → "Id".
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	var result strings.Builder
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		result.WriteString(fmt.Sprintf("%s%s", strings.ToUpper(part[:1]), part[1:]))
	}
	return result.String()
}
