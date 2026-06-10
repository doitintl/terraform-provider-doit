// Package schemaparser extracts Terraform schema field classifications from
// generated schema files (*_gen.go). It produces a SchemaFacts result that
// other analyzers can depend on to know whether each attribute is Required,
// Optional, Computed-only, or Optional+Computed.
//
// It also parses the resource/datasource Schema() method to detect runtime
// overrides (e.g., validators, plan modifiers). While these don't change the
// field classification, they're useful for other analyzers (e.g., checking
// UseStateForUnknown).
package schemaparser

import (
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// FieldClass represents the classification of a Terraform schema attribute.
type FieldClass int

const (
	// Unknown means the field classification could not be determined.
	Unknown FieldClass = iota
	// ComputedOnly means Computed: true, no Optional or Required.
	ComputedOnly
	// Required means Required: true.
	Required
	// Optional means Optional: true, no Computed.
	Optional
	// OptionalComputed means Optional: true, Computed: true.
	OptionalComputed
)

// String returns the human-readable name of the field classification.
func (fc FieldClass) String() string {
	switch fc {
	case ComputedOnly:
		return "ComputedOnly"
	case Required:
		return "Required"
	case Optional:
		return "Optional"
	case OptionalComputed:
		return "Optional+Computed"
	default:
		return "Unknown"
	}
}

// AttrInfo holds classification metadata for a single attribute.
type AttrInfo struct {
	Class FieldClass
	// IsList is true if the attribute is a ListAttribute or ListNestedAttribute.
	IsList bool
	// HasDefault is true if the attribute has a Default value in the schema.
	// Fields with defaults are resolved at plan time and are never Unknown,
	// so they don't need IsUnknown() guards in overlay functions.
	HasDefault bool
	// DefaultValue holds the extracted static default value (string, float64,
	// bool, or int64). It is nil when HasDefault is false or when the default
	// expression could not be parsed (e.g., a dynamic default).
	DefaultValue any
	// NestedAttrs holds classifications for nested attributes (if any).
	NestedAttrs map[string]*AttrInfo
	// PlanModifiers holds the names of plan modifier functions detected in
	// the Schema() method (e.g., "UseStateForUnknown",
	// "useEmptyForUnknownWhenConfigNull"). Populated during Schema() override
	// merging in applySchemaOverrides.
	PlanModifiers []string
	// NotClearable is true when the attribute has been explicitly acknowledged
	// as not user-clearable via acknowledgeNotClearable(). The clearableattr
	// linter skips these attributes.
	NotClearable bool
}

// SchemaInfo holds the parsed schema for a single resource or data source.
type SchemaInfo struct {
	// FuncName is the schema function name (e.g., "BudgetResourceSchema").
	FuncName string
	// Attrs maps attribute name → classification info.
	Attrs map[string]*AttrInfo
}

// SchemaFacts is the result type exported by this analyzer.
// It maps schema function names to their parsed schema info.
// It implements analysis.Fact so it can be shared across packages.
type SchemaFacts struct {
	Schemas map[string]*SchemaInfo
}

// AFact implements the analysis.Fact interface.
func (*SchemaFacts) AFact() {}

// String returns a description of the schemas found, used by analysistest.
func (sf *SchemaFacts) String() string {
	names := make([]string, 0, len(sf.Schemas))
	for name := range sf.Schemas {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// Analyzer is the go/analysis Analyzer for schema parsing.
var Analyzer = &analysis.Analyzer{
	Name:       "schemaparser",
	Doc:        "Extracts Terraform schema field classifications from generated schema files.",
	Run:        run,
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
	ResultType: reflect.TypeFor[*SchemaFacts](),
	FactTypes:  []analysis.Fact{(*SchemaFacts)(nil)},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	facts := &SchemaFacts{
		Schemas: make(map[string]*SchemaInfo),
	}

	// Find all functions that return schema.Schema and whose name ends with
	// "ResourceSchema" or "DataSourceSchema".
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil {
			return
		}
		name := fn.Name.Name
		if !strings.HasSuffix(name, "ResourceSchema") && !strings.HasSuffix(name, "DataSourceSchema") {
			return
		}
		// Only process generated files.
		pos := pass.Fset.Position(fn.Pos())
		if !strings.HasSuffix(pos.Filename, "_gen.go") {
			return
		}

		info := &SchemaInfo{
			FuncName: name,
			Attrs:    make(map[string]*AttrInfo),
		}

		// Find the returned schema.Schema composite literal.
		schemaLit := findReturnedCompositeLit(fn)
		if schemaLit == nil {
			return
		}

		// Find the "Attributes" key in the schema literal.
		attrsMap := findMapField(schemaLit, "Attributes")
		if attrsMap == nil {
			return
		}

		// Parse each attribute.
		parseAttributes(attrsMap, info.Attrs)

		facts.Schemas[name] = info
	})

	// Export as a package fact so downstream packages (in the same analyzer's
	// vertical dependency chain) can inherit this package's schema classifications.
	if len(facts.Schemas) > 0 {
		pass.ExportPackageFact(facts)
	}

	// Also aggregate schemas from imported packages (inherited via vertical edges).
	// This is critical: when schemaparser runs on internal/provider, it inherits
	// facts from schemaparser running on resource_label, resource_budget, etc.
	// We merge them into the Result so that overlaycheck (which reads ResultOf)
	// gets the complete picture.
	for _, pf := range pass.AllPackageFacts() {
		if sf, ok := pf.Fact.(*SchemaFacts); ok {
			for name, info := range sf.Schemas {
				if _, exists := facts.Schemas[name]; !exists {
					facts.Schemas[name] = info
				}
			}
		}
	}

	// Apply Schema() method overrides. Resources may modify the generated schema
	// at runtime (e.g., changing Optional+Computed to Required, adding new fields,
	// or deleting response-only artifacts). We detect these changes via AST and
	// merge them into the schema classification.
	applySchemaOverrides(insp, facts)

	return facts, nil
}

// findReturnedCompositeLit finds the first composite literal in a return statement.
func findReturnedCompositeLit(fn *ast.FuncDecl) *ast.CompositeLit {
	if fn.Body == nil {
		return nil
	}
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok {
			continue
		}
		for _, result := range ret.Results {
			if lit, ok := result.(*ast.CompositeLit); ok {
				return lit
			}
		}
	}
	return nil
}

