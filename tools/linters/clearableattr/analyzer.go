// Package clearableattr ensures that every Optional+Computed attribute without
// a Default in every resource's Schema() method is explicitly classified as
// either clearable (has useNullForUnknownWhenConfigNull plan modifier) or
// intentionally not clearable (suppressed via //nolint:clearableattr on the
// attribute's override block).
//
// Without classification, Optional+Computed attributes silently preserve their
// prior state value when a user removes them from config, making it impossible
// to clear the attribute. This linter enforces conscious decision-making.
//
// See: https://github.com/doitintl/terraform-provider-doit/issues/233
package clearableattr

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for clearableattr.
var Analyzer = &analysis.Analyzer{
	Name:     "clearableattr",
	Doc:      "Ensures Optional+Computed attributes without Default are explicitly classified as clearable or not.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

// clearableModifierPrefix and clearableModifierSuffix define the naming
// convention for plan modifier functions that mark an attribute as clearable.
// Typed variants exist for each Terraform type:
//   - useNullForUnknownWhenConfigNull        (string)
//   - useNullForUnknownBoolWhenConfigNull     (bool)
//   - useNullForUnknownInt64WhenConfigNull    (int64)
//   - useNullForUnknownFloat64WhenConfigNull  (float64)
//   - useNullForUnknownListWhenConfigNull     (list)
const (
	clearableModifierPrefix = "useNullForUnknown"
	clearableModifierSuffix = "WhenConfigNull"
)

func run(pass *analysis.Pass) (any, error) {
	result := pass.ResultOf[schemaparser.Analyzer]
	if result == nil {
		return nil, nil
	}
	schemaResult, ok := result.(*schemaparser.SchemaFacts)
	if !ok || schemaResult == nil {
		return nil, nil
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Find each Schema() method and check for unclassified O+C attributes.
	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Name.Name != "Schema" || fn.Body == nil {
			return
		}
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		// Find which generated schema this method references.
		schemaName := findReferencedSchemaName(fn)
		if schemaName == "" {
			return
		}

		// Only check resource schemas (not data sources).
		if strings.HasSuffix(schemaName, "DataSourceSchema") {
			return
		}

		schemaInfo, ok := schemaResult.Schemas[schemaName]
		if !ok {
			return
		}

		// Build a map of attribute name → if-block position in the Schema()
		// body. Each attribute's if-block has a unique line, so diagnostics
		// reported at these positions are not deduplicated by golangci-lint.
		attrPositions := buildAttrPositionMap(fn)

		// Collect all unclassified attributes first.
		var findings []string
		collectUnclassified(schemaInfo.Attrs, "", &findings)

		// Build a list of unique fallback positions from the function body
		// statements. golangci-lint deduplicates issues on the same file+line,
		// so each finding needs a distinct position.
		//
		// For attributes without override blocks we use unique body statement
		// positions. When there are more findings than statements, we create
		// synthetic positions by incrementing the body's opening brace offset
		// (each offset maps to a different column on the same line, which
		// golangci-lint treats as distinct).
		fallbackPositions := make([]token.Pos, 0, len(fn.Body.List))
		for _, stmt := range fn.Body.List {
			fallbackPositions = append(fallbackPositions, stmt.Pos())
		}

		// Sort for deterministic position assignment and output order.
		slices.Sort(findings)

		// Report each finding at a unique position.
		fallbackIdx := 0
		for _, fullPath := range findings {
			var pos token.Pos
			if p, ok := attrPositions[fullPath]; ok {
				pos = p
			} else if fallbackIdx < len(fallbackPositions) {
				pos = fallbackPositions[fallbackIdx]
				fallbackIdx++
			} else {
				// More findings than body statements: create synthetic
				// positions by offsetting from the opening brace. Each
				// offset produces a unique column, surviving dedup.
				pos = fn.Body.Lbrace + token.Pos(fallbackIdx)
				fallbackIdx++
			}
			pass.Reportf(pos,
				"Optional+Computed attribute %q has no clearable classification.\n"+
					"\tAdd useNullForUnknownWhenConfigNull() if the attribute should be clearable,\n"+
					"\tor acknowledgeNotClearable() if the prior value should be preserved.\n"+
					"\tSee: https://github.com/doitintl/terraform-provider-doit/issues/233",
				fullPath)
		}
	})

	return nil, nil
}

// collectUnclassified recursively collects the full paths of unclassified
// Optional+Computed attributes (no Default, no clearableModifier, not a container).
func collectUnclassified(attrs map[string]*schemaparser.AttrInfo, prefix string, out *[]string) {
	for name, info := range attrs {
		fullPath := name
		if prefix != "" {
			fullPath = prefix + "." + name
		}

		if info.Class == schemaparser.OptionalComputed && !info.HasDefault {
			// Skip container attributes — the clearable concern applies to
			// their leaf children, not the container itself.
			if info.NestedAttrs == nil {
				if !hasClearableModifier(info.PlanModifiers) && !info.NotClearable {
					*out = append(*out, fullPath)
				}
			}
		}

		// Recurse into nested attributes.
		if info.NestedAttrs != nil {
			nestedPrefix := fullPath
			if info.IsList {
				nestedPrefix = fullPath + "[*]"
			}
			collectUnclassified(info.NestedAttrs, nestedPrefix, out)
		}
	}
}

