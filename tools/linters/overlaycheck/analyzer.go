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
	"go/types"
	"reflect"
	"sort"
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

// subOverlayCall records a call to a sub-overlay function and the field it operates on.
type subOverlayCall struct {
	funcName  string // e.g., "overlayBudgetAlert"
	fieldName string // e.g., "alerts" (snake_case)
	pos       token.Pos
}

// overlayBodyResult holds the results of analyzing an overlay function body.
type overlayBodyResult struct {
	assignments map[string]*fieldAssignment
	subOverlays []subOverlayCall
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

	// Build a map of all function declarations for sub-overlay lookups.
	funcDecls := buildFuncDeclMap(insp)

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

		// Analyze the function body for field assignments and sub-overlay calls.
		bodyResult := analyzeOverlayBody(fn.Body, planParamName)

		// Resolve Go field names to schema keys via tfsdk struct tags.
		tagMap := buildTagMap(pass, fn)
		resolveFieldNames(&bodyResult, tagMap)

		// Cross-reference assignments against schema classification.
		validateOverlay(pass, fn, schemaInfo, bodyResult.assignments, planParamName)

		// Enforce that nested attributes with computed fields use sub-overlay helpers.
		enforceSubOverlayHelpers(pass, fn, schemaInfo, bodyResult.subOverlays)

		// Recursively validate sub-overlay functions against nested schemas.
		validateSubOverlays(pass, schemaInfo, bodyResult.subOverlays, funcDecls, nil)
	})

	return nil, nil
}

// buildFuncDeclMap collects all function declarations in the package,
// keyed by function name. Used for looking up sub-overlay functions.
func buildFuncDeclMap(insp *inspector.Inspector) map[string]*ast.FuncDecl {
	result := make(map[string]*ast.FuncDecl)
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name != nil && fn.Body != nil {
			result[fn.Name.Name] = fn
		}
	})
	return result
}

// validateSubOverlays recursively validates sub-overlay functions against
// the nested schemas of their parent fields.
func validateSubOverlays(
	pass *analysis.Pass,
	parentSchema *schemaparser.SchemaInfo,
	calls []subOverlayCall,
	funcDecls map[string]*ast.FuncDecl,
	visited map[string]bool,
) {
	if visited == nil {
		visited = make(map[string]bool)
	}

	for _, call := range calls {
		// Prevent infinite recursion.
		if visited[call.funcName] {
			continue
		}
		visited[call.funcName] = true

		// Look up the nested schema for this field.
		parentAttr, ok := parentSchema.Attrs[call.fieldName]
		if !ok || parentAttr.NestedAttrs == nil {
			continue // no nested schema to validate against
		}

		// Find the sub-overlay function declaration.
		subFn, ok := funcDecls[call.funcName]
		if !ok {
			continue // function not found (may be in another package)
		}

		// Determine the plan parameter (second pointer param for sub-overlays).
		planParam := findPlanParamName(subFn)
		if planParam == "" {
			continue
		}

		// Build a synthetic SchemaInfo from the nested attrs.
		nestedSchema := &schemaparser.SchemaInfo{
			Attrs: parentAttr.NestedAttrs,
		}

		// Analyze and validate the sub-overlay body.
		bodyResult := analyzeOverlayBody(subFn.Body, planParam)

		// Resolve Go field names to schema keys via tfsdk struct tags.
		tagMap := buildTagMap(pass, subFn)
		resolveFieldNames(&bodyResult, tagMap)

		validateOverlay(pass, subFn, nestedSchema, bodyResult.assignments, planParam)

		// Enforce sub-overlay helpers for deeper nested attrs too.
		enforceSubOverlayHelpers(pass, subFn, nestedSchema, bodyResult.subOverlays)

		// Recurse into deeper sub-overlays.
		validateSubOverlays(pass, nestedSchema, bodyResult.subOverlays, funcDecls, visited)
	}
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
// When multiple names share a type (e.g., `resolved, plan *Type`),
// the last name is the plan parameter.
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
				return p.Names[len(p.Names)-1].Name
			}
		}
	}
	return ""
}