// findMapField finds a map literal field by name in a composite literal.
func findMapField(lit *ast.CompositeLit, fieldName string) *ast.CompositeLit {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if ident.Name == fieldName {
			if mapLit, ok := kv.Value.(*ast.CompositeLit); ok {
				return mapLit
			}
		}
	}
	return nil
}

// parseAttributes parses a map[string]schema.Attribute composite literal,
// classifying each attribute and handling nested attributes recursively.
func parseAttributes(mapLit *ast.CompositeLit, result map[string]*AttrInfo) {
	for _, elt := range mapLit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		// Key is a string literal like "id", "name".
		attrName := unquote(kv.Key)
		if attrName == "" {
			continue
		}

		attrLit, ok := kv.Value.(*ast.CompositeLit)
		if !ok {
			continue
		}

		info := classifyAttributeLit(attrLit)
		result[attrName] = info
	}
}

// classifyAttributeLit extracts the classification from an attribute composite literal.
func classifyAttributeLit(lit *ast.CompositeLit) *AttrInfo {
	info := &AttrInfo{}

	hasComputed := false
	hasOptional := false
	hasRequired := false

	// Determine the attribute type name (e.g., "StringAttribute", "ListNestedAttribute").
	typeName := selectorTypeName(lit.Type)

	// Check if it's a list type.
	info.IsList = strings.HasPrefix(typeName, "List")

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "Computed":
			hasComputed = isTrueLiteral(kv.Value)
		case "Optional":
			hasOptional = isTrueLiteral(kv.Value)
		case "Required":
			hasRequired = isTrueLiteral(kv.Value)
		case "Default":
			// A non-nil Default means the field is resolved at plan time.
			// Exclude `Default: nil` which is effectively no default.
			if !isNilLiteral(kv.Value) {
				info.HasDefault = true
				info.DefaultValue = extractStaticDefault(kv.Value)
			}
		case "NestedObject":
			// Recurse into nested attributes (ListNestedAttribute, SetNestedAttribute).
			nestedLit, ok := kv.Value.(*ast.CompositeLit)
			if !ok {
				continue
			}
			nestedAttrsMap := findMapField(nestedLit, "Attributes")
			if nestedAttrsMap != nil {
				info.NestedAttrs = make(map[string]*AttrInfo)
				parseAttributes(nestedAttrsMap, info.NestedAttrs)
			}
		case "Attributes":
			// Recurse into nested attributes (SingleNestedAttribute).
			// The Attributes field is a direct map[string]schema.Attribute{}.
			if nestedAttrsMap, ok := kv.Value.(*ast.CompositeLit); ok {
				if info.NestedAttrs == nil {
					info.NestedAttrs = make(map[string]*AttrInfo)
				}
				parseAttributes(nestedAttrsMap, info.NestedAttrs)
			}
		}
	}

	switch {
	case hasRequired:
		info.Class = Required
	case hasComputed && hasOptional:
		info.Class = OptionalComputed
	case hasComputed:
		info.Class = ComputedOnly
	case hasOptional:
		info.Class = Optional
	default:
		info.Class = Unknown
	}

	return info
}

