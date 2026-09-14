// Package validatoranalysis contains semantic helpers shared by validator linters.
package validatoranalysis

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

const (
	attrPackage = "github.com/hashicorp/terraform-plugin-framework/attr"
	diagPackage = "github.com/hashicorp/terraform-plugin-framework/diag"
)

// PredicateKind identifies a Terraform value-state predicate.
type PredicateKind uint8

const (
	PredicateUnknown PredicateKind = iota + 1
	PredicateNull
)

// ValueKey is the semantic identity of a Terraform value expression.
type ValueKey struct {
	Root types.Object
	Path string
}

// ValueRef couples a semantic identity with a user-facing expression name.
type ValueRef struct {
	Key     ValueKey
	Display string
}

// ValueSet contains semantic Terraform value identities.
type ValueSet map[ValueKey]ValueRef

// KnownFacts records values proven not unknown by canonical structured flow.
type KnownFacts struct {
	AtIf   map[token.Pos]ValueSet
	AtCall map[token.Pos]ValueSet
}

// Functions contains declarations and validator roots in one package.
type Functions struct {
	Declarations map[*types.Func]*ast.FuncDecl
	Roots        []*ast.FuncDecl
}

// DiscoverFunctions indexes functions and finds framework validator entry points.
func DiscoverFunctions(pass *analysis.Pass, insp *inspector.Inspector) Functions {
	result := Functions{Declarations: map[*types.Func]*ast.FuncDecl{}}
	insp.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(node ast.Node) {
		decl := node.(*ast.FuncDecl)
		if decl.Body == nil || decl.Name == nil {
			return
		}
		fn, ok := pass.TypesInfo.Defs[decl.Name].(*types.Func)
		if !ok {
			return
		}
		result.Declarations[fn] = decl
		if IsValidatorEntryPoint(fn) {
			result.Roots = append(result.Roots, decl)
		}
	})
	return result
}

// ReachableFunctions returns validator roots and all same-package callees.
func ReachableFunctions(pass *analysis.Pass, functions Functions) map[*types.Func]*ast.FuncDecl {
	result := map[*types.Func]*ast.FuncDecl{}
	var visit func(*ast.FuncDecl)
	visit = func(decl *ast.FuncDecl) {
		fn, ok := pass.TypesInfo.Defs[decl.Name].(*types.Func)
		if !ok || result[fn] != nil {
			return
		}
		result[fn] = decl
		Inspect(decl.Body, func(node ast.Node) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return
			}
			if next := functions.Declarations[typeutil.StaticCallee(pass.TypesInfo, call)]; next != nil {
				visit(next)
			}
		})
	}
	for _, root := range functions.Roots {
		visit(root)
	}
	return result
}

// IsValidatorEntryPoint identifies Terraform attribute and config validators.
func IsValidatorEntryPoint(fn *types.Func) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return false
	}
	for index := 0; index < sig.Params().Len(); index++ {
		named := NamedType(sig.Params().At(index).Type())
		if named == nil || named.Obj().Pkg() == nil {
			continue
		}
		path := named.Obj().Pkg().Path()
		name := named.Obj().Name()
		if path == "github.com/hashicorp/terraform-plugin-framework/schema/validator" && strings.HasSuffix(name, "Request") {
			return true
		}
		if (path == "github.com/hashicorp/terraform-plugin-framework/resource" ||
			path == "github.com/hashicorp/terraform-plugin-framework/datasource" ||
			path == "github.com/hashicorp/terraform-plugin-framework/provider") && name == "ValidateConfigRequest" {
			return true
		}
	}
	return false
}

// Predicate resolves a Terraform attr.Value IsUnknown or IsNull call.
func Predicate(pass *analysis.Pass, call *ast.CallExpr) (PredicateKind, ValueRef, bool) {
	if len(call.Args) != 0 {
		return 0, ValueRef{}, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return 0, ValueRef{}, false
	}
	var kind PredicateKind
	switch selector.Sel.Name {
	case "IsUnknown":
		kind = PredicateUnknown
	case "IsNull":
		kind = PredicateNull
	default:
		return 0, ValueRef{}, false
	}
	if !implementsAttrValue(pass, pass.TypesInfo.TypeOf(selector.X)) {
		return 0, ValueRef{}, false
	}
	value, ok := Value(pass, selector.X)
	return kind, value, ok
}

// IsDiagnosticCall reports whether call invokes a framework diag.Diagnostics method.
func IsDiagnosticCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch selector.Sel.Name {
	case "AddError", "AddWarning", "AddAttributeError", "AddAttributeWarning":
	default:
		return false
	}
	return diagnosticsMethod(pass, selector)
}

