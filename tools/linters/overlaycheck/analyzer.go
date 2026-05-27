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

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// The schemaparser result aggregates schemas from both the current package
	// and all imported packages (via vertical fact inheritance). This gives us
	// the complete picture of all schemas visible from this package.
	facts := pass.ResultOf[schemaparser.Analyzer].(*schemaparser.SchemaFacts)

	if len(facts.Schemas) == 0 {
		return nil, nil
	}

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

		// Determine the plan parameter name (the last pointer parameter).
		planParamName := findPlanParamName(fn)
		if planParamName == "" {
			return
		}

		// Validate the 2-phase pattern: overlay must create a resolved copy
		// and call a mapping function. This check runs on ALL overlay functions
		// regardless of whether a matching schema is found.
		validate2PhasePattern(pass, fn, planParamName)

		// Schema-aware field validation requires a matching schema.
		schemaInfo := matchOverlayToSchema(name, facts)
		if schemaInfo == nil {
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
					switch inner := innerStmt.(type) {
					case *ast.AssignStmt:
						for _, lhs := range inner.Lhs {
							fieldName := selectorFieldOn(lhs, planParam)
							if fieldName != "" {
								result[fieldName] = &fieldAssignment{
									guardedByIsUnknown: true,
									pos:                inner.Pos(),
								}
							}
						}
					case *ast.ExprStmt:
						// Handle &plan.X passed to helper functions inside if body.
						handleExprForListOverlay(inner.X, planParam, result, inner.Pos())
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

				// Also handle if-else chains after IsUnknown guard:
				// `if plan.X.IsUnknown() { ... } else if !plan.X.IsNull() { ... }`
				if s.Else != nil {
					walkElseChainForAssignments(s.Else, planParam, result, s.Pos())
				}
			} else {
				// Not an IsUnknown guard. Check for if/else covering all paths:
				// `if apiResp.X != nil { plan.F = val } else { plan.F = null }`
				// Both branches assign to plan.F → treat as unconditional.
				detectIfElseUnconditional(s, planParam, result)

				// Also scan for &plan.X passed to helper functions inside if-body
				// and else branches. This catches patterns like:
				//   if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
				//       overlayConfigFields(ctx, &resolved.Config, &plan.Config)
				//   } else if plan.Config.IsUnknown() {
				//       plan.Config = resolved.Config
				//   }
				scanIfTreeForFieldHandling(s, planParam, result)
			}

		case *ast.ExprStmt:
			// Handle: diags.Append(overlayListElements(..., &plan.X, ...)...)
			// These indicate the field is being handled (for list overlay).
			handleExprForListOverlay(s.X, planParam, result, s.Pos())
		}
	}

	return result
}

// detectIfElseUnconditional checks if an if/else assigns to the same plan field
// in both branches (covering all paths). E.g.:
//
//	if apiResp.X != nil { plan.F = types.Int64Value(*apiResp.X) } else { plan.F = types.Int64Null() }
//
// This is semantically unconditional for the plan field.
func detectIfElseUnconditional(ifStmt *ast.IfStmt, planParam string, result map[string]*fieldAssignment) {
	if ifStmt.Else == nil {
		return // no else branch → not covering all paths
	}

	// Collect plan field assignments from the if-body.
	ifFields := collectPlanAssignments(ifStmt.Body, planParam)

	// Collect plan field assignments from the else branch.
	var elseFields map[string]token.Pos
	switch e := ifStmt.Else.(type) {
	case *ast.BlockStmt:
		elseFields = collectPlanAssignments(e, planParam)
	case *ast.IfStmt:
		// else-if: only treat as unconditional if the else-if also has an else.
		// For simplicity, don't recurse into chained else-ifs.
		return
	}

	// Fields assigned in BOTH branches are unconditional.
	for fieldName, pos := range ifFields {
		if _, inElse := elseFields[fieldName]; inElse {
			// Already recorded? Don't override a more specific classification.
			if _, exists := result[fieldName]; !exists {
				result[fieldName] = &fieldAssignment{
					unconditional: true,
					pos:           pos,
				}
			}
		}
	}
}