// selectorTypeName extracts the type name from a selector expression or ident.
// For schema.StringAttribute{}, it returns "StringAttribute".
func selectorTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// isTrueLiteral checks if an expression is the boolean literal `true`.
func isTrueLiteral(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "true" && ident.Obj == nil
}

// isNilLiteral checks if an expression is the `nil` literal.
func isNilLiteral(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "nil" && ident.Obj == nil
}

// unquote extracts a string from a basic literal, removing quotes.
func unquote(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind == token.STRING {
			// Remove surrounding quotes.
			s := e.Value
			if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
				return s[1 : len(s)-1]
			}
		}
	}
	return ""
}

// --- Schema() override merging ---

// applySchemaOverrides finds Schema() methods in the current package, identifies
// which generated schema they reference, and applies runtime overrides to produce
// the effective schema classification.
func applySchemaOverrides(insp *inspector.Inspector, facts *SchemaFacts) {
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil || fn.Name.Name != "Schema" {
			return
		}
		// Must be a method (has receiver).
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		// Find the base schema call: s := resource_xxx.XxxResourceSchema(ctx)
		baseSchemaName, schemaVar := findBaseSchemaCall(fn)
		if baseSchemaName == "" || schemaVar == "" {
			return
		}

		// Look up the base schema in the aggregated facts.
		baseSchema, ok := facts.Schemas[baseSchemaName]
		if !ok {
			return
		}

		// Clone the schema so we don't mutate the shared fact.
		merged := cloneSchemaInfo(baseSchema)

		// Walk the method body and apply overrides.
		for _, stmt := range fn.Body.List {
			applyStmtOverride(stmt, schemaVar, merged)
		}

		// Store the merged schema under the same key (replaces the generated one).
		facts.Schemas[baseSchemaName] = merged
	})
}

// findBaseSchemaCall finds the call to the generated schema function in a Schema()
// method body. Returns the schema function name and the local variable name.
//
// Matches: s := resource_xxx.XxxResourceSchema(ctx)
// Returns: ("XxxResourceSchema", "s")
func findBaseSchemaCall(fn *ast.FuncDecl) (schemaName, varName string) {
	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}

		// LHS must be a simple identifier.
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			continue
		}

		// RHS must be a call to pkg.XxxResourceSchema or XxxResourceSchema
		// (package-qualified or local).
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}

		var name string
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		case *ast.Ident:
			name = fn.Name
		default:
			continue
		}

		if strings.HasSuffix(name, "ResourceSchema") || strings.HasSuffix(name, "DataSourceSchema") {
			return name, lhs.Name
		}
	}
	return "", ""
}

