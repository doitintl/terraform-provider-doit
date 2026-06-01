// Package diagdrop detects leaked diagnostics in two directions:
//
// Direction 1 (return nil): functions that capture diag.Diagnostics into a
// named variable but return nil on some paths, silently dropping non-error
// diagnostics (e.g. warnings).
//
// Bad:
//
//	func populateState(...) diag.Diagnostics {
//	    user, diags := r.lookupUser(ctx, email)
//	    if diags.HasError() { return diags }
//	    return nil  // ← drops non-error diags
//	}
//
// Good:
//
//	func populateState(...) diag.Diagnostics {
//	    user, diags := r.lookupUser(ctx, email)
//	    if diags.HasError() { return diags }
//	    return diags  // ← propagates all diags
//	}
//
// Functions that never capture diagnostics into a named variable are excluded
// (e.g. overlay helpers that return nil because they truly produce no diagnostics).
//
// Early-return nil guards that precede any diag variable assignment are also
// excluded (e.g. `if x == nil { return NullValue(), nil }`).
//
// Direction 2 (unappended diags): in functions that take a resp parameter with
// a Diagnostics field (CRUD methods, validators), captured diag.Diagnostics
// variables that are never appended to resp.Diagnostics or passed to another
// function are flagged.
//
// Bad:
//
//	func (v myValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
//	    diags := req.Config.GetAttribute(ctx, p, &val)
//	    if diags.HasError() { return }  // ← diags never appended to resp.Diagnostics
//	}
//
// Good:
//
//	func (v myValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
//	    diags := req.Config.GetAttribute(ctx, p, &val)
//	    resp.Diagnostics.Append(diags...)  // ← properly propagated
//	    if diags.HasError() { return }
//	}
package diagdrop

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for diagdrop.
var Analyzer = &analysis.Analyzer{
	Name:     "diagdrop",
	Doc:      "Detects return nil that drops captured diag.Diagnostics (non-error diagnostics silently lost).",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

const diagType = "github.com/hashicorp/terraform-plugin-framework/diag.Diagnostics"

// diagVarInfo holds information about a named diag.Diagnostics variable.
type diagVarInfo struct {
	name          string
	pos           token.Pos
	isAccumulator bool // true for named returns and var declarations, false for := temporaries
	obj           types.Object
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Body != nil && fn.Type != nil {
				// Direction 1: return nil drops captured diags.
				checkFunc(pass, fn.Type, fn.Body)
				// Direction 2: captured diags never appended to resp.Diagnostics.
				checkUnappendedDiags(pass, fn.Type, fn.Body)
			}
		case *ast.FuncLit:
			if fn.Body != nil && fn.Type != nil {
				checkFunc(pass, fn.Type, fn.Body)
				checkUnappendedDiags(pass, fn.Type, fn.Body)
			}
		}
	})

	return nil, nil
}

// checkFunc analyzes a single function for the diagdrop pattern.
func checkFunc(pass *analysis.Pass, funcType *ast.FuncType, body *ast.BlockStmt) {
	// Step 1: Find which return-value position (if any) is diag.Diagnostics.
	diagPos := findDiagReturnPos(pass, funcType)
	if diagPos < 0 {
		return
	}

	// Step 2: Collect all named diag.Diagnostics variables with their positions.
	// Named return parameters are treated as having position 0 (always in scope).
	diagVars := collectDiagVars(pass, funcType, body)
	if len(diagVars) == 0 {
		// No named diag.Diagnostics variable — function constructs diags inline.
		// return nil is intentional (no diags to propagate).
		return
	}

	// Step 3: Walk all return statements and flag return nil at the diag position
	// only if a diag variable assignment precedes the return.
	ast.Inspect(body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return false
		}
		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		// Bare return (named returns) — not flagged; the named variable is implicitly returned.
		if len(ret.Results) == 0 {
			return true
		}

		// Find the expression at the diag.Diagnostics position.
		if diagPos >= len(ret.Results) {
			return true
		}
		expr := ret.Results[diagPos]

		if !isNilLiteral(expr) {
			return true
		}

		// Find the first diag variable assigned before this return.
		varName := latestDiagVarBefore(pass, diagVars, ret.Pos())
		if varName == "" {
			// No diag variable assigned before this return — safe early return.
			return true
		}

		pass.Reportf(expr.Pos(),
			"return nil drops captured diag.Diagnostics variable %q; return %s instead",
			varName, varName)

		return true
	})
}

