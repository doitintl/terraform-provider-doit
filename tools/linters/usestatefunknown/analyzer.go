// Package usestatefunknown ensures that Computed-only fields named "id",
// "create_time", and "update_time" have UseStateForUnknown() plan modifiers
// in the resource's Schema() method.
//
// These fields are universally stable after creation and should not show as
// "(known after apply)" in every plan. The UseStateForUnknown plan modifier
// tells Terraform to carry the prior state value forward, reducing plan noise.
//
// The analyzer uses schemaparser to identify Computed-only fields and then
// checks whether the Schema() method adds UseStateForUnknown for each.
package usestatefunknown

import (
	"go/ast"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for usestatefunknown.
var Analyzer = &analysis.Analyzer{
	Name:     "usestatefunknown",
	Doc:      "Ensures Computed-only stable fields (id, create_time, update_time) have UseStateForUnknown plan modifiers.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

// stableFields are Computed-only fields that should always have UseStateForUnknown.
// These are universally stable after creation — they never change on Update.
// update_time is intentionally excluded: it changes on every Update, so
// UseStateForUnknown would hide a legitimate "(known after apply)" signal.
var stableFields = map[string]bool{
	"id":          true,
	"create_time": true,
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

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Find each Schema() method and check for UseStateForUnknown on stable fields
	// from the specific schema that method references (not all schemas in the package).
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
		schemaInfo, ok := schemaResult.Schemas[schemaName]
		if !ok {
			return
		}

		// Build Computed-only stable fields for THIS specific schema only.
		computedStable := map[string]bool{}
		for fieldName, info := range schemaInfo.Attrs {
			if info.Class == schemaparser.ComputedOnly && stableFields[fieldName] {
				computedStable[fieldName] = true
			}
		}
		if len(computedStable) == 0 {
			return
		}

		// Collect field names that get UseStateForUnknown in this method.
		fieldsWithModifier := findFieldsWithUseStateForUnknown(fn.Body)

		// Collect field names that are fully overridden (e.g., id changed
		// from Computed to Required). These fields have a new classification
		// chosen by the user and should not be flagged.
		overriddenFields := findFullyOverriddenFields(fn.Body)

		// Report any stable Computed-only fields missing the modifier.
		for field := range computedStable {
			if fieldsWithModifier[field] || overriddenFields[field] {
				continue
			}
			pass.Reportf(fn.Pos(),
				"Computed-only field %q should have UseStateForUnknown() plan modifier "+
					"to avoid unnecessary \"(known after apply)\" in plans",
				field)
		}
	})

	return nil, nil
}

// findReferencedSchemaName finds the generated schema function name referenced
// in a Schema() method body. For example, in:
//
//	s := resource_folder.FolderResourceSchema(ctx)
//
// it returns "FolderResourceSchema".
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
		// Match: pkg.XxxSchema(ctx)
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			name := sel.Sel.Name
			if strings.HasSuffix(name, "Schema") {
				return name
			}
		}
		// Match: XxxSchema(ctx) — same-package call (used in tests)
		if ident, ok := call.Fun.(*ast.Ident); ok {
			if strings.HasSuffix(ident.Name, "Schema") {
				return ident.Name
			}
		}
	}
	return ""
}

// findFieldsWithUseStateForUnknown scans a Schema() method body and returns
// the set of attribute names that have UseStateForUnknown added.
//
// It detects two patterns:
//
// Pattern 1 (map access + modify):
//
//	attr := s.Attributes["id"].(schema.StringAttribute)
//	attr.PlanModifiers = append(attr.PlanModifiers, stringplanmodifier.UseStateForUnknown())
//	s.Attributes["id"] = attr
//
// Pattern 2 (inline struct):
//
//	s.Attributes["id"] = schema.StringAttribute{
//	    PlanModifiers: []planmodifier.String{
//	        stringplanmodifier.UseStateForUnknown(),
//	    },
//	}
func findFieldsWithUseStateForUnknown(body *ast.BlockStmt) map[string]bool {
	result := map[string]bool{}

	// Strategy: walk the entire AST body to find all assign statements.
	// Track variable-to-field mappings (attr := s.Attributes["id"]...)
	// and detect UseStateForUnknown calls in assign or expression statements.
	varToField := map[string]string{}

	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, rhs := range assign.Rhs {
			// Track: attr := s.Attributes["id"].(schema.XxxAttribute)
			fieldName := extractAttributeMapKey(rhs)
			if fieldName != "" && i < len(assign.Lhs) {
				if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
					varToField[ident.Name] = fieldName
				}
			}
		}

		// Check: s.Attributes["id"] = schema.XxxAttribute{...UseStateForUnknown()...}
		for i, lhs := range assign.Lhs {
			fieldName := extractAttributeMapKey(lhs)
			if fieldName != "" && i < len(assign.Rhs) {
				if containsUseStateForUnknown(assign.Rhs[i]) {
					result[fieldName] = true
				}
			}
		}

		// Check: attr.PlanModifiers = append(..., UseStateForUnknown())
		for _, rhs := range assign.Rhs {
			if !containsUseStateForUnknown(rhs) {
				continue
			}
			for _, lhs := range assign.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok {
					if ident, ok := sel.X.(*ast.Ident); ok {
						if fieldName, exists := varToField[ident.Name]; exists {
							result[fieldName] = true
						}
					}
				}
			}
		}

		return true
	})

	return result
}

// findFullyOverriddenFields finds attribute names whose entire schema definition
// is replaced in the Schema() method body. For example:
//
//	s.Attributes["id"] = schema.StringAttribute{Required: true, ...}
//
// This changes the field's classification (e.g., Computed→Required), so we
// should not flag it for missing UseStateForUnknown.
func findFullyOverriddenFields(body *ast.BlockStmt) map[string]bool {
	result := map[string]bool{}

	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}

		for i, lhs := range assign.Lhs {
			fieldName := extractAttributeMapKey(lhs)
			if fieldName == "" || i >= len(assign.Rhs) {
				continue
			}
			// RHS must be a composite literal (full schema replacement).
			if _, ok := assign.Rhs[i].(*ast.CompositeLit); ok {
				result[fieldName] = true
			}
		}

		return true
	})

	return result
}

// extractAttributeMapKey returns the string key from expressions like:
// s.Attributes["id"] or s.Attributes["id"].(schema.StringAttribute)
func extractAttributeMapKey(expr ast.Expr) string {
	// Handle type assertion: s.Attributes["id"].(schema.XxxAttribute)
	if ta, ok := expr.(*ast.TypeAssertExpr); ok {
		return extractAttributeMapKey(ta.X)
	}

	// Handle map index: s.Attributes["id"]
	ie, ok := expr.(*ast.IndexExpr)
	if !ok {
		return ""
	}
	// Check that the receiver is something.Attributes
	sel, ok := ie.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Attributes" {
		return ""
	}
	// Extract the string key
	lit, ok := ie.Index.(*ast.BasicLit)
	if !ok {
		return ""
	}
	// Remove quotes
	return strings.Trim(lit.Value, "\"")
}

// containsUseStateForUnknown checks if an expression tree contains a call
// to UseStateForUnknown().
func containsUseStateForUnknown(expr ast.Node) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name == "UseStateForUnknown" {
			found = true
			return false
		}
		return true
	})
	return found
}