// applyStmtOverride applies a single statement's schema override to the merged schema.
// Handles:
//   - delete(s.Attributes, "field")           → removes field
//   - s.Attributes["field"] = schema.Xxx{...} → full replacement
//   - if attr, ok := s.Attributes["field"].(type); ok { ... } → modify-in-place
//   - s.Attributes["field"] = expr            → ignores non-schema (e.g., timeouts)
func applyStmtOverride(stmt ast.Stmt, schemaVar string, schema *SchemaInfo) {
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		call, ok := s.X.(*ast.CallExpr)
		if !ok {
			return
		}

		// Check for acknowledgeNotClearable(s, "path1", "path2", ...)
		if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "acknowledgeNotClearable" {
			if len(call.Args) >= 2 {
				// First arg should be the schema variable.
				if schemaIdent, ok := call.Args[0].(*ast.Ident); ok && schemaIdent.Name == schemaVar {
					for _, arg := range call.Args[1:] {
						if p := unquote(arg); p != "" {
							markNotClearable(schema, p)
						}
					}
				}
			}
			return
		}

		// Check for delete(s.Attributes, "field")
		ident, ok := call.Fun.(*ast.Ident)
		if !ok || ident.Name != "delete" || len(call.Args) != 2 {
			return
		}
		// Verify first arg is s.Attributes.
		if !isSchemaAttributes(call.Args[0], schemaVar) {
			return
		}
		fieldName := unquote(call.Args[1])
		if fieldName != "" {
			delete(schema.Attrs, fieldName)
		}

	case *ast.AssignStmt:
		// Check for s.Attributes["field"] = schema.XxxAttribute{...}
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return
		}
		fieldName := extractAttributeIndexField(s.Lhs[0], schemaVar)
		if fieldName == "" {
			return
		}
		// RHS must be a composite literal (schema.XxxAttribute{...}).
		if lit, ok := s.Rhs[0].(*ast.CompositeLit); ok {
			// Verify it's a schema attribute type.
			typeName := selectorTypeName(lit.Type)
			if strings.HasSuffix(typeName, "Attribute") {
				schema.Attrs[fieldName] = classifyAttributeLit(lit)
			}
		}

	case *ast.IfStmt:
		// Check for: if attr, ok := s.Attributes["field"].(type); ok { ... }
		applyIfBlockOverride(s, schemaVar, schema)
	}
}

