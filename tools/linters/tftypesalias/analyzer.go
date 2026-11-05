// Package tftypesalias reports writes to the maps and slices that
// tftypes.Value.As returns.
//
// As with a *map[string]tftypes.Value or *[]tftypes.Value destination hands
// back the value's own map or slice, not a copy. Writing to it changes the
// value, which the framework may still hold (for example req.Plan.Raw), so a
// planner that edits "its" map silently edits the plan it was given. Copy
// first (maps.Clone, slices.Clone, or the asCopy helper in internal/provider).
//
// The check is intentionally local: it tracks the variable passed as &v to As
// and reports writes to v, or to a subslice of v, unless v was reassigned
// before them on every path. It does not follow v into
// other variables or functions.
package tftypesalias

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const tftypesPath = "github.com/hashicorp/terraform-plugin-go/tftypes"

// Analyzer is the go/analysis Analyzer for tftypesalias.
var Analyzer = &analysis.Analyzer{
	Name:     "tftypesalias",
	Doc:      "Reports writes to the maps and slices returned by tftypes.Value.As, which are the value's own.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

// inPlaceSliceFuncs are the slices and sort functions that reorder or
// overwrite their first argument.
var inPlaceSliceFuncs = map[string]bool{
	"slices.Reverse": true, "slices.Sort": true, "slices.SortFunc": true,
	"slices.SortStableFunc": true, "slices.Fill": true,
	"slices.Delete": true, "slices.DeleteFunc": true, "slices.Insert": true,
	"slices.Replace": true, "slices.Compact": true, "slices.CompactFunc": true,
	"sort.Slice": true, "sort.SliceStable": true,
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		if fn := n.(*ast.FuncDecl); fn.Body != nil {
			checkBody(pass, fn.Body)
		}
	})
	return nil, nil
}

// reassignment is a plain or short-declaration assignment to an alias
// variable. It clears the alias only for positions inside scope, the block
// that holds it: an assignment under an if, in a loop body or in a closure
// that is never called is not guaranteed to run before a later write.
type reassignment struct {
	end   token.Pos
	scope ast.Node
}

// checkBody collects the alias variables of one function body, then reports
// the writes to them. Positions order the two: a write counts when some As
// call precedes it with no guaranteed reassignment in between.
func checkBody(pass *analysis.Pass, body *ast.BlockStmt) {
	aliasFrom := map[types.Object][]token.Pos{} // object → ends of its As calls
	reassigned := map[types.Object][]reassignment{}

	var stack []ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		switch n := n.(type) {
		case *ast.CallExpr:
			if obj := aliasTarget(pass, n); obj != nil {
				aliasFrom[obj] = append(aliasFrom[obj], n.End())
			}
		case *ast.AssignStmt:
			if n.Tok != token.ASSIGN && n.Tok != token.DEFINE {
				break
			}
			for _, lhs := range n.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				// Uses holds the variables an assignment reuses; a := that
				// declares a new name records it in Defs instead.
				if obj := pass.TypesInfo.Uses[id]; obj != nil {
					reassigned[obj] = append(reassigned[obj], reassignment{n.End(), enclosingScope(stack)})
				}
			}
		}
		return true
	})
	if len(aliasFrom) == 0 {
		return
	}

	// written reports whether expr, possibly sliced, is an alias variable
	// that is live at pos.
	written := func(expr ast.Expr, pos token.Pos) (string, bool) {
		id, ok := unsliced(expr).(*ast.Ident)
		if !ok {
			return "", false
		}
		obj := pass.TypesInfo.Uses[id]
		for _, from := range aliasFrom[obj] {
			if pos < from {
				continue
			}
			cleared := slices.ContainsFunc(reassigned[obj], func(r reassignment) bool {
				return r.end > from && r.end <= pos && r.scope.Pos() <= pos && pos < r.scope.End()
			})
			if !cleared {
				return id.Name, true
			}
		}
		return "", false
	}
	report := func(pos token.Pos, name string) {
		pass.Reportf(pos,
			"writing to %q changes the original value: tftypes.Value.As returns the value's own map or slice (copy it first, e.g. with asCopy)",
			name)
	}
	// indexed reports a write target of the form alias[...].
	indexed := func(lhs ast.Expr) {
		if ie, ok := ast.Unparen(lhs).(*ast.IndexExpr); ok {
			if name, ok := written(ie.X, ie.Pos()); ok {
				report(ie.Pos(), name)
			}
		}
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				indexed(lhs)
			}
		case *ast.IncDecStmt:
			indexed(n.X)
		case *ast.CallExpr:
			if len(n.Args) == 0 || !isWriteCall(pass, n) {
				return true
			}
			if name, ok := written(n.Args[0], n.Pos()); ok {
				report(n.Pos(), name)
			}
		}
		return true
	})
}

// enclosingScope returns the innermost block of the node on top of stack.
func enclosingScope(stack []ast.Node) ast.Node {
	for i := len(stack) - 2; i >= 0; i-- {
		switch stack[i].(type) {
		case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
			return stack[i]
		}
	}
	return stack[0]
}

// unsliced strips parentheses and slice expressions: s[1:] and s[:] share
// the backing array of s.
func unsliced(expr ast.Expr) ast.Expr {
	for {
		switch e := ast.Unparen(expr).(type) {
		case *ast.SliceExpr:
			expr = e.X
		default:
			return e
		}
	}
}

// aliasTarget returns the variable v of a call x.As(&v), where x is a
// tftypes.Value and v is a map[string]tftypes.Value or []tftypes.Value.
func aliasTarget(pass *analysis.Pass, call *ast.CallExpr) types.Object {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "As" || len(call.Args) != 1 || !isTftypesValue(pass.TypesInfo.TypeOf(sel.X)) {
		return nil
	}
	un, ok := call.Args[0].(*ast.UnaryExpr)
	if !ok || un.Op != token.AND {
		return nil
	}
	id, ok := ast.Unparen(un.X).(*ast.Ident)
	if !ok {
		return nil
	}
	obj := pass.TypesInfo.Uses[id]
	if obj == nil {
		return nil
	}
	switch t := obj.Type().Underlying().(type) {
	case *types.Map:
		if isTftypesValue(t.Elem()) {
			return obj
		}
	case *types.Slice:
		if isTftypesValue(t.Elem()) {
			return obj
		}
	}
	return nil
}

// isWriteCall reports whether call is a builtin or library call that mutates
// its first argument.
func isWriteCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		b, ok := pass.TypesInfo.Uses[fun].(*types.Builtin)
		return ok && (b.Name() == "delete" || b.Name() == "clear" || b.Name() == "copy")
	case *ast.SelectorExpr:
		fn, ok := pass.TypesInfo.Uses[fun.Sel].(*types.Func)
		return ok && fn.Pkg() != nil && inPlaceSliceFuncs[fn.Pkg().Path()+"."+fn.Name()]
	}
	return false
}

func isTftypesValue(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Name() == "Value" && obj.Pkg() != nil && obj.Pkg().Path() == tftypesPath
}
