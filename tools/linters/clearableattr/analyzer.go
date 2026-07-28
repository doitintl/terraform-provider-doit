// Package clearableattr ensures that every Optional+Computed attribute without
// a Default in every resource's Schema() method is explicitly classified.
//
// Leaf attributes must be either clearable (Category A: a
// use…WhenConfigNull plan modifier) or intentionally not clearable (Category B:
// acknowledgeNotClearable() or //nolint:clearableattr on the override block).
//
// Nested single-nested object containers (the count/forecast_settings shape: an
// optional sub-block inside another object) must be classified as Category A — an
// object plan modifier plus an explicit null-send (<Field>.SetNull()); Category C —
// replace-on-clear via requiresReplaceWhenCleared(...) in the resource's ModifyPlan
// method; or Category B — acknowledgeNotClearable(), for an object that (verified
// empirically) is silently preserved without drift. Some nested O+C objects are
// re-marked "known after apply" when absent from config and drift perpetually
// (needing A or C); others behave like a Cat B leaf, so acknowledgement suffices.
// Top-level
// single-nested objects (structural wrappers like config, effectively always
// present), list containers, and any object nested inside a list element (the list
// clears as a whole via [], and Category C cannot target an individual element)
// are checked only at their leaves.
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

// clearableModifierSuffix and clearableModifierPrefix* define the naming
// convention for plan modifier functions that mark an attribute as clearable.
// Typed variants exist for each Terraform type:
//   - useEmptyForUnknownWhenConfigNull        (string — proposes "")
//   - useNullForUnknownBoolWhenConfigNull     (bool)
//   - useNullForUnknownInt64WhenConfigNull    (int64)
//   - useNullForUnknownFloat64WhenConfigNull  (float64)
//   - useNullForUnknownListWhenConfigNull     (list)
//   - useNullForUnknownStringWhenConfigNull   (string — proposes null)
const (
	clearableModifierPrefixEmpty = "useEmptyForUnknown"
	clearableModifierPrefixNull  = "useNullForUnknown"
	clearableModifierSuffix      = "WhenConfigNull"
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

	// Index every .SetNull() call in the package by the generated schema package
	// its enclosing function is tied to, and by the snake_case field-path the call
	// targets. Category A object containers must send an explicit null; this index
	// is how we verify they do (see setNullIndex.satisfies).
	setNulls := collectSetNulls(insp)

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

		// Find which generated schema this method references, and the package it
		// lives in (e.g. "resource_report"). The package scopes the .SetNull()
		// lookup so a Category A container is only satisfied by a null-send in the
		// same resource, not one that happens to share a field name elsewhere.
		schemaName, schemaPkg := findReferencedSchema(fn)
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
		var findings []finding
		collectUnclassified(schemaInfo.Attrs, "", schemaPkg, false, setNulls, &findings)

		// golangci-lint keeps only one issue per source line (uniq-by-line, on by
		// default), so any two findings sharing a line are silently dropped. Give
		// each finding a distinct line: attributes with an override block use its
		// (unique) line; the rest are spread across the remaining distinct lines of
		// the Schema() body. A borrowed line may belong to an unrelated statement
		// (e.g. the timeouts assignment) — the diagnostic message still names the
		// exact attribute. We collect every distinct body line (not just top-level
		// statements) so we never run out and drop findings.
		usedLines := make(map[int]bool)
		for _, f := range findings {
			if p, ok := attrPositions[f.path]; ok {
				usedLines[pass.Fset.Position(p).Line] = true
			}
		}
		var fallbackPositions []token.Pos
		seenLine := make(map[int]bool)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			pos := n.Pos()
			if !pos.IsValid() {
				return true
			}
			line := pass.Fset.Position(pos).Line
			if usedLines[line] || seenLine[line] {
				return true
			}
			seenLine[line] = true
			fallbackPositions = append(fallbackPositions, pos)
			return true
		})
		slices.Sort(fallbackPositions)

		// Sort for deterministic position assignment and output order.
		slices.SortFunc(findings, func(a, b finding) int {
			if a.path != b.path {
				return strings.Compare(a.path, b.path)
			}
			return int(a.kind) - int(b.kind)
		})

		// Report each finding at a unique position.
		fallbackIdx := 0
		for _, f := range findings {
			var pos token.Pos
			if p, ok := attrPositions[f.path]; ok {
				pos = p
			} else if fallbackIdx < len(fallbackPositions) {
				pos = fallbackPositions[fallbackIdx]
				fallbackIdx++
			} else {
				// Should not happen for real schemas (bodies have far more lines
				// than findings); offset from the brace as a last resort.
				pos = fn.Body.Lbrace + token.Pos(fallbackIdx)
				fallbackIdx++
			}
			pass.Reportf(pos, messageFor(f), f.path)
		}
	})

	return nil, nil
}

// findingKind distinguishes the classification failure so the right diagnostic
// message is emitted.
type findingKind int