// applyIfBlockOverride handles the modify-in-place pattern:
//
//	if attr, ok := s.Attributes["field"].(schema.StringAttribute); ok {
//	    attr.Required = true
//	    attr.Optional = false
//	    attr.Computed = false
//	    s.Attributes["field"] = attr
//	}
func applyIfBlockOverride(ifStmt *ast.IfStmt, schemaVar string, schema *SchemaInfo) {
	if ifStmt.Init == nil || ifStmt.Body == nil {
		return
	}

	// Init must be: attr, ok := s.Attributes["field"].(type)
	assign, ok := ifStmt.Init.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) < 1 || len(assign.Rhs) != 1 {
		return
	}

	// RHS is a type assertion: s.Attributes["field"].(schema.StringAttribute)
	typeAssert, ok := assign.Rhs[0].(*ast.TypeAssertExpr)
	if !ok {
		return
	}

	// The assertion target must be schemaVar.Attributes["field"] or
	// schemaVar.NestedObject.Attributes["field"]. The first form is used
	// by top-level and SingleNestedAttribute, the second by ListNestedAttribute.
	fieldName := extractAttributeIndexField(typeAssert.X, schemaVar)
	if fieldName == "" {
		fieldName = extractNestedAttributeField(typeAssert.X, schemaVar)
	}
	if fieldName == "" {
		return
	}

	// Get the local variable name (e.g., "attr").
	attrVarIdent, ok := assign.Lhs[0].(*ast.Ident)
	if !ok {
		return
	}
	attrVar := attrVarIdent.Name

	// Start from the existing classification (if any).
	existing, exists := schema.Attrs[fieldName]

	// Track which flags are explicitly set.
	hasRequired := false
	hasOptional := false
	hasComputed := false
	reqVal := false
	optVal := false
	compVal := false

	if exists {
		// Pre-populate from existing classification.
		reqVal = existing.Class == Required
		optVal = existing.Class == Optional || existing.Class == OptionalComputed
		compVal = existing.Class == ComputedOnly || existing.Class == OptionalComputed
	}

	// Track plan modifiers detected in this block.
	var modifiers []string

	// Walk the if body looking for attr.Required = true/false, etc.
	for _, bodyStmt := range ifStmt.Body.List {
		bodyAssign, ok := bodyStmt.(*ast.AssignStmt)
		if !ok {
			// Check for nested if blocks (e.g., nested attribute overrides).
			// Recurse by wrapping the parent's nested attrs into a synthetic
			// SchemaInfo. This supports arbitrary nesting depth.
			if nestedIf, ok := bodyStmt.(*ast.IfStmt); ok {
				nestedFieldName := extractNestedIfFieldName(nestedIf, attrVar)
				if nestedFieldName != "" {
					// Build a synthetic SchemaInfo from the current attribute's
					// nested attrs so we can recurse with the same function.
					targetAttrs := getOrCreateNestedAttrs(schema, fieldName)
					nestedSchema := &SchemaInfo{Attrs: targetAttrs}
					applyIfBlockOverride(nestedIf, attrVar, nestedSchema)
				}
			}
			// Check for range loops that inject modifiers on nested attributes.
			if rangeStmt, ok := bodyStmt.(*ast.RangeStmt); ok {
				parseRangeModifiers(rangeStmt, ifStmt.Body, schema, fieldName)
			}
			continue
		}
		if len(bodyAssign.Lhs) != 1 || len(bodyAssign.Rhs) != 1 {
			continue
		}

		// Check for attr.Required, attr.Optional, attr.Computed.
		sel, ok := bodyAssign.Lhs[0].(*ast.SelectorExpr)
		if !ok {
			continue
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != attrVar {
			continue
		}

		switch sel.Sel.Name {
		case "Required":
			val := isTrueLiteral(bodyAssign.Rhs[0])
			hasRequired = true
			reqVal = val
		case "Optional":
			val := isTrueLiteral(bodyAssign.Rhs[0])
			hasOptional = true
			optVal = val
		case "Computed":
			val := isTrueLiteral(bodyAssign.Rhs[0])
			hasComputed = true
			compVal = val
		case "PlanModifiers":
			// attr.PlanModifiers = append(attr.PlanModifiers, xxx())
			modifiers = append(modifiers, extractModifierNames(bodyAssign.Rhs[0])...)
		}
	}

	// Only update if any classification flags or modifiers were found.
	if !hasRequired && !hasOptional && !hasComputed && len(modifiers) == 0 {
		return
	}

	info := &AttrInfo{}
	if exists {
		// Copy existing metadata that isn't affected by classification changes.
		info.IsList = existing.IsList
		info.HasDefault = existing.HasDefault
		info.DefaultValue = existing.DefaultValue
		info.NestedAttrs = existing.NestedAttrs
		info.PlanModifiers = existing.PlanModifiers
	}
	// Merge newly detected modifiers.
	info.PlanModifiers = appendUnique(info.PlanModifiers, modifiers...)

	// Re-classify based on the (potentially modified) flags.
	switch {
	case reqVal:
		info.Class = Required
	case compVal && optVal:
		info.Class = OptionalComputed
	case compVal:
		info.Class = ComputedOnly
	case optVal:
		info.Class = Optional
	default:
		info.Class = Unknown
	}

	schema.Attrs[fieldName] = info
}

// extractNestedIfFieldName extracts the attribute field name from a nested
// if-block's init statement. It checks both parentVar.Attributes["field"] and
// parentVar.NestedObject.Attributes["field"] patterns.
func extractNestedIfFieldName(ifStmt *ast.IfStmt, parentAttrVar string) string {
	if ifStmt.Init == nil {
		return ""
	}
	assign, ok := ifStmt.Init.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 {
		return ""
	}
	typeAssert, ok := assign.Rhs[0].(*ast.TypeAssertExpr)
	if !ok {
		return ""
	}
	name := extractNestedAttributeField(typeAssert.X, parentAttrVar)
	if name == "" {
		// Also try direct .Attributes["field"] (SingleNestedAttribute child).
		name = extractAttributeIndexField(typeAssert.X, parentAttrVar)
	}
	return name
}

