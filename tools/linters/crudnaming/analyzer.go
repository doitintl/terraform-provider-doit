// Package crudnaming enforces consistent variable naming in CRUD methods:
//   - Create and Update methods should use `plan` (data from Terraform plan)
//   - Read and Delete methods should use `state` (data from Terraform state)
//
// GEMINI.md §6 (Resource Code Conventions):
//
//	Use `plan` in Create and Update methods (data from Terraform plan)
//	Use `state` in Read and Delete methods (data from Terraform state)
package crudnaming

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer is the go/analysis Analyzer for crudnaming.
var Analyzer = &analysis.Analyzer{
	Name:     "crudnaming",
	Doc:      "Enforces `plan` in Create/Update and `state` in Read/Delete (GEMINI.md §6).",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

// methodRules maps method names to (expected variable name, wrong variable name).
var methodRules = map[string]struct {
	expected string
	wrong    string
}{
	"Create": {expected: "plan", wrong: "state"},
	"Update": {expected: "plan", wrong: "state"},
	"Read":   {expected: "state", wrong: "plan"},
	"Delete": {expected: "state", wrong: "plan"},
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{(*ast.FuncDecl)(nil)}
	insp.Preorder(nodeFilter, func(n ast.Node) {
		fn := n.(*ast.FuncDecl)
		if fn.Name == nil || fn.Body == nil {
			return
		}
		// Must be a method (has receiver).
		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			return
		}

		rule, ok := methodRules[fn.Name.Name]
		if !ok {
			return
		}

		// Walk the method body looking for `var <wrong> <model>` or
		// `<wrong> := ...` where the variable name matches the wrong convention.
		for _, stmt := range fn.Body.List {
			// Pattern 1: var state myResourceModel (in Create/Update)
			if ds, ok := stmt.(*ast.DeclStmt); ok {
				if gd, ok := ds.Decl.(*ast.GenDecl); ok {
					for _, spec := range gd.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range vs.Names {
								if name.Name == rule.wrong {
									pass.Reportf(name.Pos(),
										"%s method should use %q instead of %q for the resource model variable (GEMINI.md §6)",
										fn.Name.Name, rule.expected, rule.wrong)
								}
							}
						}
					}
				}
			}
		}
	})

	return nil, nil
}
