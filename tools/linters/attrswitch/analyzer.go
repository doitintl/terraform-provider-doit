// Package attrswitch requires type switches over schema.Attribute to be
// exhaustive where being partial silently skips values.
//
// Generic code that walks a schema switches on the attribute kind. A switch
// that handles some nested attribute kinds but not others skips every value of
// the missing kind, and a switch that reads PlanModifiers, Validators or
// Default from some kinds ignores them on the rest. Neither fails loudly.
//
// A switch is reported when it:
//   - handles any of SingleNestedAttribute, ListNestedAttribute,
//     SetNestedAttribute or MapNestedAttribute but not all four, or
//   - reads PlanModifiers, Validators or Default from the case variable but
//     does not handle every attribute kind that has that field.
//
// A default clause with a comment states that the remaining kinds are skipped
// on purpose and exempts the switch.
package attrswitch

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for attrswitch.
var Analyzer = &analysis.Analyzer{
	Name:     "attrswitch",
	Doc:      "Requires type switches over schema.Attribute to handle every nested kind and every kind with the PlanModifiers, Validators or Default field they read.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

// nestedKinds are the attribute kinds that contain further attributes.
var nestedKinds = []string{
	"SingleNestedAttribute",
	"ListNestedAttribute",
	"SetNestedAttribute",
	"MapNestedAttribute",
}

// checkedFields are the attribute fields that must be read from every kind
// that has them.
var checkedFields = []string{"PlanModifiers", "Validators", "Default"}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.Preorder([]ast.Node{(*ast.TypeSwitchStmt)(nil)}, func(n ast.Node) {
		sw := n.(*ast.TypeSwitchStmt)
		pkg := attributePackage(pass, sw)
		if pkg == nil {
			return
		}
		if hasCommentedDefault(pass, sw) {
			return
		}

		handled := map[string]bool{}
		reads := map[string]bool{}
		for _, stmt := range sw.Body.List {
			clause := stmt.(*ast.CaseClause)
			for _, expr := range clause.List {
				if name := kindName(pass, expr, pkg); name != "" {
					handled[name] = true
				}
			}
			for _, field := range fieldsRead(pass, clause) {
				reads[field] = true
			}
		}

		var problems []string
		if missing := missingNested(handled); missing != nil {
			problems = append(problems, "handles some nested attribute kinds but not "+strings.Join(missing, ", "))
		}
		for _, field := range checkedFields {
			if !reads[field] {
				continue
			}
			if missing := missingWithField(pkg, field, handled); missing != nil {
				problems = append(problems, "reads "+field+" but does not handle "+strings.Join(missing, ", "))
			}
		}
		if problems == nil {
			return
		}
		pass.Reportf(sw.Pos(),
			"type switch over schema.Attribute %s; handle them or add a default clause with a comment saying why they are skipped",
			strings.Join(problems, " and "))
	})
	return nil, nil
}

// attributePackage returns the schema package when the switch is over a
// schema.Attribute from a resource, data source or provider schema package.
func attributePackage(pass *analysis.Pass, sw *ast.TypeSwitchStmt) *types.Package {
	var x ast.Expr
	switch a := sw.Assign.(type) {
	case *ast.AssignStmt:
		if len(a.Rhs) == 1 {
			x = a.Rhs[0]
		}
	case *ast.ExprStmt:
		x = a.X
	}
	assert, ok := x.(*ast.TypeAssertExpr)
	if !ok {
		return nil
	}
	named, ok := types.Unalias(pass.TypesInfo.TypeOf(assert.X)).(*types.Named)
	if !ok || named.Obj().Name() != "Attribute" || named.Obj().Pkg() == nil {
		return nil
	}
	path := named.Obj().Pkg().Path()
	for _, suffix := range []string{"/resource/schema", "/datasource/schema", "/provider/schema"} {
		if strings.HasSuffix(path, suffix) {
			return named.Obj().Pkg()
		}
	}
	return nil
}

// kindName returns the name of the schema attribute type a case lists.
func kindName(pass *analysis.Pass, expr ast.Expr, pkg *types.Package) string {
	t := types.Unalias(pass.TypesInfo.TypeOf(expr))
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj().Pkg() != pkg {
		return ""
	}
	return named.Obj().Name()
}

// hasCommentedDefault reports whether the switch has a default clause that
// carries a comment.
func hasCommentedDefault(pass *analysis.Pass, sw *ast.TypeSwitchStmt) bool {
	var file *ast.File
	for _, f := range pass.Files {
		if f.FileStart <= sw.Pos() && sw.End() <= f.FileEnd {
			file = f
			break
		}
	}
	if file == nil {
		return false
	}
	for i, stmt := range sw.Body.List {
		clause := stmt.(*ast.CaseClause)
		if clause.List != nil {
			continue
		}
		// The clause ends with its last statement; an empty one extends to the
		// next clause, or to the closing brace of the switch if it is last.
		// The next clause's own leading comment is not the default's.
		from, to := clause.Pos(), clause.End()
		followed := i+1 < len(sw.Body.List)
		if len(clause.Body) == 0 {
			to = sw.Body.Rbrace
			if followed {
				to = sw.Body.List[i+1].Pos()
			}
		}
		lastLine := pass.Fset.Position(clause.End()).Line
		for _, group := range file.Comments {
			if group.Pos() < from {
				continue
			}
			// A comment that starts on the line the clause ends on trails it.
			if pass.Fset.Position(group.Pos()).Line == lastLine {
				return true
			}
			if group.End() > to {
				continue
			}
			if len(clause.Body) == 0 && followed &&
				pass.Fset.Position(group.End()).Line+1 == pass.Fset.Position(to).Line {
				continue
			}
			return true
		}
	}
	return false
}

// fieldsRead returns the checked fields the clause reads from its case
// variable. Plain assignment targets are writes, not reads.
func fieldsRead(pass *analysis.Pass, clause *ast.CaseClause) []string {
	obj := pass.TypesInfo.Implicits[clause]
	if obj == nil {
		return nil
	}
	var fields []string
	written := map[*ast.SelectorExpr]bool{} // targets of plain assignments
	for _, stmt := range clause.Body {
		ast.Inspect(stmt, func(n ast.Node) bool {
			if as, ok := n.(*ast.AssignStmt); ok && as.Tok == token.ASSIGN {
				for _, lhs := range as.Lhs {
					if sel, ok := lhs.(*ast.SelectorExpr); ok {
						written[sel] = true
					}
				}
			}
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || written[sel] {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || pass.TypesInfo.Uses[id] != obj {
				return true
			}
			for _, f := range checkedFields {
				if sel.Sel.Name == f {
					fields = append(fields, f)
				}
			}
			return true
		})
	}
	return fields
}

// missingNested returns the nested kinds the switch does not handle, or nil
// when it handles none or all of them.
func missingNested(handled map[string]bool) []string {
	var missing []string
	seen := false
	for _, k := range nestedKinds {
		if handled[k] {
			seen = true
		} else {
			missing = append(missing, k)
		}
	}
	if !seen {
		return nil
	}
	return missing
}

// missingWithField returns the attribute kinds of pkg that have the field and
// that the switch does not handle.
func missingWithField(pkg *types.Package, field string, handled map[string]bool) []string {
	attr, ok := pkg.Scope().Lookup("Attribute").Type().Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	var missing []string
	for _, name := range pkg.Scope().Names() {
		tn, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok || handled[name] {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok || !types.Implements(tn.Type(), attr) || !hasField(st, field) {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return missing
}

func hasField(st *types.Struct, name string) bool {
	for f := range st.Fields() {
		if f.Name() == name {
			return true
		}
	}
	return false
}