// getOrCreateNestedAttrs returns the nested attributes map for a parent
// attribute in the schema, creating it if necessary.
func getOrCreateNestedAttrs(schema *SchemaInfo, parentFieldName string) map[string]*AttrInfo {
	parent, ok := schema.Attrs[parentFieldName]
	if !ok {
		parent = &AttrInfo{}
		schema.Attrs[parentFieldName] = parent
	}
	if parent.NestedAttrs == nil {
		parent.NestedAttrs = make(map[string]*AttrInfo)
	}
	return parent.NestedAttrs
}

// markNotClearable walks a dotted attribute path (e.g., "config.currency",
// "scopes[*].inverse") into the schema's nested AttrInfo tree and sets
// NotClearable = true on the leaf attribute. Path segments separated by "."
// navigate into NestedAttrs; "[*]" suffixes on segments are stripped (they
// denote list element access).
func markNotClearable(schema *SchemaInfo, path string) {
	segments := strings.Split(path, ".")
	attrs := schema.Attrs

	for i, seg := range segments {
		// Strip [*] suffix — it's a path convention, not an attribute name part.
		seg = strings.TrimSuffix(seg, "[*]")

		info, ok := attrs[seg]
		if !ok {
			// Attribute not found in the parsed schema. This can happen for
			// attributes defined only in Schema() overrides or for typos.
			// Create a stub so the clearableattr linter can see it.
			info = &AttrInfo{}
			attrs[seg] = info
		}

		if i == len(segments)-1 {
			// Leaf: mark as not clearable.
			info.NotClearable = true
			return
		}

		// Intermediate: descend into nested attrs.
		if info.NestedAttrs == nil {
			info.NestedAttrs = make(map[string]*AttrInfo)
		}
		attrs = info.NestedAttrs
	}
}

// isSchemaAttributes checks if an expression is schemaVar.Attributes
// (e.g., s.Attributes).
func isSchemaAttributes(expr ast.Expr, schemaVar string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == schemaVar && sel.Sel.Name == "Attributes"
}

// extractAttributeIndexField extracts the field name from an expression like
// s.Attributes["field"]. Returns "" if the pattern doesn't match.
func extractAttributeIndexField(expr ast.Expr, schemaVar string) string {
	indexExpr, ok := expr.(*ast.IndexExpr)
	if !ok {
		return ""
	}
	if !isSchemaAttributes(indexExpr.X, schemaVar) {
		return ""
	}
	return unquote(indexExpr.Index)
}

// extractNestedAttributeField extracts the field name from expressions like
// attr.NestedObject.Attributes["user"]. Returns "" if the pattern doesn't match.
func extractNestedAttributeField(expr ast.Expr, parentAttrVar string) string {
	indexExpr, ok := expr.(*ast.IndexExpr)
	if !ok {
		return ""
	}
	// X should be attr.NestedObject.Attributes or attr.Attributes
	sel1, ok := indexExpr.X.(*ast.SelectorExpr)
	if !ok || sel1.Sel.Name != "Attributes" {
		return ""
	}
	// Case 1: attr.NestedObject.Attributes["field"] (ListNestedAttribute)
	if sel2, ok := sel1.X.(*ast.SelectorExpr); ok && sel2.Sel.Name == "NestedObject" {
		if ident, ok := sel2.X.(*ast.Ident); ok && ident.Name == parentAttrVar {
			return unquote(indexExpr.Index)
		}
	}
	// Case 2: attr.Attributes["field"] (SingleNestedAttribute)
	if ident, ok := sel1.X.(*ast.Ident); ok && ident.Name == parentAttrVar {
		return unquote(indexExpr.Index)
	}
	return ""
}

