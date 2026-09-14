// Package validatorshape enforces the supported control-flow shapes for
// Terraform validators and their local helpers.
package validatorshape

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"

	"github.com/doitintl/terraform-provider-doit/tools/linters/internal/validatoranalysis"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer enforces the repository's canonical unknown-safe validator shape.
var Analyzer = &analysis.Analyzer{
	Name:       "validatorshape",
	Doc:        "Enforces canonical unknown and null handling in Terraform validators.",
	Run:        run,
	ResultType: reflect.TypeOf((*Result)(nil)),
	Requires:   []*analysis.Analyzer{inspect.Analyzer},
}

// Result describes validator functions whose shape can be consumed safely by
// semantic validator analyzers.
type Result struct {
	Functions map[*types.Func]FunctionFacts
}

// FunctionFacts are the verified facts for one validator or helper.
type FunctionFacts struct {
	Declaration *ast.FuncDecl
	Conforms    bool
	Guards      map[token.Pos][]validatoranalysis.ValueRef
	Trackers    map[token.Pos]TrackerFact
	KnownAtIf   map[token.Pos]validatoranalysis.ValueSet
	KnownAtCall map[token.Pos]validatoranalysis.ValueSet
}

// TrackerFact associates a canonical collection uncertainty tracker with the
// Terraform values whose unknown state updates it.
type TrackerFact struct {
	Tracker validatoranalysis.ValueRef
	Values  []validatoranalysis.ValueRef
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	functions := validatoranalysis.DiscoverFunctions(pass, insp)
	reachable := validatoranalysis.ReachableFunctions(pass, functions)
	result := &Result{Functions: map[*types.Func]FunctionFacts{}}

	for function, declaration := range reachable {
		facts := analyzeFunction(pass, declaration)
		result.Functions[function] = facts
	}

	return result, nil
}

func analyzeFunction(pass *analysis.Pass, declaration *ast.FuncDecl) FunctionFacts {
	known := validatoranalysis.AnalyzeKnownFacts(pass, declaration.Body)
	facts := FunctionFacts{
		Declaration: declaration,
		Conforms:    true,
		Guards:      map[token.Pos][]validatoranalysis.ValueRef{},
		Trackers:    map[token.Pos]TrackerFact{},
		KnownAtIf:   known.AtIf,
		KnownAtCall: known.AtCall,
	}
	parents := parentMap(declaration.Body)
	reported := map[token.Pos]bool{}

	ast.Inspect(declaration.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		kind, value, ok := validatoranalysis.Predicate(pass, call)
		if !ok {
			return true
		}
		fail := func(message string) {
			facts.Conforms = false
			if !reported[call.Pos()] {
				reported[call.Pos()] = true
				pass.Reportf(call.Pos(), "%s", message)
			}
		}

		if hasAncestor[*ast.FuncLit](call, declaration.Body, parents) {
			fail("Terraform value predicates in validator closures are unsupported; move the check into a self-guarding helper")
			return true
		}
		if hasPredicateAliasAncestor(call, declaration.Body, parents) {
			if kind == validatoranalysis.PredicateNull && valueIsKnown(facts.KnownAtCall[call.Pos()], value) {
				return true
			}
			fail("do not store IsUnknown or IsNull results; use the predicate directly in a canonical validator guard")
			return true
		}

		ifStatement := containingIfCondition(call, declaration.Body, parents)
		if ifStatement == nil {
			if kind == validatoranalysis.PredicateNull && valueIsKnown(facts.KnownAtCall[call.Pos()], value) && hasSwitchAncestor(call, declaration.Body, parents) {
				return true
			}
			fail("Terraform value predicate uses unsupported validator control flow; use a canonical if guard")
			return true
		}
		positive := predicatePositive(call, ifStatement.Cond, parents)
		if kind == validatoranalysis.PredicateNull && positive &&
			!valueIsKnown(facts.KnownAtCall[call.Pos()], value) &&
			hasUnprotectedBusinessUse(pass, ifStatement.Cond, value, parents) {
			fail("IsNull cannot share a diagnostic condition with business logic until the value is known")
			return true
		}
		if kind != validatoranalysis.PredicateUnknown || !positive {
			return true
		}

		values, canonical := canonicalDeferralCondition(pass, ifStatement.Cond)
		if !canonical {
			fail("IsUnknown must be used in a direct ||-joined deferral guard without business conditions")
			return true
		}
		if canonicalExitBody(ifStatement.Body) {
			facts.Guards[ifStatement.Pos()] = values
			return true
		}
		if tracker, ok := canonicalTrackerBody(pass, ifStatement.Body); ok {
			facts.Trackers[ifStatement.Pos()] = TrackerFact{Tracker: tracker, Values: values}
			return true
		}
		fail("positive IsUnknown checks must return, continue, or record uncertainty and continue")
		return true
	})

	return facts
}

func hasUnprotectedBusinessUse(pass *analysis.Pass, condition ast.Expr, value validatoranalysis.ValueRef, parents map[ast.Node]ast.Node) bool {
	unsafe := false
	ast.Inspect(condition, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if _, _, predicate := validatoranalysis.Predicate(pass, call); predicate {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		receiver, ok := validatoranalysis.Value(pass, selector.X)
		if !ok || receiver.Key != value.Key {
			return true
		}
		if !guardedByNegativeUnknown(pass, call, condition, value, parents) {
			unsafe = true
		}
		return true
	})
	return unsafe
}