// hasClearableModifier returns true if any modifier in the list matches the
// useNullForUnknown*WhenConfigNull naming convention.
func hasClearableModifier(modifiers []string) bool {
	for _, m := range modifiers {
		if strings.HasPrefix(m, clearableModifierPrefix) && strings.HasSuffix(m, clearableModifierSuffix) {
			return true
		}
	}
	return false
}

// buildAttrPositionMap scans a Schema() method body for if-blocks that access
// s.Attributes["fieldname"] and returns a map of field path → position.
//
// It supports arbitrary nesting depth. For example:
//
//	if configAttr, ok := s.Attributes["config"].(schema.SingleNestedAttribute); ok {
//	    if attr, ok := configAttr.Attributes["currency"].(schema.StringAttribute); ok { ... }
//	    if scopesAttr, ok := configAttr.Attributes["scopes"].(schema.ListNestedAttribute); ok {
//	        if attr, ok := scopesAttr.NestedObject.Attributes["inverse"].(schema.BoolAttribute); ok { ... }
//	    }
//	}
//
// produces: {"config": pos1, "config.currency": pos2, "config.scopes[*].inverse": pos3}
func buildAttrPositionMap(fn *ast.FuncDecl) map[string]token.Pos {
	positions := make(map[string]token.Pos)
	for _, stmt := range fn.Body.List {
		ifStmt, ok := stmt.(*ast.IfStmt)
		if !ok {
			continue
		}
		scanIfBlock(ifStmt, "", positions)
	}
	return positions
}

// scanIfBlock recursively scans an if-block and its nested if-blocks for
// attribute access patterns and populates the positions map.
func scanIfBlock(ifStmt *ast.IfStmt, prefix string, positions map[string]token.Pos) {
	name := extractIfBlockAttrName(ifStmt)
	if name == "" {
		name = extractNestedAttrName(ifStmt)
	}
	if name == "" {
		return
	}

	fullPath := name
	if prefix != "" {
		fullPath = prefix + "." + name
	}
	positions[fullPath] = ifStmt.Pos()

	// Determine if this attribute is a list (has NestedObject access) by
	// checking the type assertion target. If it's a ListNestedAttribute,
	// nested children use the [*] path segment.
	childPrefix := fullPath
	if isListNestedAttr(ifStmt) {
		childPrefix = fullPath + "[*]"
	}

	// Recurse into the if-block's body for deeper nesting.
	if ifStmt.Body != nil {
		for _, innerStmt := range ifStmt.Body.List {
			innerIf, ok := innerStmt.(*ast.IfStmt)
			if !ok {
				// Handle for-range loops with if-blocks inside (e.g., for _, field := range []string{...} { if ... })
				if rangeStmt, ok := innerStmt.(*ast.RangeStmt); ok && rangeStmt.Body != nil {
					for _, rangeInner := range rangeStmt.Body.List {
						if rangeIf, ok := rangeInner.(*ast.IfStmt); ok {
							scanIfBlock(rangeIf, childPrefix, positions)
						}
					}
				}
				continue
			}
			scanIfBlock(innerIf, childPrefix, positions)
		}
	}
}

// isListNestedAttr checks if the if-block's type assertion targets a ListNestedAttribute.
func isListNestedAttr(ifStmt *ast.IfStmt) bool {
	if ifStmt.Init == nil {
		return false
	}
	assign, ok := ifStmt.Init.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 {
		return false
	}
	typeAssert, ok := assign.Rhs[0].(*ast.TypeAssertExpr)
	if !ok || typeAssert.Type == nil {
		return false
	}
	sel, ok := typeAssert.Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "ListNestedAttribute"
}

// extractIfBlockAttrName extracts the attribute name from an if-block pattern like:
//
//	if attr, ok := s.Attributes["description"].(schema.StringAttribute); ok {
//
// Returns the attribute name ("description") or empty string if the pattern doesn't match.
func extractIfBlockAttrName(ifStmt *ast.IfStmt) string {
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
	indexExpr, ok := typeAssert.X.(*ast.IndexExpr)
	if !ok {
		return ""
	}
	// Check that it's accessing .Attributes["name"]
	sel, ok := indexExpr.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Attributes" {
		return ""
	}
	return unquote(indexExpr.Index)
}

// extractNestedAttrName extracts a nested attribute name from patterns like:
//
//	if nested, ok := attr.Attributes["currency"].(schema.StringAttribute); ok {
//	if nested, ok := attr.NestedObject.Attributes["field"].(schema.StringAttribute); ok {
func extractNestedAttrName(ifStmt *ast.IfStmt) string {
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
	indexExpr, ok := typeAssert.X.(*ast.IndexExpr)
	if !ok {
		return ""
	}
	// Check .Attributes["name"] or .NestedObject.Attributes["name"]
	switch x := indexExpr.X.(type) {
	case *ast.SelectorExpr:
		if x.Sel.Name == "Attributes" {
			return unquote(indexExpr.Index)
		}
	}
	return ""
}

// unquote extracts the string value from a basic literal.
func unquote(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s := lit.Value
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return ""
}

// findReferencedSchemaName finds the generated schema function name referenced
// in a Schema() method body.
func findReferencedSchemaName(fn *ast.FuncDecl) string {
	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
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