// findDiagReturnPos returns the 0-based index of the diag.Diagnostics return
// value, or -1 if the function doesn't return diag.Diagnostics.
func findDiagReturnPos(pass *analysis.Pass, funcType *ast.FuncType) int {
	if funcType.Results == nil {
		return -1
	}

	pos := 0
	for _, field := range funcType.Results.List {
		typ := pass.TypesInfo.TypeOf(field.Type)
		if typ != nil && isDiagnosticsType(typ) {
			return pos
		}
		// A field with multiple names (e.g. "a, b int") counts for each name.
		names := len(field.Names)
		if names == 0 {
			names = 1
		}
		pos += names
	}
	return -1
}

// collectDiagVars returns all named diag.Diagnostics variables found in the
// function signature (named return params) and body (assignments).
// Named return parameters use position 0 to indicate they are always in scope.
func collectDiagVars(pass *analysis.Pass, funcType *ast.FuncType, body *ast.BlockStmt) []diagVarInfo {
	var vars []diagVarInfo

	// Check named return parameters — always in scope (position 0).
	if funcType.Results != nil {
		for _, field := range funcType.Results.List {
			typ := pass.TypesInfo.TypeOf(field.Type)
			if typ != nil && isDiagnosticsType(typ) {
				for _, name := range field.Names {
					if name.Name != "" && name.Name != "_" {
						vars = append(vars, diagVarInfo{name: name.Name, pos: 0, isAccumulator: true, obj: pass.TypesInfo.Defs[name]})
					}
				}
			}
		}
	}

	// Scan function body for var declarations (e.g. var diags diag.Diagnostics)
	// and assignments (e.g. diags := lookupUser(...)) that capture diag.Diagnostics.
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return false
		case *ast.GenDecl:
			// Handle: var diags diag.Diagnostics
			if node.Tok != token.VAR {
				return true
			}
			for _, spec := range node.Specs {
				vSpec, ok := spec.(*ast.ValueSpec)
				if !ok || vSpec.Type == nil {
					continue
				}
				typ := pass.TypesInfo.TypeOf(vSpec.Type)
				if typ != nil && isDiagnosticsType(typ) {
					for _, name := range vSpec.Names {
						if name.Name != "" && name.Name != "_" {
							vars = append(vars, diagVarInfo{name: name.Name, pos: node.Pos(), isAccumulator: true, obj: pass.TypesInfo.Defs[name]})
						}
					}
				}
			}

		case *ast.AssignStmt:
			// Handle: diags := lookupUser(...) or d := mapToModel()
			for i, lhs := range node.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || ident.Name == "_" {
					continue
				}

				var typ types.Type
				if len(node.Rhs) == 1 {
					// Multi-return: check tuple position.
					rhsType := pass.TypesInfo.TypeOf(node.Rhs[0])
					if tuple, ok := rhsType.(*types.Tuple); ok && i < tuple.Len() {
						typ = tuple.At(i).Type()
					} else if i == 0 {
						typ = rhsType
					}
				} else if i < len(node.Rhs) {
					typ = pass.TypesInfo.TypeOf(node.Rhs[i])
				}

				if typ != nil && isDiagnosticsType(typ) {
					obj := pass.TypesInfo.Defs[ident]
					if obj == nil {
						obj = pass.TypesInfo.Uses[ident]
					}
					vars = append(vars, diagVarInfo{name: ident.Name, pos: node.Pos(), obj: obj})
				}
			}
		}
		return true
	})

	return vars
}

// latestDiagVarBefore returns the name of the best diag variable to suggest
// returning, among those declared or assigned before the given position.
// It prefers accumulators (named returns, var declarations) over assignment
// temporaries. Among equal-priority variables, it picks the latest.
func latestDiagVarBefore(pass *analysis.Pass, vars []diagVarInfo, pos token.Pos) string {
	innerScope := findInnermostScope(pass, pos)
	if innerScope == nil {
		innerScope = pass.Pkg.Scope()
	}

	var best string
	var bestPos token.Pos
	var bestIsAccum bool
	for _, v := range vars {
		if v.pos >= pos {
			continue
		}
		// Verify that the variable is still in scope at 'pos' and resolves to the same object.
		if v.obj != nil {
			_, lookupObj := innerScope.LookupParent(v.name, pos)
			if lookupObj != v.obj {
				continue
			}
		}
		// Prefer accumulators over temporaries; among same kind, prefer latest.
		if best == "" || (v.isAccumulator && !bestIsAccum) || (v.isAccumulator == bestIsAccum && v.pos >= bestPos) {
			best = v.name
			bestPos = v.pos
			bestIsAccum = v.isAccumulator
		}
	}
	return best
}