// analyzeOverlayBody walks the overlay function body and records all
// assignments to plan.FieldName, noting whether they are unconditional
// or guarded by IsUnknown(). It also records calls to sub-overlay functions.
func analyzeOverlayBody(body *ast.BlockStmt, planParam string) overlayBodyResult {
	result := overlayBodyResult{
		assignments: make(map[string]*fieldAssignment),
	}

	for _, stmt := range body.List {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			// Direct assignment: plan.X = resolved.X
			for _, lhs := range s.Lhs {
				fieldName := selectorFieldOn(lhs, planParam)
				if fieldName != "" {
					result.assignments[fieldName] = &fieldAssignment{
						unconditional: true,
						pos:           s.Pos(),
					}
				}
			}
			// Also check RHS for sub-overlay calls in assignments like:
			// diags = overlayListElements(ctx, &resolved.X, &plan.X, overlayFn)
			for _, rhs := range s.Rhs {
				collectSubOverlayCallsFromExpr(rhs, planParam, &result)
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
								result.assignments[fieldName] = &fieldAssignment{
									guardedByIsUnknown: true,
									pos:                inner.Pos(),
								}
							}
						}
					case *ast.ExprStmt:
						// Handle &plan.X passed to helper functions inside if body.
						handleExprForListOverlay(inner.X, planParam, result.assignments, inner.Pos())
					}
				}
				// If no explicit assignment found in the body but the guard is
				// present, record the guarded field as IsUnknown-guarded.
				if _, exists := result.assignments[guardedField]; !exists {
					result.assignments[guardedField] = &fieldAssignment{
						guardedByIsUnknown: true,
						pos:                s.Pos(),
					}
				}

				// Also handle if-else chains after IsUnknown guard:
				// `if plan.X.IsUnknown() { ... } else if !plan.X.IsNull() { ... }`
				if s.Else != nil {
					walkElseChainForAssignments(s.Else, planParam, result.assignments, s.Pos())
					// Also collect sub-overlay calls from the else chain.
					// This catches: else if !plan.X.IsNull() { overlayListElements(..., overlayFn) }
					collectSubOverlayCallsFromElse(s.Else, planParam, &result)
				}
			} else {
				// Not an IsUnknown guard. Check for if/else covering all paths:
				// `if apiResp.X != nil { plan.F = val } else { plan.F = null }`
				// Both branches assign to plan.F → treat as unconditional.
				detectIfElseUnconditional(s, planParam, result.assignments)

				// Also scan for &plan.X passed to helper functions inside if-body
				// and else branches. This catches patterns like:
				//   if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
				//       overlayConfigFields(ctx, &resolved.Config, &plan.Config)
				//   } else if plan.Config.IsUnknown() {
				//       plan.Config = resolved.Config
				//   }
				scanIfTreeForFieldHandling(s, planParam, result.assignments)

				// Track sub-overlay calls within if-trees.
				collectSubOverlayCalls(s, planParam, &result)
			}

		case *ast.ExprStmt:
			// Handle: diags.Append(overlayListElements(..., &plan.X, ...)...)
			// These indicate the field is being handled (for list overlay).
			handleExprForListOverlay(s.X, planParam, result.assignments, s.Pos())

			// Track sub-overlay calls (overlayListElements with callback, or direct calls).
			collectSubOverlayCallsFromExpr(s.X, planParam, &result)
		}
	}

	return result
}

// collectSubOverlayCalls extracts sub-overlay function calls from an if-tree.
// Handles patterns like:
//
//	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
//	    diags.Append(overlayConfigFields(ctx, &resolved.Config, &plan.Config)...)
//	}
//
// collectSubOverlayCallsFromElse walks an else chain and collects sub-overlay calls.
func collectSubOverlayCallsFromElse(elseStmt ast.Stmt, planParam string, result *overlayBodyResult) {
	switch e := elseStmt.(type) {
	case *ast.BlockStmt:
		for _, stmt := range e.List {
			if es, ok := stmt.(*ast.ExprStmt); ok {
				collectSubOverlayCallsFromExpr(es.X, planParam, result)
			}
			if as, ok := stmt.(*ast.AssignStmt); ok {
				for _, rhs := range as.Rhs {
					collectSubOverlayCallsFromExpr(rhs, planParam, result)
				}
			}
			if ret, ok := stmt.(*ast.ReturnStmt); ok {
				for _, r := range ret.Results {
					collectSubOverlayCallsFromExpr(r, planParam, result)
				}
			}
		}
	case *ast.IfStmt:
		collectSubOverlayCalls(e, planParam, result)
	}
}

func collectSubOverlayCalls(ifStmt *ast.IfStmt, planParam string, result *overlayBodyResult) {
	if ifStmt.Body != nil {
		for _, stmt := range ifStmt.Body.List {
			if es, ok := stmt.(*ast.ExprStmt); ok {
				collectSubOverlayCallsFromExpr(es.X, planParam, result)
			}
			if as, ok := stmt.(*ast.AssignStmt); ok {
				for _, rhs := range as.Rhs {
					collectSubOverlayCallsFromExpr(rhs, planParam, result)
				}
			}
			if ret, ok := stmt.(*ast.ReturnStmt); ok {
				for _, r := range ret.Results {
					collectSubOverlayCallsFromExpr(r, planParam, result)
				}
			}
		}
	}
	// Recurse into else branches.
	if ifStmt.Else != nil {
		switch e := ifStmt.Else.(type) {
		case *ast.IfStmt:
			collectSubOverlayCalls(e, planParam, result)
		case *ast.BlockStmt:
			for _, stmt := range e.List {
				if es, ok := stmt.(*ast.ExprStmt); ok {
					collectSubOverlayCallsFromExpr(es.X, planParam, result)
				}
			}
		}
	}
}