// IsDiagnosticsHasError reports whether call invokes diag.Diagnostics.HasError.
func IsDiagnosticsHasError(pass *analysis.Pass, call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "HasError" && diagnosticsMethod(pass, selector)
}

func diagnosticsMethod(pass *analysis.Pass, selector *ast.SelectorExpr) bool {
	selection := pass.TypesInfo.Selections[selector]
	if selection == nil {
		return false
	}
	method, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	signature, ok := method.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return false
	}
	named := NamedType(signature.Recv().Type())
	return named != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == diagPackage && named.Obj().Name() == "Diagnostics"
}

// Value returns the semantic root and selector path for an expression.
func Value(pass *analysis.Pass, expression ast.Expr) (ValueRef, bool) {
	switch expression := expression.(type) {
	case *ast.Ident:
		object := pass.TypesInfo.ObjectOf(expression)
		if object == nil {
			return ValueRef{}, false
		}
		return ValueRef{Key: ValueKey{Root: object}, Display: expression.Name}, true
	case *ast.SelectorExpr:
		root, ok := Value(pass, expression.X)
		if !ok {
			return ValueRef{}, false
		}
		root.Key.Path += "." + expression.Sel.Name
		root.Display += "." + expression.Sel.Name
		return root, true
	case *ast.IndexExpr:
		root, ok := Value(pass, expression.X)
		if !ok {
			return ValueRef{}, false
		}
		root.Key.Path += "[*]"
		root.Display += "[*]"
		return root, true
	case *ast.ParenExpr:
		return Value(pass, expression.X)
	default:
		return ValueRef{}, false
	}
}

// NamedType unwraps a pointer and returns its named type, if any.
func NamedType(value types.Type) *types.Named {
	if pointer, ok := value.(*types.Pointer); ok {
		value = pointer.Elem()
	}
	named, _ := value.(*types.Named)
	return named
}

// Inspect walks a node without descending into nested function literals.
func Inspect(node ast.Node, visit func(ast.Node)) {
	ast.Inspect(node, func(candidate ast.Node) bool {
		if candidate == nil {
			return true
		}
		if candidate != node {
			if _, ok := candidate.(*ast.FuncLit); ok {
				return false
			}
		}
		visit(candidate)
		return true
	})
}

// AnalyzeKnownFacts computes facts for direct if guards, returns, and continues.
// Unsupported statements are traversed conservatively without exporting facts
// from their bodies.
func AnalyzeKnownFacts(pass *analysis.Pass, body *ast.BlockStmt) KnownFacts {
	facts := KnownFacts{
		AtIf:   map[token.Pos]ValueSet{},
		AtCall: map[token.Pos]ValueSet{},
	}
	recordCalls := func(node ast.Node, known ValueSet) {
		if node == nil {
			return
		}
		Inspect(node, func(candidate ast.Node) {
			if call, ok := candidate.(*ast.CallExpr); ok {
				facts.AtCall[call.Pos()] = CloneValues(known)
			}
		})
	}

	var visitBlock func(*ast.BlockStmt, ValueSet) (ValueSet, bool)
	var visitIf func(*ast.IfStmt, ValueSet) (ValueSet, bool)
	visitIf = func(statement *ast.IfStmt, incoming ValueSet) (ValueSet, bool) {
		recordCalls(statement.Init, incoming)
		recordCalls(statement.Cond, incoming)

		trueKnown := CloneValues(incoming)
		AddValues(trueKnown, KnownValuesWhen(pass, statement.Cond, true))
		facts.AtIf[statement.Pos()] = CloneValues(trueKnown)
		trueKnown, trueContinues := visitBlock(statement.Body, trueKnown)

		falseKnown := CloneValues(incoming)
		AddValues(falseKnown, KnownValuesWhen(pass, statement.Cond, false))
		falseContinues := true
		switch alternative := statement.Else.(type) {
		case *ast.BlockStmt:
			falseKnown, falseContinues = visitBlock(alternative, falseKnown)
		case *ast.IfStmt:
			falseKnown, falseContinues = visitIf(alternative, falseKnown)
		}

		switch {
		case trueContinues && falseContinues:
			return IntersectValueSets(trueKnown, falseKnown), true
		case trueContinues:
			return trueKnown, true
		case falseContinues:
			return falseKnown, true
		default:
			return ValueSet{}, false
		}
	}
	visitBlock = func(block *ast.BlockStmt, incoming ValueSet) (ValueSet, bool) {
		known := CloneValues(incoming)
		for _, statement := range block.List {
			switch statement := statement.(type) {
			case *ast.IfStmt:
				var continues bool
				known, continues = visitIf(statement, known)
				if !continues {
					return known, false
				}
			case *ast.ForStmt:
				recordCalls(statement.Init, known)
				recordCalls(statement.Cond, known)
				recordCalls(statement.Post, known)
				_, _ = visitBlock(statement.Body, known)
			case *ast.RangeStmt:
				recordCalls(statement.X, known)
				_, _ = visitBlock(statement.Body, known)
			case *ast.BlockStmt:
				var continues bool
				known, continues = visitBlock(statement, known)
				if !continues {
					return known, false
				}
			case *ast.ReturnStmt:
				recordCalls(statement, known)
				return known, false
			case *ast.BranchStmt:
				recordCalls(statement, known)
				if statement.Tok == token.CONTINUE {
					return known, false
				}
			default:
				recordCalls(statement, known)
			}
		}
		return known, true
	}
	_, _ = visitBlock(body, ValueSet{})
	return facts
}