// cloneSchemaInfo creates a deep copy of a SchemaInfo.
func cloneSchemaInfo(src *SchemaInfo) *SchemaInfo {
	dst := &SchemaInfo{
		FuncName: src.FuncName,
		Attrs:    make(map[string]*AttrInfo, len(src.Attrs)),
	}
	for name, info := range src.Attrs {
		dst.Attrs[name] = cloneAttrInfo(info)
	}
	return dst
}

// cloneAttrInfo creates a deep copy of an AttrInfo.
func cloneAttrInfo(src *AttrInfo) *AttrInfo {
	dst := &AttrInfo{
		Class:        src.Class,
		IsList:       src.IsList,
		HasDefault:   src.HasDefault,
		DefaultValue: src.DefaultValue,
		NotClearable: src.NotClearable,
	}
	if len(src.PlanModifiers) > 0 {
		dst.PlanModifiers = make([]string, len(src.PlanModifiers))
		copy(dst.PlanModifiers, src.PlanModifiers)
	}
	if src.NestedAttrs != nil {
		dst.NestedAttrs = make(map[string]*AttrInfo, len(src.NestedAttrs))
		for name, info := range src.NestedAttrs {
			dst.NestedAttrs[name] = cloneAttrInfo(info)
		}
	}
	return dst
}

// extractStaticDefault attempts to extract the static default value from
// a Default field expression. It recognizes these patterns:
//
//   - stringdefault.StaticString("value") → string
//   - float64default.StaticFloat64(0)     → float64
//   - booldefault.StaticBool(false)       → bool
//   - int64default.StaticInt64(1000)      → int64
//
// Returns nil if the expression does not match a known static default pattern.
func extractStaticDefault(expr ast.Expr) any {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil
	}

	arg := call.Args[0]

	switch {
	case pkg.Name == "stringdefault" && sel.Sel.Name == "StaticString":
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			s, err := strconv.Unquote(lit.Value)
			if err == nil {
				return s
			}
		}
	case pkg.Name == "float64default" && sel.Sel.Name == "StaticFloat64":
		if lit, ok := arg.(*ast.BasicLit); ok && (lit.Kind == token.INT || lit.Kind == token.FLOAT) {
			f, err := strconv.ParseFloat(lit.Value, 64)
			if err == nil {
				return f
			}
		}
		// Handle negative: -X (unary minus)
		if unary, ok := arg.(*ast.UnaryExpr); ok && unary.Op == token.SUB {
			if lit, ok := unary.X.(*ast.BasicLit); ok && (lit.Kind == token.INT || lit.Kind == token.FLOAT) {
				f, err := strconv.ParseFloat(lit.Value, 64)
				if err == nil {
					return -f
				}
			}
		}
	case pkg.Name == "booldefault" && sel.Sel.Name == "StaticBool":
		if ident, ok := arg.(*ast.Ident); ok {
			switch ident.Name {
			case "true":
				return true
			case "false":
				return false
			}
		}
	case pkg.Name == "int64default" && sel.Sel.Name == "StaticInt64":
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.INT {
			i, err := strconv.ParseInt(lit.Value, 10, 64)
			if err == nil {
				return i
			}
		}
		// Handle negative: -X (unary minus)
		if unary, ok := arg.(*ast.UnaryExpr); ok && unary.Op == token.SUB {
			if lit, ok := unary.X.(*ast.BasicLit); ok && lit.Kind == token.INT {
				i, err := strconv.ParseInt(lit.Value, 10, 64)
				if err == nil {
					return -i
				}
			}
		}
	}
	return nil
}

// --- Plan modifier extraction ---

// extractModifierNames walks an expression tree (typically an append() call)
// and returns the names of all plan modifier function calls found.
//
// Matches:
//   - stringplanmodifier.UseStateForUnknown()  → "UseStateForUnknown"
//   - useEmptyForUnknownWhenConfigNull()         → "useEmptyForUnknownWhenConfigNull"
func extractModifierNames(expr ast.Expr) []string {
	var names []string
	ast.Inspect(expr, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			// pkg.FuncName() — e.g., stringplanmodifier.UseStateForUnknown()
			names = append(names, fn.Sel.Name)
		case *ast.Ident:
			// localFunc() — e.g., useEmptyForUnknownWhenConfigNull()
			// Exclude "append" itself.
			if fn.Name != "append" {
				names = append(names, fn.Name)
			}
		}
		return true
	})
	return names
}