// collectSubOverlayCallsFromExpr inspects an expression for sub-overlay
// function calls. It detects two patterns:
//
//  1. overlayListElements(ctx, &resolved.X, &plan.X, overlayCallback)
//     → records overlayCallback as sub-overlay for field X
//
//  2. overlayHelperFunc(&resolved.X, &plan.X) or overlayHelperFunc(ctx, &resolved.X, &plan.X)
//     → records overlayHelperFunc as sub-overlay for field X
func collectSubOverlayCallsFromExpr(expr ast.Expr, planParam string, result *overlayBodyResult) {
	ast.Inspect(expr, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		calledName := calledFuncNameFromExpr(call)
		if calledName == "" {
			return true
		}

		// Pattern 1: overlayListElements(ctx, &resolved.X, &plan.X, overlayCallback)
		if calledName == "overlayListElements" || calledName == "overlayListElementsByKey" {
			if len(call.Args) >= 4 {
				// The plan field is the third argument: &plan.X
				fieldName := extractFieldFromAddrExpr(call.Args[2], planParam)
				// The callback is the last argument
				callbackName := identName(call.Args[len(call.Args)-1])
				if fieldName != "" && callbackName != "" && strings.HasPrefix(callbackName, "overlay") {
					result.subOverlays = append(result.subOverlays, subOverlayCall{
						funcName:  callbackName,
						fieldName: fieldName,
						pos:       call.Pos(),
					})
				}
			}
			return false // don't recurse into overlayListElements args
		}

		// Pattern 2: overlayHelperFunc(..., &plan.X) — direct sub-overlay call
		if strings.HasPrefix(calledName, "overlay") && !isTopLevelOverlay(calledName) {
			for _, arg := range call.Args {
				fieldName := extractFieldFromAddrExpr(arg, planParam)
				if fieldName != "" {
					result.subOverlays = append(result.subOverlays, subOverlayCall{
						funcName:  calledName,
						fieldName: fieldName,
						pos:       call.Pos(),
					})
					break // one field per sub-overlay call
				}
			}
		}

		return true
	})
}

// extractFieldFromAddrExpr extracts the field name from &planParam.FieldName.
func extractFieldFromAddrExpr(expr ast.Expr, planParam string) string {
	unary, ok := expr.(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return ""
	}
	return selectorFieldOn(unary.X, planParam)
}

