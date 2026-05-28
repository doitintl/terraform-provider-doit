// Package requestguard validates IsUnknown() usage in request builder functions.
//
// Direction 1 (redundant guard): flags IsUnknown() checks on fields that can
// never be Unknown (Required, Optional without Computed, Optional+Computed with
// Default). These guards are dead code.
//
// Direction 2 (missing guard): flags non-pointer value accessors (ValueString,
// ValueBool, ValueFloat64, ValueInt64) on Optional+Computed fields without
// Default when not guarded by IsUnknown(). Pointer accessors (ValueStringPointer
// etc.) are excluded because they return nil for Unknown, which is often acceptable.
package requestguard

import (
	"go/ast"
	"go/token"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for requestguard.
var Analyzer = &analysis.Analyzer{
	Name:     "requestguard",
	Doc:      "Validates IsUnknown() usage in request builder functions.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

// neverUnknownClasses are schema classifications where IsUnknown() is dead code.
var neverUnknownClasses = map[schemaparser.FieldClass]string{
	schemaparser.Required: "Required",
	schemaparser.Optional: "Optional (not Computed)",
}

// nonPointerAccessors are value accessors that return zero values for Unknown
// (not nil), making them unsafe without an IsUnknown() guard.
var nonPointerAccessors = map[string]bool{
	"ValueString":  true,
	"ValueBool":    true,
	"ValueFloat64": true,
	"ValueInt64":   true,
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

	// Index schemas by function name. Each schema's Attrs map contains
	// only top-level attributes; nested attrs live in AttrInfo.NestedAttrs.
	// This avoids name collisions between top-level and nested attrs.
	type attrMap = map[string]*schemaparser.AttrInfo
	perSchema := map[string]attrMap{}
	for name, info := range schemaResult.Schemas {
		perSchema[name] = info.Attrs
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

	// Step 2: Find request builder functions and check guards.
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil {
			return
		}
		if !isRequestBuilder(fn.Name.Name) {
			return
		}

		// Resolve schema: try receiver, then parameter types.
		schema := resolveSchema(fn, receiverToSchema, perSchema)
		if schema == nil {
			return
		}

		// Build variable → schema path mapping for nested field tracking.
		// Initialize with the plan parameter name.
		planParam := findPlanParam(fn)
		if planParam == "" {
			return
		}

		// varSchemas maps variable names to their nested schema context.
		// The plan parameter maps to the top-level schema.
		varSchemas := map[string]attrMap{planParam: schema}

		// Track variable assignments to nested types.
		trackVariableAssignments(fn.Body, varSchemas)

		// Direction 1: Find redundant IsUnknown() guards.
		checkRedundantGuards(pass, fn.Body, varSchemas)

		// Direction 2: Find missing guards on non-pointer value accessors.
		checkMissingGuards(pass, fn.Body, varSchemas)
	})

	return nil, nil
}



// isRequestBuilder returns true if the function name matches a request builder
// pattern (toCreateRequest, toUpdateRequest, fillXxxCommon, toAlertConfig, etc.).
func isRequestBuilder(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "request") ||
		strings.Contains(lower, "common") ||
		strings.Contains(lower, "config") && strings.HasPrefix(lower, "to")
}

// resolveSchema resolves the schema for a function by trying receiver type,
// then parameter types.
func resolveSchema(fn *ast.FuncDecl, receiverToSchema map[string]string, perSchema map[string]map[string]*schemaparser.AttrInfo) map[string]*schemaparser.AttrInfo {
	// Try receiver first.
	recvType := extractReceiverTypeName(fn)
	if recvType != "" {
		if schemaName, ok := receiverToSchema[recvType]; ok {
			if s, ok := perSchema[schemaName]; ok {
				return s
			}
		}
		// Try deriving schema from model type name.
		schemaName := modelTypeToSchema(recvType)
		if s, ok := perSchema[schemaName]; ok {
			return s
		}
	}

	// Try parameter types.
	if fn.Type != nil && fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			typeName := extractStarTypeName(field.Type)
			if typeName == "" {
				continue
			}
			schemaName := modelTypeToSchema(typeName)
			if s, ok := perSchema[schemaName]; ok {
				return s
			}
		}
	}

	return nil
}

// modelTypeToSchema derives the schema function name from a model type name.
// E.g., "budgetResourceModel" → "BudgetResourceSchema".
func modelTypeToSchema(typeName string) string {
	base := ""
	if strings.HasSuffix(typeName, "ResourceModel") {
		base = strings.TrimSuffix(typeName, "Model")
	} else if strings.HasSuffix(typeName, "Model") {
		base = strings.TrimSuffix(typeName, "Model") + "Resource"
	}
	if base == "" {
		return ""
	}
	// Capitalize first letter.
	return strings.ToUpper(base[:1]) + base[1:] + "Schema"
}

