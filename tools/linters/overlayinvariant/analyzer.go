// Package overlayinvariant enforces the invariant that Create/Update methods
// must call an overlay function (not mapResourceToModel/populateState directly),
// and Read/ImportState must call mapResourceToModel/populateState (not an overlay).
//
// This analyzer is schema-aware: it only requires an overlay for resources whose
// schema contains Computed-only or Optional+Computed fields that need to be
// populated from an API response. Resources with only Required/Optional fields
// (e.g., label_assignments) can safely write the plan directly to state.
package overlayinvariant

import (
	"go/ast"
	"strings"

	"github.com/doitintl/terraform-provider-doit/tools/linters/schemaparser"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for overlayinvariant.
var Analyzer = &analysis.Analyzer{
	Name:     "overlayinvariant",
	Doc:      "Ensures Create/Update use overlay functions and Read/ImportState use mapping functions, not vice versa.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer, schemaparser.Analyzer},
}

// mappingFuncNames are the names of functions that should only be called
// from Read/ImportState, never from Create/Update.
var mappingFuncNames = map[string]bool{
	"mapResourceToModel": true,
	"populateState":      true,
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	facts := pass.ResultOf[schemaparser.Analyzer].(*schemaparser.SchemaFacts)

	// First pass: collect all Schema() methods and their generated schema function
	// names, keyed by receiver type name. This lets us map Create/Update methods
	// back to their resource's schema.
	schemaMethodMap := buildSchemaMethodMap(insp)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil {
			return
		}
		// Only check methods on resource types (have a receiver).
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		methodName := fn.Name.Name
		isCreateUpdate := methodName == "Create" || methodName == "Update"
		isReadImport := methodName == "Read" || methodName == "ImportState"

		if !isCreateUpdate && !isReadImport {
			return
		}

		// Walk the function body looking for function calls.
		overlayCallFound := false
		setsState := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			calledName := calledFuncName(call)
			if calledName == "" {
				return true
			}

			if isOverlayFunc(calledName) {
				overlayCallFound = true
			}

			// Detect resp.State.Set(...) calls — indicates state population.
			if calledName == "Set" {
				setsState = true
			}

			if isCreateUpdate && isMappingFunc(calledName) {
				pass.Reportf(call.Pos(),
					"%s must not call %s directly; use the overlay function (e.g., overlay*ComputedFields) instead",
					methodName, calledName)
			}

			if isReadImport && isOverlayFunc(calledName) {
				pass.Reportf(call.Pos(),
					"%s must not call %s; use mapResourceToModel or populateState instead",
					methodName, calledName)
			}

			return true
		})

		// Create/Update must call an overlay function to populate state,
		// but only if:
		//   1. The method actually sets state (calls resp.State.Set)
		//   2. The resource's schema has fields that require an overlay
		//      (Computed-only or Optional+Computed)
		if isCreateUpdate && !overlayCallFound && setsState {
			if needsOverlay(fn, facts, schemaMethodMap) {
				pass.Reportf(fn.Pos(),
					"%s must call an overlay function (overlay*ComputedFields) to populate state; "+
						"do not assign plan fields inline or use ad-hoc helpers",
					methodName)
			}
		}
	})

	return nil, nil
}

// buildSchemaMethodMap collects Schema() methods and extracts which generated
// schema function they call. Returns a map from receiver type name to the
// generated schema function name (e.g., "reportResource" -> "ReportResourceSchema").
func buildSchemaMethodMap(insp *inspector.Inspector) map[string]string {
	result := make(map[string]string)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Name.Name != "Schema" || fn.Body == nil {
			return
		}
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		recvType := receiverTypeName(fn)
		if recvType == "" {
			return
		}

		// Look for a call to XxxResourceSchema or XxxDataSourceSchema.
		// Handles both package-qualified (pkg.XxxResourceSchema) and local
		// (XxxResourceSchema) calls.
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			var name string
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			case *ast.Ident:
				name = fn.Name
			default:
				return true
			}

			if strings.HasSuffix(name, "ResourceSchema") || strings.HasSuffix(name, "DataSourceSchema") {
				result[recvType] = name
				return false // stop searching
			}
			return true
		})
	})

	return result
}

// needsOverlay checks whether the resource's schema has any Computed-only or
// Optional+Computed fields that require an overlay. If the schema can't be
// determined, returns true (conservative: assume overlay is needed).
func needsOverlay(fn *ast.FuncDecl, facts *schemaparser.SchemaFacts, schemaMethodMap map[string]string) bool {
	recvType := receiverTypeName(fn)
	if recvType == "" {
		return true // can't determine, assume needed
	}

	schemaFuncName, ok := schemaMethodMap[recvType]
	if !ok {
		// No Schema() method found for this type. This could be a resource
		// that defines its schema inline (not using a generated function).
		// Check if there's even a Schema method at all.
		return false // no generated schema = assume no overlay needed
	}

	schemaInfo, ok := facts.Schemas[schemaFuncName]
	if !ok {
		return true // schema exists but not parsed, assume needed
	}

	// Check if any field is Computed-only or Optional+Computed.
	for _, attr := range schemaInfo.Attrs {
		if attr.Class == schemaparser.ComputedOnly || attr.Class == schemaparser.OptionalComputed {
			return true
		}
	}

	return false
}

// receiverTypeName extracts the receiver type name from a method declaration.
// Handles both pointer (*myType) and value (myType) receivers.
func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	recvType := fn.Recv.List[0].Type
	// Unwrap pointer receiver.
	if star, ok := recvType.(*ast.StarExpr); ok {
		recvType = star.X
	}
	ident, ok := recvType.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

// calledFuncName extracts the function name from a call expression.
// Handles both direct calls (populateState(...)) and method calls (r.populateState(...)).
func calledFuncName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

// isMappingFunc checks if a function name matches a mapping function pattern.
func isMappingFunc(name string) bool {
	if mappingFuncNames[name] {
		return true
	}
	// Also match mapXxxToModel pattern.
	if strings.HasPrefix(name, "map") && strings.HasSuffix(name, "ToModel") {
		return true
	}
	return false
}

// isOverlayFunc checks if a function name matches an overlay function pattern.
func isOverlayFunc(name string) bool {
	return strings.HasPrefix(name, "overlay")
}