// parseRangeModifiers handles loop-based plan modifier injection:
//
//	clearableFields := []string{"metadata", "external_id", "external_url"}
//	for _, field := range clearableFields {
//	    if nested, ok := rrAttr.NestedObject.Attributes[field].(schema.StringAttribute); ok {
//	        nested.PlanModifiers = append(nested.PlanModifiers, useEmptyForUnknownWhenConfigNull())
//	        rrAttr.NestedObject.Attributes[field] = nested
//	    }
//	}
//
// It extracts the static field names from the range source and maps each to the
// detected modifiers, updating the nested AttrInfo entries in the parent schema.
func parseRangeModifiers(rangeStmt *ast.RangeStmt, enclosingBody *ast.BlockStmt, schema *SchemaInfo, parentFieldName string) {
	if rangeStmt.Body == nil {
		return
	}

	// Find the range source — could be a variable name or an inline literal.
	rangeSourceVar := ""
	if ident, ok := rangeStmt.X.(*ast.Ident); ok {
		rangeSourceVar = ident.Name
	}
	// If not a named variable and not an inline literal, skip.
	if rangeSourceVar == "" {
		if _, ok := rangeStmt.X.(*ast.CompositeLit); !ok {
			return
		}
	}

	// Detect plan modifier calls inside the loop body.
	var modifiers []string
	ast.Inspect(rangeStmt.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		sel, ok := assign.Lhs[0].(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "PlanModifiers" {
			return true
		}
		modifiers = append(modifiers, extractModifierNames(assign.Rhs[0])...)
		return true
	})

	if len(modifiers) == 0 {
		return
	}

	// Find the static string slice that feeds the loop.
	// Search both the range statement itself (inline literal) and the
	// enclosing block for a variable definition matching the source.
	fieldNames := findSliceLiteralFields(rangeStmt, enclosingBody, rangeSourceVar)
	if len(fieldNames) == 0 {
		return
	}

	// Apply modifiers to each nested attribute.
	parentAttr, ok := schema.Attrs[parentFieldName]
	if !ok || parentAttr.NestedAttrs == nil {
		return
	}
	for _, fn := range fieldNames {
		if nested, ok := parentAttr.NestedAttrs[fn]; ok {
			nested.PlanModifiers = appendUnique(nested.PlanModifiers, modifiers...)
		}
	}
}

// findSliceLiteralFields finds the string literals from the range source.
// It handles two cases:
//  1. Inline literal:  for _, f := range []string{"a", "b"} { ... }
//  2. Named variable:  fields := []string{"a", "b"}; for _, f := range fields { ... }
func findSliceLiteralFields(rangeStmt *ast.RangeStmt, enclosingBody *ast.BlockStmt, sourceVar string) []string {
	// Case 1: inline composite literal.
	if comp, ok := rangeStmt.X.(*ast.CompositeLit); ok {
		return extractStringLiteralElts(comp)
	}
	// Case 2: named variable — search enclosing block for definition.
	for _, stmt := range enclosingBody.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || ident.Name != sourceVar {
			continue
		}
		comp, ok := assign.Rhs[0].(*ast.CompositeLit)
		if !ok {
			continue
		}
		return extractStringLiteralElts(comp)
	}
	return nil
}

// extractStringLiteralElts extracts string values from a []string{...} composite literal.
func extractStringLiteralElts(comp *ast.CompositeLit) []string {
	var result []string
	for _, elt := range comp.Elts {
		if lit, ok := elt.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			result = append(result, unquote(elt))
		}
	}
	return result
}

// appendUnique appends values to a slice, skipping duplicates.
func appendUnique(dst []string, values ...string) []string {
	seen := make(map[string]bool, len(dst))
	for _, v := range dst {
		seen[v] = true
	}
	for _, v := range values {
		if !seen[v] {
			dst = append(dst, v)
			seen[v] = true
		}
	}
	return dst
}