func guardedByNegativeUnknown(pass *analysis.Pass, node ast.Node, condition ast.Expr, value validatoranalysis.ValueRef, parents map[ast.Node]ast.Node) bool {
	for current := node; current != nil && current != condition; current = parents[current] {
		binary, ok := parents[current].(*ast.BinaryExpr)
		if !ok {
			continue
		}
		if binary.Op == token.LOR {
			return false
		}
		if binary.Op == token.LAND && containsNegativeUnknown(pass, binary, value) {
			return true
		}
	}
	return false
}

func containsNegativeUnknown(pass *analysis.Pass, expression ast.Expr, value validatoranalysis.ValueRef) bool {
	found := false
	var visit func(ast.Expr, bool)
	visit = func(expression ast.Expr, positive bool) {
		switch expression := expression.(type) {
		case *ast.ParenExpr:
			visit(expression.X, positive)
		case *ast.UnaryExpr:
			if expression.Op == token.NOT {
				visit(expression.X, !positive)
			}
		case *ast.BinaryExpr:
			if expression.Op == token.LAND {
				visit(expression.X, positive)
				visit(expression.Y, positive)
			}
		case *ast.CallExpr:
			kind, candidate, ok := validatoranalysis.Predicate(pass, expression)
			if ok && kind == validatoranalysis.PredicateUnknown && !positive && candidate.Key == value.Key {
				found = true
			}
		}
	}
	visit(expression, true)
	return found
}

func canonicalDeferralCondition(pass *analysis.Pass, expression ast.Expr) ([]validatoranalysis.ValueRef, bool) {
	switch expression := expression.(type) {
	case *ast.ParenExpr:
		return canonicalDeferralCondition(pass, expression.X)
	case *ast.BinaryExpr:
		if expression.Op != token.LOR {
			return nil, false
		}
		left, leftOK := canonicalDeferralCondition(pass, expression.X)
		right, rightOK := canonicalDeferralCondition(pass, expression.Y)
		return append(left, right...), leftOK && rightOK
	case *ast.CallExpr:
		if kind, value, ok := validatoranalysis.Predicate(pass, expression); ok {
			if kind == validatoranalysis.PredicateUnknown {
				return []validatoranalysis.ValueRef{value}, true
			}
			return nil, true
		}
		return nil, validatoranalysis.IsDiagnosticsHasError(pass, expression)
	default:
		return nil, false
	}
}

func valueIsKnown(known validatoranalysis.ValueSet, value validatoranalysis.ValueRef) bool {
	_, ok := known[value.Key]
	return ok
}

func hasSwitchAncestor(node ast.Node, root ast.Node, parents map[ast.Node]ast.Node) bool {
	for current := node; current != nil && current != root; current = parents[current] {
		switch current.(type) {
		case *ast.SwitchStmt, *ast.TypeSwitchStmt:
			return true
		}
	}
	return false
}

func canonicalExitBody(body *ast.BlockStmt) bool {
	if body == nil || len(body.List) != 1 {
		return false
	}
	switch statement := body.List[0].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return statement.Tok == token.CONTINUE
	default:
		return false
	}
}

func canonicalTrackerBody(pass *analysis.Pass, body *ast.BlockStmt) (validatoranalysis.ValueRef, bool) {
	if body == nil || len(body.List) != 2 {
		return validatoranalysis.ValueRef{}, false
	}
	branch, ok := body.List[1].(*ast.BranchStmt)
	if !ok || branch.Tok != token.CONTINUE {
		return validatoranalysis.ValueRef{}, false
	}
	switch statement := body.List[0].(type) {
	case *ast.IncDecStmt:
		if statement.Tok != token.INC {
			return validatoranalysis.ValueRef{}, false
		}
		return validatoranalysis.Value(pass, statement.X)
	case *ast.AssignStmt:
		if len(statement.Lhs) != 1 || len(statement.Rhs) != 1 {
			return validatoranalysis.ValueRef{}, false
		}
		literal, ok := statement.Rhs[0].(*ast.Ident)
		if !ok || literal.Name != "true" {
			return validatoranalysis.ValueRef{}, false
		}
		return validatoranalysis.Value(pass, statement.Lhs[0])
	default:
		return validatoranalysis.ValueRef{}, false
	}
}

func parentMap(root ast.Node) map[ast.Node]ast.Node {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

func containingIfCondition(node ast.Node, root ast.Node, parents map[ast.Node]ast.Node) *ast.IfStmt {
	for current := node; current != nil && current != root; current = parents[current] {
		if statement, ok := current.(*ast.IfStmt); ok && contains(statement.Cond, node) {
			return statement
		}
	}
	return nil
}

func hasPredicateAliasAncestor(node ast.Node, root ast.Node, parents map[ast.Node]ast.Node) bool {
	for current := node; current != nil && current != root; current = parents[current] {
		switch current.(type) {
		case *ast.AssignStmt, *ast.ValueSpec:
			return true
		case *ast.IfStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
			return false
		}
	}
	return false
}

func predicatePositive(node ast.Node, condition ast.Expr, parents map[ast.Node]ast.Node) bool {
	positive := true
	for current := node; current != nil && current != condition; current = parents[current] {
		if unary, ok := parents[current].(*ast.UnaryExpr); ok && unary.Op == token.NOT {
			positive = !positive
		}
	}
	return positive
}

func contains(root, target ast.Node) bool {
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		if node == target {
			found = true
			return false
		}
		return !found
	})
	return found
}

func hasAncestor[T ast.Node](node ast.Node, root ast.Node, parents map[ast.Node]ast.Node) bool {
	for current := node; current != nil && current != root; current = parents[current] {
		if _, ok := current.(T); ok {
			return true
		}
	}
	return false
}