const (
	// findingLeaf: an O+C leaf attribute with no clearable modifier and no
	// acknowledgeNotClearable().
	findingLeaf findingKind = iota
	// findingContainer: an O+C single-nested object with neither a clearing plan
	// modifier (Category A) nor requiresReplaceWhenCleared (Category C).
	findingContainer
	// findingMissingNullSend: a Category A object container (has a plan modifier)
	// that never sends an explicit null (.SetNull()) to the API.
	findingMissingNullSend
)

// finding is a single unclassified/misclassified attribute report.
type finding struct {
	path string
	kind findingKind
}

// messageFor returns the diagnostic format string for a finding kind. Each takes
// the attribute path as its single %q argument.
func messageFor(f finding) string {
	const ref = "\tSee: https://github.com/doitintl/terraform-provider-doit/issues/233"
	switch f.kind {
	case findingContainer:
		return "nested Optional+Computed attribute %q has no clearable classification.\n" +
			"\tAttach an object plan modifier that sends null (Category A, like forecast_settings),\n" +
			"\tcall requiresReplaceWhenCleared(...) in ModifyPlan (Category C, like count),\n" +
			"\tor acknowledgeNotClearable() if it is silently preserved without drift (Category B).\n" + ref
	case findingMissingNullSend:
		return "clearable nested attribute %q must send an explicit null to the API:\n" +
			"\tadd <Field>.SetNull() in the request builder (e.g. toExternalConfig/toUpdateRequest).\n" + ref
	default: // findingLeaf
		return "Optional+Computed attribute %q has no clearable classification.\n" +
			"\tAdd useEmptyForUnknownWhenConfigNull() if the attribute should be clearable,\n" +
			"\tor acknowledgeNotClearable() if the prior value should be preserved.\n" + ref
	}
}

// collectUnclassified recursively collects unclassified Optional+Computed
// attributes. Leaf attributes must be Category A (clearing modifier) or Category B
// (acknowledgeNotClearable). Nested single-nested object containers must be
// Category A (object plan modifier + explicit null-send) or Category C
// (requiresReplaceWhenCleared in ModifyPlan). The container check is skipped for
// top-level single-nested objects (structural wrappers), list containers, and any
// object with a list ancestor — see the switch cases below. hasListAncestor is
// true when some enclosing attribute is a list.
func collectUnclassified(attrs map[string]*schemaparser.AttrInfo, prefix, schemaPkg string, hasListAncestor bool, setNulls *setNullIndex, out *[]finding) {
	for name, info := range attrs {
		fullPath := name
		if prefix != "" {
			fullPath = prefix + "." + name
		}

		if info.Class == schemaparser.OptionalComputed && !info.HasDefault {
			switch {
			case info.NestedAttrs == nil:
				// Leaf attribute (checked at any depth, including inside lists).
				if !hasClearableModifier(info.PlanModifiers) && !info.NotClearable {
					*out = append(*out, finding{path: fullPath, kind: findingLeaf})
				}
			case !info.IsList && prefix != "" && !hasListAncestor:
				// Nested single-nested object container (the count/forecast_settings
				// shape: an optional sub-block inside another object). Exempt:
				//   - top-level single-nested objects (prefix == "") — structural
				//     wrappers (e.g. config), effectively always present;
				//   - objects with a list ancestor — a list clears as a whole (via
				//     [] ) and Category C cannot target an individual element, so the
				//     list, not its element sub-objects, is the clearable unit.
				// Their leaves are still checked above.
				switch {
				case info.RequiresReplaceOnClear:
					// Category C — removal forces replacement. OK.
				case info.NotClearable:
					// Category B — prior state sticks and the object is verified not
					// to drift (acknowledgeNotClearable). Not every nested O+C object
					// re-marks "known after apply"; those that don't behave like a
					// Cat B leaf, so acknowledgement is sufficient. OK.
				case hasObjectClearingModifier(info):
					// Category A — must also send an explicit null to the API.
					if !setNulls.satisfies(schemaPkg, fullPath) {
						*out = append(*out, finding{path: fullPath, kind: findingMissingNullSend})
					}
				default:
					*out = append(*out, finding{path: fullPath, kind: findingContainer})
				}
			}
		}

		// Recurse into nested attributes.
		if info.NestedAttrs != nil {
			nestedPrefix := fullPath
			if info.IsList {
				nestedPrefix = fullPath + "[*]"
			}
			collectUnclassified(info.NestedAttrs, nestedPrefix, schemaPkg, hasListAncestor || info.IsList, setNulls, out)
		}
	}
}

// hasObjectClearingModifier reports whether a single-nested object container has a
// plan modifier attached. In this codebase a SingleNestedAttribute only receives a
// plan modifier to handle clearing (e.g. useNullOrDefaultForForecastSettings),
// which does not follow the scalar/list use…WhenConfigNull naming convention, so
// any modifier presence marks the container as Category A (clearable).
func hasObjectClearingModifier(info *schemaparser.AttrInfo) bool {
	return len(info.PlanModifiers) > 0
}

// setNullIndex records, for every <recv>.SetNull() call in the package, the
// snake_case field-path it targets — bucketed by the generated schema package its
// enclosing function is tied to (byPkg), or global when the function references no
// generated package. Scoping by package prevents a null-send in one resource from
// satisfying a same-named container in another.
type setNullIndex struct {
	byPkg  map[string]map[string]bool // genPkg → set of dotted snake field-paths
	global map[string]bool            // dotted snake field-paths, no package tie
}

