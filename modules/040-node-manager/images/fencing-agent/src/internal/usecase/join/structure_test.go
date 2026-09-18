/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package join

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	errgroupPath      = "golang.org/x/sync/errgroup"
	watchdogUsecase   = "fencing-agent/internal/usecase/watchdog"
	structureMovedMsg = "the code this test reads has moved"
)

func TestCandidateReadsUseAPlainErrgroup(t *testing.T) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, "join.go", nil, 0)
	if err != nil {
		t.Fatalf("parse join.go: %v", err)
	}

	pkg := importName(file, errgroupPath)
	if pkg == "" {
		t.Fatalf("join.go does not import %s: %s", errgroupPath, structureMovedMsg)
	}

	fn := methodDecl(file, "readCandidates")
	if fn == nil {
		t.Fatalf("readCandidates not found in join.go: %s", structureMovedMsg)
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && isPkgSelector(sel, pkg, "WithContext") {
			t.Errorf("readCandidates uses %s.WithContext, want a plain %s.Group: one failed read must not cancel the others", pkg, pkg)
		}

		return true
	})

	groups := plainGroupVars(fn, pkg)
	if len(groups) == 0 {
		t.Fatalf("readCandidates holds no plain group in a variable (`var g %[1]s.Group`, `%[1]s.Group{}`, `&%[1]s.Group{}` or `new(%[1]s.Group)`), "+
			"want the candidate reads to run in one", pkg)
	}

	reads := readCandidateRefs(fn)
	if len(reads) == 0 {
		t.Fatalf("readCandidates does not refer to readCandidate: %s", structureMovedMsg)
	}

	inGroup := goFuncLits(fn, groups)

	for _, read := range reads {
		if !inGroup[read.lit] {
			t.Errorf("readCandidate at %s is not directly in a function literal passed to Go on the plain group %v, "+
				"want every candidate read to run in that group", fset.Position(read.pos), groups)
		}
	}
}

func TestJoinDoesNotImportTheWatchdogUsecase(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}

	fset := token.NewFileSet()
	checked := 0

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		checked++

		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: unquote import %s: %v", name, spec.Path.Value, err)
			}

			if path == watchdogUsecase || strings.HasPrefix(path, watchdogUsecase+"/") {
				t.Errorf("%s imports %s, want the join usecase to stay out of the watchdog usecase", name, path)
			}
		}
	}

	if checked == 0 {
		t.Fatalf("no non-test Go file found in the package directory: %s", structureMovedMsg)
	}
}

func importName(file *ast.File, path string) string {
	for _, spec := range file.Imports {
		if got, err := strconv.Unquote(spec.Path.Value); err != nil || got != path {
			continue
		}

		if spec.Name != nil {
			return spec.Name.Name
		}

		return path[strings.LastIndex(path, "/")+1:]
	}

	return ""
}

func methodDecl(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv != nil && fn.Name.Name == name && fn.Body != nil {
			return fn
		}
	}

	return nil
}

func plainGroupVars(fn *ast.FuncDecl, pkg string) []string {
	var names []string

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.ValueSpec:
			if len(node.Values) == 0 {
				if isGroupType(node.Type, pkg) {
					for _, ident := range node.Names {
						names = append(names, ident.Name)
					}
				}

				return true
			}

			for i, value := range node.Values {
				if i < len(node.Names) && isPlainGroup(value, pkg) {
					names = append(names, node.Names[i].Name)
				}
			}
		case *ast.AssignStmt:
			if len(node.Lhs) != len(node.Rhs) {
				return true
			}

			for i, value := range node.Rhs {
				if ident, ok := node.Lhs[i].(*ast.Ident); ok && isPlainGroup(value, pkg) {
					names = append(names, ident.Name)
				}
			}
		}

		return true
	})

	return names
}

func isPlainGroup(expr ast.Expr, pkg string) bool {
	expr = ast.Unparen(expr)

	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		lit, ok := ast.Unparen(unary.X).(*ast.CompositeLit)

		return ok && len(lit.Elts) == 0 && isGroupType(lit.Type, pkg)
	}

	switch e := expr.(type) {
	case *ast.CompositeLit:
		return len(e.Elts) == 0 && isGroupType(e.Type, pkg)
	case *ast.CallExpr:
		ident, ok := e.Fun.(*ast.Ident)

		return ok && ident.Name == "new" && len(e.Args) == 1 && isGroupType(e.Args[0], pkg)
	}

	return false
}

func isGroupType(expr ast.Expr, pkg string) bool {
	sel, ok := ast.Unparen(expr).(*ast.SelectorExpr)

	return ok && isPkgSelector(sel, pkg, "Group")
}

func goFuncLits(fn *ast.FuncDecl, groups []string) map[*ast.FuncLit]bool {
	lits := make(map[*ast.FuncLit]bool)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Go" {
			return true
		}

		if ident, ok := sel.X.(*ast.Ident); !ok || !slices.Contains(groups, ident.Name) {
			return true
		}

		for _, arg := range call.Args {
			if lit, ok := ast.Unparen(arg).(*ast.FuncLit); ok {
				lits[lit] = true
			}
		}

		return true
	})

	return lits
}

type readRef struct {
	pos token.Pos
	lit *ast.FuncLit
}

func readCandidateRefs(fn *ast.FuncDecl) []readRef {
	var (
		refs  []readRef
		stack []ast.Node
		lits  []*ast.FuncLit
	)

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if n == nil {
			if _, ok := stack[len(stack)-1].(*ast.FuncLit); ok {
				lits = lits[:len(lits)-1]
			}

			stack = stack[:len(stack)-1]

			return true
		}

		stack = append(stack, n)

		switch node := n.(type) {
		case *ast.FuncLit:
			lits = append(lits, node)
		case *ast.SelectorExpr:
			if node.Sel.Name == "readCandidate" {
				ref := readRef{pos: node.Pos()}
				if len(lits) > 0 {
					ref.lit = lits[len(lits)-1]
				}

				refs = append(refs, ref)
			}
		}

		return true
	})

	return refs
}

func isPkgSelector(sel *ast.SelectorExpr, receiver, name string) bool {
	ident, ok := sel.X.(*ast.Ident)

	return ok && ident.Name == receiver && sel.Sel.Name == name
}
