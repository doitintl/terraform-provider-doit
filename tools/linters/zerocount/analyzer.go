// Package zerocount flags TestCheckResourceAttr assertions that a collection
// has zero elements.
//
// terraform-plugin-testing treats a missing count key as zero, so
// TestCheckResourceAttr(addr, "attr.#", "0") and the "attr.%" form pass when the
// attribute is null as well as when it is empty. A test meant to assert [] or {}
// therefore cannot tell them apart from null. statecheck.ExpectKnownValue with
// knownvalue.ListExact, SetExact, MapExact or ObjectExact fails on null.
//
// Where "absent or empty" really is the intent, suppress with
// //nolint:zerocount and a reason.
//
// The check applies to test files only. Key and value are read as constants, so
// a named constant for either is seen through; a non-constant argument is
// skipped.
package zerocount

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	// resourcePkg is matched by full package path so that an unrelated
	// function of the same name is not flagged.
	resourcePkg = "github.com/hashicorp/terraform-plugin-testing/helper/resource"

	checkFunc = "TestCheckResourceAttr"

	// checkArgs is the argument count of TestCheckResourceAttr: name, key, value.
	checkArgs = 3
)

// Analyzer is the go/analysis Analyzer for zerocount.
var Analyzer = &analysis.Analyzer{
	Name:     "zerocount",
	Doc:      "Flags TestCheckResourceAttr zero-count checks (.# / .%), which also pass for null attributes.",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call := n.(*ast.CallExpr)
		if !strings.HasSuffix(pass.Fset.Position(call.Pos()).Filename, "_test.go") {
			return
		}
		if len(call.Args) != checkArgs || !isCheckResourceAttr(pass, call) {
			return
		}

		key, ok := stringConst(pass, call.Args[1])
		if !ok {
			return
		}
		value, ok := stringConst(pass, call.Args[2])
		if !ok || value != "0" {
			return
		}

		var kind string
		switch {
		case strings.HasSuffix(key, ".#"):
			kind = "list or set"
		case strings.HasSuffix(key, ".%"):
			kind = "map or object"
		default:
			return
		}

		pass.Reportf(call.Pos(),
			"%s %q = \"0\" also passes when the attribute is null; assert an empty "+
				"%s with statecheck.ExpectKnownValue and knownvalue.ListExact, "+
				"SetExact, MapExact or ObjectExact, which fail on null, or add "+
				"//nolint:zerocount with a reason if \"absent or empty\" is intended",
			checkFunc, key, kind)
	})

	return nil, nil
}

// isCheckResourceAttr reports whether call invokes resource.TestCheckResourceAttr.
func isCheckResourceAttr(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != checkFunc {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pkgName, ok := pass.TypesInfo.ObjectOf(ident).(*types.PkgName)
	return ok && pkgName.Imported().Path() == resourcePkg
}

// stringConst returns the constant string value of expr.
func stringConst(pass *analysis.Pass, expr ast.Expr) (string, bool) {
	tv, ok := pass.TypesInfo.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}