// satisfies reports whether some .SetNull() call in the given schema package (or a
// package-agnostic one) targets a field-path that is a suffix of the container's
// full attribute path. Suffix matching accepts both fully-qualified accessors
// (plan.Config.ForecastSettings → "config.forecast_settings") and the common
// flattened form (externalConfig.ForecastSettings → "forecast_settings").
func (idx *setNullIndex) satisfies(schemaPkg, path string) bool {
	if suffixMatch(idx.global, path) {
		return true
	}
	if paths, ok := idx.byPkg[schemaPkg]; ok && suffixMatch(paths, path) {
		return true
	}
	return false
}

// suffixMatch reports whether any candidate field-path in the set equals the
// container path or is a trailing segment-run of it.
func suffixMatch(set map[string]bool, path string) bool {
	if set[path] {
		return true
	}
	segs := strings.Split(path, ".")
	for i := 1; i < len(segs); i++ {
		if set[strings.Join(segs[i:], ".")] {
			return true
		}
	}
	return false
}

// collectSetNulls builds a setNullIndex from every zero-arg <recv>.SetNull() call
// in the package, keyed by the generated package(s) referenced in the enclosing
// function.
func collectSetNulls(insp *inspector.Inspector) *setNullIndex {
	idx := &setNullIndex{byPkg: make(map[string]map[string]bool), global: make(map[string]bool)}

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Body == nil {
			return
		}
		genPkgs := genPackagesInFunc(fn)
		paths := setNullPathsInFunc(fn)
		for _, p := range paths {
			if len(genPkgs) == 0 {
				idx.global[p] = true
				continue
			}
			for pkg := range genPkgs {
				if idx.byPkg[pkg] == nil {
					idx.byPkg[pkg] = make(map[string]bool)
				}
				idx.byPkg[pkg][p] = true
			}
		}
	})
	return idx
}

// setNullPathsInFunc returns the snake_case field-paths targeted by zero-arg
// .SetNull() calls in fn (the receiver's root variable is dropped, since it carries
// no schema identity: externalConfig.ForecastSettings → "forecast_settings").
func setNullPathsInFunc(fn *ast.FuncDecl) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "SetNull" {
			return true
		}
		chain := goFieldChain(sel.X)
		if len(chain) == 0 {
			return true
		}
		for i, seg := range chain {
			chain[i] = toSnakeCase(seg)
		}
		out = append(out, strings.Join(chain, "."))
		return true
	})
	return out
}

// goFieldChain returns the selector field names of x with the root variable
// dropped: externalConfig.ForecastSettings → ["ForecastSettings"];
// req.Config.Count → ["Config", "Count"]. Returns nil if x is not a
// variable-rooted selector chain.
func goFieldChain(x ast.Expr) []string {
	var fields []string
	for {
		sel, ok := x.(*ast.SelectorExpr)
		if !ok {
			break
		}
		fields = append([]string{sel.Sel.Name}, fields...)
		x = sel.X
	}
	if _, ok := x.(*ast.Ident); !ok {
		return nil
	}
	return fields
}

// genPackagesInFunc returns the set of generated schema package identifiers
// (convention: "resource_*") referenced anywhere in fn's signature or body.
func genPackagesInFunc(fn *ast.FuncDecl) map[string]bool {
	pkgs := make(map[string]bool)
	collect := func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && strings.HasPrefix(ident.Name, "resource_") {
				pkgs[ident.Name] = true
			}
		}
		return true
	}
	if fn.Recv != nil {
		ast.Inspect(fn.Recv, collect)
	}
	if fn.Type != nil {
		ast.Inspect(fn.Type, collect)
	}
	ast.Inspect(fn.Body, collect)
	return pkgs
}

// toSnakeCase converts a Go field name to snake_case, matching the convention used
// by the requestguard linter's schema-path resolution.
func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// hasClearableModifier returns true if any modifier in the list matches the
// use{Empty,Null}ForUnknown*WhenConfigNull naming convention.
func hasClearableModifier(modifiers []string) bool {
	for _, m := range modifiers {
		if strings.HasSuffix(m, clearableModifierSuffix) &&
			(strings.HasPrefix(m, clearableModifierPrefixEmpty) || strings.HasPrefix(m, clearableModifierPrefixNull)) {
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

// findReferencedSchema finds the generated schema function referenced in a
// Schema() method body, returning its name and the package qualifier it was called
// through (e.g. "ReportResourceSchema", "resource_report"). The package is empty
// for an unqualified (same-package) call.
func findReferencedSchema(fn *ast.FuncDecl) (name, pkg string) {
	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		var n, p string
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			n = f.Sel.Name
			if ident, ok := f.X.(*ast.Ident); ok {
				p = ident.Name
			}
		case *ast.Ident:
			n = f.Name
		default:
			continue
		}
		if strings.HasSuffix(n, "ResourceSchema") || strings.HasSuffix(n, "DataSourceSchema") {
			return n, p
		}
	}
	return "", ""
}
