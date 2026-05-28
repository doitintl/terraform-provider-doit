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

	// Build a flat map of all schema attrs (including nested) indexed by
	// schema function name. Each entry maps field name → AttrInfo.
	// We store both prefixed ("config.metric") and unprefixed ("metric")
	// for nested attrs so nested helper functions can resolve fields.
	type attrMap = map[string]*schemaparser.AttrInfo
	perSchema := map[string]attrMap{}
	for name, info := range schemaResult.Schemas {
		m := attrMap{}
		collectAttrs(info.Attrs, m, "")
		perSchema[name] = m
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
		trackVariableAssignments(fn.Body, varSchemas, schema)

		// Direction 1: Find redundant IsUnknown() guards.
		checkRedundantGuards(pass, fn.Body, varSchemas, schema)

		// Direction 2: Find missing guards on non-pointer value accessors.
		checkMissingGuards(pass, fn.Body, varSchemas)
	})

	return nil, nil
}

// collectAttrs recursively builds a flat map of attribute name → AttrInfo.
// Nested attrs are stored with dot-prefixed keys AND without prefix.
func collectAttrs(attrs map[string]*schemaparser.AttrInfo, out map[string]*schemaparser.AttrInfo, prefix string) {
	for name, info := range attrs {
		out[prefix+name] = info
		if info.NestedAttrs != nil {
			// Store nested attrs both with prefix (for top-level resolution)
			// and without (for when a variable holds the nested type directly).
			collectAttrs(info.NestedAttrs, out, prefix+name+".")
			collectAttrs(info.NestedAttrs, out, "")
		}
	}
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
func trackVariableAssignments(body *ast.BlockStmt, varSchemas map[string]map[string]*schemaparser.AttrInfo, topSchema map[string]*schemaparser.AttrInfo) {
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
			if _, ok := varSchemas[varName]; !ok {
				continue
			}
			// The field corresponds to a nested attribute — register
			// the assigned variable with the nested schema.
			attrName := toSnakeCase(fieldName)
			nestedKey := attrName + "."
			nested := map[string]*schemaparser.AttrInfo{}
			for k, v := range topSchema {
				if strings.HasPrefix(k, nestedKey) {
					nested[strings.TrimPrefix(k, nestedKey)] = v
				}
			}
			// Also include unprefixed nested attrs.
			for k, v := range topSchema {
				if !strings.Contains(k, ".") {
					nested[k] = v
				}
			}
			if len(nested) > 0 {
				varSchemas[lhsIdent.Name] = nested
			}
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
// by looking up the field name in the variable's schema context.
func resolveFieldAttr(sel *ast.SelectorExpr, varSchemas map[string]map[string]*schemaparser.AttrInfo) *schemaparser.AttrInfo {
	fieldName := sel.Sel.Name
	attrName := toSnakeCase(fieldName)

	// Get the variable that owns this field.
	switch x := sel.X.(type) {
	case *ast.Ident:
		if schema, ok := varSchemas[x.Name]; ok {
			if attr, ok := schema[attrName]; ok {
				return attr
			}
		}
	case *ast.SelectorExpr:
		// Nested: e.g., plan.Config.Metric → look up Metric in Config's schema.
		// First resolve the intermediate selector.
		parentField := x.Sel.Name
		parentAttr := toSnakeCase(parentField)

		rootVar := ""
		switch rx := x.X.(type) {
		case *ast.Ident:
			rootVar = rx.Name
		case *ast.SelectorExpr:
			rootVar, _ = resolveSelectorChain(rx)
		}

		if rootVar == "" {
			return nil
		}

		if schema, ok := varSchemas[rootVar]; ok {
			// Look for parentAttr.attrName in the schema.
			nestedKey := parentAttr + "." + attrName
			if attr, ok := schema[nestedKey]; ok {
				return attr
			}
		}
	}
	return nil
}



// extractGuardedFromCond extracts field selector strings from conditions
// containing IsUnknown() calls.
func extractGuardedFromCond(expr ast.Expr, guarded map[string]bool) {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		extractGuardedFromCond(e.X, guarded)
		extractGuardedFromCond(e.Y, guarded)
	case *ast.UnaryExpr:
		extractGuardedFromCond(e.X, guarded)
	case *ast.ParenExpr:
		extractGuardedFromCond(e.X, guarded)
	case *ast.CallExpr:
		// Check for plan.X.IsUnknown()
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "IsUnknown" {
			return
		}
		// The receiver of IsUnknown is plan.X.
		key := selectorKey(sel.X)
		if key != "" {
			guarded[key] = true
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
func checkRedundantGuards(pass *analysis.Pass, body *ast.BlockStmt, varSchemas map[string]map[string]*schemaparser.AttrInfo, schema map[string]*schemaparser.AttrInfo) {
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
		extractGuardedFromCond(ifStmt.Cond, guarded)
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
func findSchemaCall(fn *ast.FuncDecl) string {
	var schemaName string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok {
			name := ident.Name
			if strings.HasSuffix(name, "ResourceSchema") || strings.HasSuffix(name, "DataSourceSchema") {
				schemaName = name
				return false
			}
		}
		return true
	})
	return schemaName
}