// findPlanParam returns the name of the plan parameter in a request builder.
// It looks for a parameter named "plan" or a receiver (method on model).
func findPlanParam(fn *ast.FuncDecl) string {
	// If method, the receiver acts as the plan.
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		for _, name := range fn.Recv.List[0].Names {
			return name.Name
		}
	}
	// Look for a parameter named "plan" or a *XxxModel parameter.
	if fn.Type != nil && fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				if name.Name == "plan" {
					return name.Name
				}
			}
			// Check if it's a model type parameter.
			typeName := extractStarTypeName(field.Type)
			if typeName != "" && (strings.HasSuffix(typeName, "Model") || strings.HasSuffix(typeName, "Value")) {
				for _, name := range field.Names {
					return name.Name
				}
			}
		}
	}
	return ""
}

// trackVariableAssignments scans the function body for assignments that
// create aliases to nested schema contexts (e.g., "config := plan.Config").
// It derives the nested schema from the parent AttrInfo.NestedAttrs rather
// than flattening the schema tree, preventing name collisions.
func trackVariableAssignments(body *ast.BlockStmt, varSchemas map[string]map[string]*schemaparser.AttrInfo) {
	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range assign.Rhs {
			if i >= len(assign.Lhs) {
				continue
			}
			lhsIdent, ok := assign.Lhs[i].(*ast.Ident)
			if !ok {
				continue
			}
			// Check if RHS is varName.Field.
			sel, ok := rhs.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			varName, fieldName := resolveSelectorChain(sel)
			if varName == "" {
				continue
			}
			// Check if the parent variable has a known schema.
			parentSchema, ok := varSchemas[varName]
			if !ok {
				continue
			}
			// Look up the field in the parent's schema and use its
			// NestedAttrs as the assigned variable's schema context.
			attrName := toSnakeCase(fieldName)
			parentAttr, ok := parentSchema[attrName]
			if !ok || parentAttr.NestedAttrs == nil {
				continue
			}
			varSchemas[lhsIdent.Name] = parentAttr.NestedAttrs
		}
		return true
	})
}

// resolveSelectorChain resolves a.B to ("a", "B"), a.B.C to ("a", "C") etc.
// Returns the root variable name and the immediate field name.
func resolveSelectorChain(sel *ast.SelectorExpr) (rootVar, fieldName string) {
	fieldName = sel.Sel.Name
	switch x := sel.X.(type) {
	case *ast.Ident:
		return x.Name, fieldName
	case *ast.SelectorExpr:
		root, _ := resolveSelectorChain(x)
		return root, fieldName
	}
	return "", ""
}

// resolveFieldAttr resolves a selector expression to its schema AttrInfo
// by navigating the schema tree via NestedAttrs. Supports arbitrary nesting
// depth (e.g., plan.Config.SubConfig.Field).
func resolveFieldAttr(sel *ast.SelectorExpr, varSchemas map[string]map[string]*schemaparser.AttrInfo) *schemaparser.AttrInfo {
	// Build the full selector chain: plan.A.B.C → ["plan", "A", "B", "C"]
	chain := buildSelectorChain(sel)
	if len(chain) < 2 {
		return nil
	}

	// chain[0] is the root variable, chain[1:len-1] are intermediate fields,
	// chain[len-1] is the leaf field we want the AttrInfo for.
	rootVar := chain[0]
	schema, ok := varSchemas[rootVar]
	if !ok {
		return nil
	}

	// Navigate through intermediate fields via NestedAttrs.
	for i := 1; i < len(chain)-1; i++ {
		attrName := toSnakeCase(chain[i])
		attr, ok := schema[attrName]
		if !ok || attr.NestedAttrs == nil {
			return nil
		}
		schema = attr.NestedAttrs
	}

	// Look up the leaf field.
	leafAttr := toSnakeCase(chain[len(chain)-1])
	if attr, ok := schema[leafAttr]; ok {
		return attr
	}
	return nil
}

// buildSelectorChain converts a selector expression to an ordered list of names.
// E.g., plan.Config.Metric → ["plan", "Config", "Metric"].
func buildSelectorChain(expr ast.Expr) []string {
	switch e := expr.(type) {
	case *ast.Ident:
		return []string{e.Name}
	case *ast.SelectorExpr:
		parent := buildSelectorChain(e.X)
		if parent == nil {
			return nil
		}
		return append(parent, e.Sel.Name)
	}
	return nil
}



// extractGuardedFromCond extracts field selector strings from conditions
// containing negated IsUnknown() calls (!plan.X.IsUnknown()). Only negated
// calls guard the if-body — a positive check (plan.X.IsUnknown()) means the
// body runs WHEN Unknown, so it is not a guard.
func extractGuardedFromCond(expr ast.Expr, guarded map[string]bool, negated bool) {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		extractGuardedFromCond(e.X, guarded, negated)
		extractGuardedFromCond(e.Y, guarded, negated)
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			extractGuardedFromCond(e.X, guarded, !negated)
		} else {
			extractGuardedFromCond(e.X, guarded, negated)
		}
	case *ast.ParenExpr:
		extractGuardedFromCond(e.X, guarded, negated)
	case *ast.CallExpr:
		// Check for plan.X.IsUnknown()
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "IsUnknown" {
			return
		}
		// Only treat as guard when negated: !plan.X.IsUnknown()
		if negated {
			key := selectorKey(sel.X)
			if key != "" {
				guarded[key] = true
			}
		}
	}
}