// collectPlanAssignments extracts all plan.X = ... assignments from a block.
func collectPlanAssignments(block *ast.BlockStmt, planParam string) map[string]token.Pos {
	fields := make(map[string]token.Pos)
	for _, stmt := range block.List {
		if assign, ok := stmt.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				fieldName := selectorFieldOn(lhs, planParam)
				if fieldName != "" {
					fields[fieldName] = assign.Pos()
				}
			}
		}
	}
	return fields
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

// scanIfTreeForFieldHandling recursively scans an if/else-if/else tree for
// &plan.X patterns in function calls and plan.X = assignments. This is used for
// if-statements that don't have a simple IsUnknown guard, such as:
//
//	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
//	    diags.Append(overlayConfigFields(ctx, &resolved.Config, &plan.Config)...)
//	} else if plan.Config.IsUnknown() {
//	    plan.Config = resolved.Config
//	}
func scanIfTreeForFieldHandling(ifStmt *ast.IfStmt, planParam string, result map[string]*fieldAssignment) {
	// Scan the if-body.
	scanBlockForFieldHandling(ifStmt.Body, planParam, result)

	// Scan else branches.
	if ifStmt.Else != nil {
		switch e := ifStmt.Else.(type) {
		case *ast.BlockStmt:
			scanBlockForFieldHandling(e, planParam, result)
		case *ast.IfStmt:
			// Also check if the else-if has an IsUnknown guard for an assignment.
			guardedField := extractIsUnknownGuard(e.Cond, planParam)
			if guardedField != "" {
				for _, innerStmt := range e.Body.List {
					if assign, ok := innerStmt.(*ast.AssignStmt); ok {
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
			}
			scanIfTreeForFieldHandling(e, planParam, result)
		}
	}
}

// scanBlockForFieldHandling scans a block for &plan.X patterns in function calls
// and plan.X = assignments.
func scanBlockForFieldHandling(block *ast.BlockStmt, planParam string, result map[string]*fieldAssignment) {
	for _, stmt := range block.List {
		switch s := stmt.(type) {
		case *ast.ExprStmt:
			handleExprForListOverlay(s.X, planParam, result, s.Pos())
		case *ast.AssignStmt:
			for _, lhs := range s.Lhs {
				fieldName := selectorFieldOn(lhs, planParam)
				if fieldName != "" {
					if _, exists := result[fieldName]; !exists {
						result[fieldName] = &fieldAssignment{
							guardedByIsUnknown: true,
							pos:                s.Pos(),
						}
					}
				}
			}
		}
	}
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
	// Collect missing fields by category to aggregate into single diagnostics.
	// This avoids golangci-lint deduplicating multiple reports at fn.Pos().
	var missingComputed []string
	var unhandledOptComp []string

	for attrName, attrInfo := range schema.Attrs {
		// Skip the "timeouts" attribute — it's framework-managed.
		if attrName == "timeouts" {
			continue
		}

		assignment, hasAssignment := assignments[attrName]

		switch attrInfo.Class {
		case schemaparser.ComputedOnly:
			if !hasAssignment {
				missingComputed = append(missingComputed, attrName)
			} else if assignment.guardedByIsUnknown {
				pass.Reportf(assignment.pos,
					"%s: Computed-only field %q should be assigned unconditionally (not guarded by IsUnknown)",
					fn.Name.Name, attrName)
			}

		case schemaparser.Required:
			if hasAssignment {
				// Allow Required nested objects that have Optional+Computed children.
				// The correct pattern is: if plan.Config.IsUnknown() { plan.Config = resolved }
				// else { overlaySubfields() }. The IsUnknown guard on a Required field
				// is defensive and acceptable when the field has nested subfields.
				if assignment.guardedByIsUnknown && attrInfo.NestedAttrs != nil {
					continue
				}
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
			// Fields with a schema Default are resolved at plan time and are
			// never Unknown, so they don't need an IsUnknown() guard.
			if attrInfo.HasDefault {
				continue
			}
			if !hasAssignment {
				unhandledOptComp = append(unhandledOptComp, attrName)
			} else if assignment.unconditional {
				pass.Reportf(assignment.pos,
					"%s: Optional+Computed field %q is assigned unconditionally; "+
						"must be guarded by IsUnknown() to preserve user-configured values",
					fn.Name.Name, attrName)
			}
		}
	}

	// Report aggregated missing fields.
	if len(missingComputed) > 0 {
		pass.Reportf(fn.Pos(),
			"%s: Computed-only field(s) not set from API response: %s; "+
				"add unconditional assignment: %s.<Field> = resolved.<Field>",
			fn.Name.Name, strings.Join(missingComputed, ", "), planParam)
	}
	if len(unhandledOptComp) > 0 {
		pass.Reportf(fn.Pos(),
			"%s: Optional+Computed field(s) not handled: %s; "+
				"add: if %s.<Field>.IsUnknown() { %s.<Field> = resolved.<Field> }",
			fn.Name.Name, strings.Join(unhandledOptComp, ", "), planParam, planParam)
	}
}

// validate2PhasePattern checks that an overlay function uses the standard
// 2-phase pattern:
//
//	Phase 1: resolved := *plan; mapXxxToModel(apiResp, &resolved)
//	Phase 2: plan.X = resolved.X
//
// Variants are accepted:
//   - resolved := *plan (copy dereference)
//   - var resolved ModelType (zero value)
//   - resolved := ModelType{...} (struct literal)
//
// The mapping function can be any of: mapXxxToModel, populateState, mapResourceToModel,
// or any function matching map*ToModel / map*ToResourceModel.
func validate2PhasePattern(pass *analysis.Pass, fn *ast.FuncDecl, planParam string) {
	hasResolvedVar := false
	hasMappingCall := false

	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.AssignStmt:
			// Check for `resolved := *plan` or `resolved := ModelType{...}`
			for _, lhs := range n.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || ident.Name != "resolved" {
					continue
				}
				hasResolvedVar = true
			}

		case *ast.DeclStmt:
			// Check for `var resolved ModelType`
			genDecl, ok := n.Decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.VAR {
				break
			}
			for _, spec := range genDecl.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if name.Name == "resolved" {
						hasResolvedVar = true
					}
				}
			}

		case *ast.CallExpr:
			// Check for mapping function call (mapXxxToModel, populateState, etc.)
			calledName := ""
			switch fn := n.Fun.(type) {
			case *ast.Ident:
				calledName = fn.Name
			case *ast.SelectorExpr:
				calledName = fn.Sel.Name
			}
			if calledName != "" && isMappingFunc(calledName) {
				hasMappingCall = true
			}
		}
		return true
	})

	if !hasResolvedVar {
		pass.Reportf(fn.Pos(),
			"%s: missing 2-phase pattern; overlay must create a 'resolved' copy "+
				"(e.g., 'resolved := *%s') and call a mapping function (mapXxxToModel/populateState)",
			fn.Name.Name, planParam)
	} else if !hasMappingCall {
		pass.Reportf(fn.Pos(),
			"%s: missing mapping function call; overlay must call a mapping function "+
				"(e.g., mapXxxToModel, populateState) to populate the resolved copy from the API response",
			fn.Name.Name)
	}
}

// isMappingFunc checks if a function name matches a mapping function pattern.
func isMappingFunc(name string) bool {
	if name == "mapResourceToModel" || name == "populateState" {
		return true
	}
	if strings.HasPrefix(name, "map") && (strings.HasSuffix(name, "ToModel") || strings.HasSuffix(name, "ToResourceModel")) {
		return true
	}
	return false
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