// findInnermostScope returns the innermost types.Scope containing the given position by searching pass.TypesInfo.Scopes.
func findInnermostScope(pass *analysis.Pass, pos token.Pos) *types.Scope {
	var innermost *types.Scope
	for _, scope := range pass.TypesInfo.Scopes {
		if scope.Pos() <= pos && pos <= scope.End() {
			if innermost == nil || scope.End()-scope.Pos() < innermost.End()-innermost.Pos() {
				innermost = scope
			}
		}
	}
	return innermost
}

// isNilLiteral checks if an expression is the nil identifier.
func isNilLiteral(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

// isDiagnosticsType checks if a type is diag.Diagnostics.
func isDiagnosticsType(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return obj.Pkg().Path()+"."+obj.Name() == diagType
}

// --- Direction 2: unappended diagnostics ---

// checkUnappendedDiags flags captured diag.Diagnostics variables that are never
// appended to resp.Diagnostics (or passed to another function) in functions
// that take a resp parameter with a Diagnostics field.
func checkUnappendedDiags(pass *analysis.Pass, funcType *ast.FuncType, body *ast.BlockStmt) {
	// Only applies to functions with a resp-style parameter.
	if !hasRespDiagnosticsParam(pass, funcType) {
		return
	}

	// Collect all captured diag.Diagnostics variables in the body.
	allVars := collectDiagVars(pass, funcType, body)
	if len(allVars) == 0 {
		return
	}

	// Find which variable objects are propagated (appended, passed, or returned).
	// Uses types.Object identity, not names, to handle same-name variables in
	// different scopes (e.g., two loops both declaring diags := ...).
	propagated := collectPropagatedObjects(pass, body)

	// Flag variables that are captured but never propagated.
	for _, v := range allVars {
		// Skip named return params — they are implicitly returned.
		if v.isAccumulator && v.pos == 0 {
			continue
		}
		if v.obj != nil && propagated[v.obj] {
			continue
		}
		pass.Reportf(v.pos,
			"captured diag.Diagnostics variable %q is never appended to resp.Diagnostics",
			v.name)
	}
}

// hasRespDiagnosticsParam checks if a function has a parameter whose underlying
// type contains a Diagnostics field of type diag.Diagnostics. This matches
// resource.CreateResponse, resource.ValidateConfigResponse, etc.
func hasRespDiagnosticsParam(pass *analysis.Pass, funcType *ast.FuncType) bool {
	if funcType.Params == nil {
		return false
	}
	for _, field := range funcType.Params.List {
		typ := pass.TypesInfo.TypeOf(field.Type)
		if typ == nil {
			continue
		}
		// Unwrap pointer.
		if ptr, ok := typ.(*types.Pointer); ok {
			typ = ptr.Elem()
		}
		// Check if the struct has a Diagnostics field of type diag.Diagnostics.
		st, ok := typ.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			if f.Name() == "Diagnostics" && isDiagnosticsType(f.Type()) {
				return true
			}
		}
	}
	return false
}

// collectPropagatedObjects walks a function body and returns the set of
// types.Object identities that are propagated via:
//   - *.Append(varName...) calls
//   - someFunc(varName) or someFunc(varName...) calls (passed as argument)
//   - return statements that include the variable
//
// Using types.Object instead of names ensures that same-name variables in
// different scopes (e.g., loop iterations) are tracked independently.
func collectPropagatedObjects(pass *analysis.Pass, body *ast.BlockStmt) map[types.Object]bool {
	result := map[types.Object]bool{}

	ast.Inspect(body, func(n ast.Node) bool {
		// Skip nested function literals.
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}

		switch node := n.(type) {
		case *ast.CallExpr:
			// Check all arguments (covers both Append and general function calls).
			for _, arg := range node.Args {
				if ident, ok := arg.(*ast.Ident); ok {
					if obj := resolveObject(pass, ident); obj != nil {
						result[obj] = true
					}
				}
			}

		case *ast.ReturnStmt:
			for _, expr := range node.Results {
				if ident, ok := expr.(*ast.Ident); ok {
					if obj := resolveObject(pass, ident); obj != nil {
						result[obj] = true
					}
				}
			}
		}

		return true
	})

	return result
}

// resolveObject returns the types.Object for an identifier, preferring Uses over Defs.
func resolveObject(pass *analysis.Pass, ident *ast.Ident) types.Object {
	if obj := pass.TypesInfo.Uses[ident]; obj != nil {
		return obj
	}
	return pass.TypesInfo.Defs[ident]
}