// calledFuncNameFromExpr extracts the function name from a call expression,
// handling both direct calls and selector calls (r.method()).
func calledFuncNameFromExpr(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

// identName extracts the name from an *ast.Ident.
func identName(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
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
			if !hasAssignment {
				// Fields with a schema Default are resolved at plan time and are
				// never Unknown, so they don't need to be handled in the overlay.
				if !attrInfo.HasDefault {
					unhandledOptComp = append(unhandledOptComp, attrName)
				}
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

// enforceSubOverlayHelpers verifies that nested attributes with Computed-only
// or Optional+Computed children use sub-overlay helper functions (e.g.,
// overlayListElements callbacks or direct overlay* calls) rather than inline
// handling. This ensures consistent patterns across all overlay functions.
//
// Missing attributes are aggregated into a single diagnostic to avoid
// golangci-lint's --uniq-by-line deduplication hiding findings.
func enforceSubOverlayHelpers(
	pass *analysis.Pass,
	fn *ast.FuncDecl,
	schema *schemaparser.SchemaInfo,
	subOverlays []subOverlayCall,
) {
	// Build a set of field names that have sub-overlay calls.
	coveredFields := make(map[string]bool, len(subOverlays))
	for _, call := range subOverlays {
		coveredFields[call.fieldName] = true
	}

	var missing []string
	for attrName, attrInfo := range schema.Attrs {
		if attrInfo.NestedAttrs == nil {
			continue
		}
		// Computed-only nested objects are always assigned unconditionally
		// (plan.X = resolved.X), so no sub-overlay helper is needed.
		if attrInfo.Class == schemaparser.ComputedOnly {
			continue
		}
		// Only check nested attrs that contain fields needing overlay.
		if !nestedHasOverlayFields(attrInfo.NestedAttrs) {
			continue
		}
		// Skip if a sub-overlay call already exists for this field.
		if coveredFields[attrName] {
			continue
		}
		missing = append(missing, attrName)
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		pass.Reportf(fn.Pos(),
			"%s: nested attribute(s) with computed fields need sub-overlay helpers: %s",
			fn.Name.Name, strings.Join(missing, ", "))
	}
}

// nestedHasOverlayFields returns true if any attribute in the map is
// Computed-only or Optional+Computed (i.e., fields that require overlay handling).
func nestedHasOverlayFields(attrs map[string]*schemaparser.AttrInfo) bool {
	for _, attr := range attrs {
		if attr.Class == schemaparser.ComputedOnly || attr.Class == schemaparser.OptionalComputed {
			return true
		}
		// Recurse into deeper nested attrs.
		if attr.NestedAttrs != nil && nestedHasOverlayFields(attr.NestedAttrs) {
			return true
		}
	}
	return false
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

// buildTagMap extracts a Go-field-name → tfsdk-tag-name mapping from the plan
// parameter's struct type. This handles cases where the code generator prefixes
// Go field names to avoid keyword collisions (e.g., DimensionsType → tfsdk:"type").
//
// Returns nil if the plan parameter's type cannot be resolved.
func buildTagMap(pass *analysis.Pass, fn *ast.FuncDecl) map[string]string {
	if fn.Type.Params == nil || pass.TypesInfo == nil {
		return nil
	}

	// Find the plan parameter (last pointer parameter).
	params := fn.Type.Params.List
	var planField *ast.Field
	for i := len(params) - 1; i >= 0; i-- {
		if _, ok := params[i].Type.(*ast.StarExpr); ok {
			planField = params[i]
			break
		}
	}
	if planField == nil || len(planField.Names) == 0 {
		return nil
	}

	// Resolve the plan parameter's type via the type checker.
	planIdent := planField.Names[len(planField.Names)-1]
	obj := pass.TypesInfo.Defs[planIdent]
	if obj == nil {
		return nil
	}

	// Dereference pointer to get the struct type.
	typ := obj.Type()
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}

	// Unwrap named types to get the underlying struct.
	structType, ok := typ.Underlying().(*types.Struct)
	if !ok {
		return nil
	}

	return buildTagMapFromStruct(structType)
}

// buildTagMapFromStruct builds a Go-field-name → tfsdk-tag-name mapping
// from a types.Struct by parsing `tfsdk:"..."` struct tags.
func buildTagMapFromStruct(s *types.Struct) map[string]string {
	tagMap := make(map[string]string)
	for i := 0; i < s.NumFields(); i++ {
		field := s.Field(i)
		tag := s.Tag(i)
		if tag == "" {
			continue
		}
		tfsdkTag := reflect.StructTag(tag).Get("tfsdk")
		if tfsdkTag == "" || tfsdkTag == "-" {
			continue
		}
		// Strip options after comma (e.g., tfsdk:"name,omitempty").
		if idx := strings.IndexByte(tfsdkTag, ','); idx >= 0 {
			tfsdkTag = tfsdkTag[:idx]
		}
		tagMap[field.Name()] = tfsdkTag
	}
	return tagMap
}

// resolveFieldNames translates Go field names in an overlay body result to
// schema keys using the tfsdk struct tag mapping. Falls back to toSnakeCase
// for fields not found in the tag map.
//
// This fixes false positives where code-generated Go names don't match schema
// keys via simple snake_case conversion (e.g., DimensionsType → "type", not
// "dimensions_type").
func resolveFieldNames(result *overlayBodyResult, tagMap map[string]string) {
	if tagMap == nil {
		return // no type info available; toSnakeCase was already applied
	}

	// Rebuild the assignments map with resolved keys.
	resolved := make(map[string]*fieldAssignment, len(result.assignments))
	for goName, assignment := range result.assignments {
		schemaKey := resolveGoNameToSchemaKey(goName, tagMap)
		resolved[schemaKey] = assignment
	}
	result.assignments = resolved

	// Resolve field names in sub-overlay calls.
	for i := range result.subOverlays {
		result.subOverlays[i].fieldName = resolveGoNameToSchemaKey(
			result.subOverlays[i].fieldName, tagMap,
		)
	}
}

// resolveGoNameToSchemaKey converts a Go field name to its schema key.
// It first checks if the selectorFieldOn already produced a snake_case name
// that exists in the tagMap values (i.e., toSnakeCase happened to produce the
// right answer). If not, it looks for the original Go name in the tagMap keys.
func resolveGoNameToSchemaKey(snakeName string, tagMap map[string]string) string {
	// Check if snakeName is already a valid tfsdk tag value.
	for _, v := range tagMap {
		if v == snakeName {
			return snakeName
		}
	}

	// The snakeName was produced by toSnakeCase from a Go field name.
	// Try to reverse-lookup: find a Go field name whose toSnakeCase matches,
	// then use its tfsdk tag instead.
	for goName, tfsdkName := range tagMap {
		if toSnakeCase(goName) == snakeName {
			return tfsdkName
		}
	}

	// No match found — keep the snake_case name as-is.
	return snakeName
}