// selectorKey returns a string key for a selector expression, e.g., "plan.Metric".
func selectorKey(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		parent := selectorKey(e.X)
		if parent != "" {
			return parent + "." + e.Sel.Name
		}
	}
	return ""
}

// checkRedundantGuards walks the function body and flags IsUnknown() calls
// on fields that can never be Unknown.
func checkRedundantGuards(pass *analysis.Pass, body *ast.BlockStmt, varSchemas map[string]map[string]*schemaparser.AttrInfo) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "IsUnknown" {
			return true
		}

		// Resolve the field being checked.
		fieldSel, ok := sel.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		attr := resolveFieldAttr(fieldSel, varSchemas)
		if attr == nil {
			return true
		}

		// Check if the field can never be Unknown.
		if reason, ok := neverUnknownClasses[attr.Class]; ok {
			attrName := toSnakeCase(fieldSel.Sel.Name)
			pass.Reportf(call.Pos(),
				"IsUnknown() on %s field %q is dead code (%s fields are always Known)",
				reason, attrName, reason)
		} else if attr.Class == schemaparser.OptionalComputed && attr.HasDefault {
			attrName := toSnakeCase(fieldSel.Sel.Name)
			pass.Reportf(call.Pos(),
				"IsUnknown() on field %q with schema Default is dead code "+
					"(defaults are resolved at plan time)",
				attrName)
		}

		return true
	})
}

// checkMissingGuards walks the function body and flags non-pointer value
// accessors on Optional+Computed fields without Default when not guarded
// by an enclosing if-condition with IsUnknown().
func checkMissingGuards(pass *analysis.Pass, body *ast.BlockStmt, varSchemas map[string]map[string]*schemaparser.AttrInfo) {
	// Build a map from position ranges of if-bodies to the fields they guard.
	// This allows us to check if a value accessor is inside a guarded scope.
	type guardScope struct {
		startPos, endPos int
		guardedKeys      map[string]bool
	}
	var scopes []guardScope

	ast.Inspect(body, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		guarded := map[string]bool{}
		extractGuardedFromCond(ifStmt.Cond, guarded, false)
		if len(guarded) > 0 && ifStmt.Body != nil {
			scopes = append(scopes, guardScope{
				startPos:    int(ifStmt.Body.Pos()),
				endPos:      int(ifStmt.Body.End()),
				guardedKeys: guarded,
			})
		}
		return true
	})

	// Now check all value accessor calls.
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// Check if it's a non-pointer accessor.
		if !nonPointerAccessors[sel.Sel.Name] {
			return true
		}

		// The receiver should be plan.X (a field selector).
		fieldSel, ok := sel.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		attr := resolveFieldAttr(fieldSel, varSchemas)
		if attr == nil {
			return true
		}

		// Only flag Optional+Computed without Default.
		if attr.Class != schemaparser.OptionalComputed || attr.HasDefault {
			return true
		}

		// Check if the call is inside a guarded scope for this field.
		key := selectorKey(fieldSel)
		callPos := int(call.Pos())
		for _, scope := range scopes {
			if callPos >= scope.startPos && callPos <= scope.endPos && scope.guardedKeys[key] {
				return true // inside guarded scope
			}
		}

		attrName := toSnakeCase(fieldSel.Sel.Name)
		pass.Reportf(call.Pos(),
			"%s() on Optional+Computed field %q without IsUnknown() guard; "+
				"field may be Unknown at plan time",
			sel.Sel.Name, attrName)

		return true
	})
}

// toSnakeCase converts a Go field name to snake_case by inserting underscores
// before uppercase letters.
func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + 32) // lowercase
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// extractReceiverTypeName extracts the type name from a method's receiver.
func extractReceiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	return extractStarTypeName(fn.Recv.List[0].Type)
}

// extractStarTypeName extracts the type name from *T.
func extractStarTypeName(expr ast.Expr) string {
	star, ok := expr.(*ast.StarExpr)
	if !ok {
		if ident, ok := expr.(*ast.Ident); ok {
			return ident.Name
		}
		return ""
	}
	if ident, ok := star.X.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// findSchemaCall searches the Schema() method body for a call to a
// function whose name ends with "ResourceSchema" or "DataSourceSchema".
// Supports both unqualified calls (AlertResourceSchema) and package-qualified
// calls (resource_alert.AlertResourceSchema).
func findSchemaCall(fn *ast.FuncDecl) string {
	var schemaName string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		var name string
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			name = fun.Name
		case *ast.SelectorExpr:
			name = fun.Sel.Name
		}
		if name != "" && (strings.HasSuffix(name, "ResourceSchema") || strings.HasSuffix(name, "DataSourceSchema")) {
			schemaName = name
			return false
		}
		return true
	})
	return schemaName
}