// KnownValuesWhen returns values proven not unknown when expression has result.
func KnownValuesWhen(pass *analysis.Pass, expression ast.Expr, result bool) []ValueRef {
	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return KnownValuesWhen(pass, expression.X, result)
	case *ast.BinaryExpr:
		if expression.Op != token.LAND && expression.Op != token.LOR {
			return nil
		}
		left := KnownValuesWhen(pass, expression.X, result)
		right := KnownValuesWhen(pass, expression.Y, result)
		if (expression.Op == token.LAND && result) || (expression.Op == token.LOR && !result) {
			return UniqueValues(append(left, right...))
		}
		return IntersectValues(left, right)
	case *ast.UnaryExpr:
		if expression.Op == token.NOT {
			return KnownValuesWhen(pass, expression.X, !result)
		}
	case *ast.CallExpr:
		kind, value, ok := Predicate(pass, expression)
		if !ok {
			return nil
		}
		unknownOutcome := kind == PredicateUnknown
		if result != unknownOutcome {
			return []ValueRef{value}
		}
	}
	return nil
}

// CloneValues copies a semantic value set.
func CloneValues(values ValueSet) ValueSet {
	result := ValueSet{}
	for key, value := range values {
		result[key] = value
	}
	return result
}

// AddValues adds semantic values to a set.
func AddValues(destination ValueSet, values []ValueRef) {
	for _, value := range values {
		destination[value.Key] = value
	}
}

// IntersectValueSets intersects semantic value sets.
func IntersectValueSets(left, right ValueSet) ValueSet {
	result := ValueSet{}
	for key, value := range left {
		if _, ok := right[key]; ok {
			result[key] = value
		}
	}
	return result
}

// IntersectValues intersects semantic value lists.
func IntersectValues(left, right []ValueRef) []ValueRef {
	rightSet := ValueSet{}
	AddValues(rightSet, right)
	var result []ValueRef
	for _, value := range left {
		if _, ok := rightSet[value.Key]; ok {
			result = append(result, value)
		}
	}
	return UniqueValues(result)
}

// UniqueValues deduplicates semantic value identities.
func UniqueValues(values []ValueRef) []ValueRef {
	seen := ValueSet{}
	result := make([]ValueRef, 0, len(values))
	for _, value := range values {
		if value.Key.Root == nil {
			continue
		}
		if _, ok := seen[value.Key]; ok {
			continue
		}
		seen[value.Key] = value
		result = append(result, value)
	}
	return result
}

func implementsAttrValue(pass *analysis.Pass, value types.Type) bool {
	if value == nil {
		return false
	}
	attr := importedPackage(pass.Pkg, attrPackage, map[*types.Package]bool{})
	if attr == nil {
		return false
	}
	object := attr.Scope().Lookup("Value")
	if object == nil {
		return false
	}
	iface, ok := object.Type().Underlying().(*types.Interface)
	if !ok {
		return false
	}
	iface.Complete()
	return types.Implements(value, iface) || types.Implements(types.NewPointer(value), iface)
}

func importedPackage(current *types.Package, path string, seen map[*types.Package]bool) *types.Package {
	if current == nil || seen[current] {
		return nil
	}
	seen[current] = true
	if current.Path() == path {
		return current
	}
	for _, imported := range current.Imports() {
		if found := importedPackage(imported, path, seen); found != nil {
			return found
		}
	}
	return nil
}
