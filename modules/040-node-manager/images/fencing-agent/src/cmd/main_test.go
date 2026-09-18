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

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"strings"
	"testing"
)

const wiringMoved = "the wiring this test reads has moved"

func TestProcessStartIsTakenBeforeIdentityAndProfile(t *testing.T) {
	run := parseRun(t)
	start := processStart(t, run)

	for _, step := range []struct {
		name    string
		timeout string
		match   func(call *ast.CallExpr) bool
	}{
		{
			name:    "resolveIdentity",
			timeout: "resolveIdentityTimeout",
			match: func(call *ast.CallExpr) bool {
				ident, ok := call.Fun.(*ast.Ident)

				return ok && ident.Name == "resolveIdentity"
			},
		},
		{
			name:    "profile.Load",
			timeout: "profileLoadTimeout",
			match:   func(call *ast.CallExpr) bool { return isSelector(call.Fun, "profile", "Load") },
		},
	} {
		stepCalls := calls(run, step.match)
		if len(stepCalls) == 0 {
			t.Fatalf("found no %s call in run: %s", step.name, wiringMoved)
		}

		for _, call := range stepCalls {
			if call.Pos() < start.Pos() {
				t.Errorf("startedAt := time.Now() comes after %s, want it before: the start of this life would move by up to %s",
					step.name, step.timeout)
			}
		}
	}

	checkStartReachesTheAgent(t, run)
	checkSetLoggerPrecedesControllerRuntime(t, run)
}

func parseRun(t *testing.T) *ast.FuncDecl {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "run" && fn.Recv == nil && fn.Body != nil {
			return fn
		}
	}

	t.Fatalf("run not found in main.go: %s", wiringMoved)

	return nil
}

func processStart(t *testing.T, run *ast.FuncDecl) *ast.AssignStmt {
	t.Helper()

	const want = "want exactly one write, startedAt := time.Now(), before resolveIdentity and profile.Load"

	var starts []*ast.AssignStmt

	rejected := 0

	ast.Inspect(run.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if !slices.ContainsFunc(node.Lhs, func(lhs ast.Expr) bool { return isIdent(lhs, "startedAt") }) {
				return true
			}

			if isProcessStart(node) {
				starts = append(starts, node)
			} else {
				rejected++

				t.Errorf("run writes startedAt in %s, %s", assignString(node), want)
			}
		case *ast.ValueSpec:
			if slices.ContainsFunc(node.Names, func(name *ast.Ident) bool { return name.Name == "startedAt" }) {
				rejected++

				t.Errorf("run declares startedAt with var, %s", want)
			}
		}

		return true
	})

	switch {
	case len(starts) == 1:
		return starts[0]
	case len(starts) > 1:
		t.Fatalf("found %d startedAt := time.Now() in run, %s", len(starts), want)
	case rejected > 0:
		t.FailNow()
	default:
		t.Fatalf("found no startedAt := time.Now() in run: %s", wiringMoved)
	}

	return nil
}

func checkStartReachesTheAgent(t *testing.T, run *ast.FuncDecl) {
	t.Helper()

	type handOver struct {
		node  ast.Node
		value ast.Expr
	}

	var handOvers []handOver

	ast.Inspect(run.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "StartedAt" {
					write := handOver{node: node}
					if len(node.Lhs) == len(node.Rhs) {
						write.value = node.Rhs[i]
					}

					handOvers = append(handOvers, write)
				}
			}
		case *ast.CompositeLit:
			if !isSelector(node.Type, "agent", "Deps") {
				return true
			}

			for _, elt := range node.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok && isIdent(kv.Key, "StartedAt") {
					handOvers = append(handOvers, handOver{node: kv, value: kv.Value})
				}
			}
		}

		return true
	})

	switch len(handOvers) {
	case 0:
		t.Fatalf("found no deps.StartedAt = startedAt in run: %s", wiringMoved)
	case 1:
	default:
		t.Errorf("run writes Deps.StartedAt %d times, want exactly one deps.StartedAt = startedAt: another write can replace the process start",
			len(handOvers))

		return
	}

	write := handOvers[0]

	if write.value == nil {
		t.Errorf("run writes Deps.StartedAt in a multi-value assignment, want deps.StartedAt = startedAt")
	} else if !isIdent(write.value, "startedAt") {
		t.Errorf("run sets Deps.StartedAt to %s, want startedAt, the process start", types.ExprString(write.value))
	}

	agentNew := calls(run, func(call *ast.CallExpr) bool { return isSelector(call.Fun, "agent", "New") })
	if len(agentNew) != 1 {
		t.Fatalf("found %d agent.New calls in run, want exactly one: %s", len(agentNew), wiringMoved)
	}

	if write.node.Pos() > agentNew[0].End() {
		t.Error("deps.StartedAt = startedAt comes after agent.New, want it before: agent.New takes Deps by value, " +
			"so the agent would start without the process start")
	}
}

var controllerRuntimeUsers = []string{"kubeclient", "fencingstate", "profile", "agent"}

func checkSetLoggerPrecedesControllerRuntime(t *testing.T, run *ast.FuncDecl) {
	t.Helper()

	last := len(controllerRuntimeUsers) - 1
	packages := strings.Join(controllerRuntimeUsers[:last], ", ") + " or " + controllerRuntimeUsers[last]
	users := "the first call into " + packages

	idx := slices.IndexFunc(run.Body.List, isSetLogger)
	if idx < 0 {
		t.Errorf("run has no ctrllog.SetLogger(...) statement of its own, want one before %s: "+
			"controller-runtime drops what it logs before SetLogger", users)

		return
	}

	setLogger := run.Body.List[idx]

	first := calls(run, func(call *ast.CallExpr) bool {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}

		pkg, ok := sel.X.(*ast.Ident)

		return ok && slices.Contains(controllerRuntimeUsers, pkg.Name)
	})
	if len(first) == 0 {
		t.Fatalf("found no call into %s in run: %s", packages, wiringMoved)
	}

	if first[0].Pos() < setLogger.Pos() {
		t.Errorf("ctrllog.SetLogger(...) comes after %s, want it before %s: controller-runtime drops what it logs before SetLogger",
			types.ExprString(first[0].Fun), users)
	}
}

func isProcessStart(stmt *ast.AssignStmt) bool {
	if stmt.Tok != token.DEFINE || len(stmt.Lhs) != 1 || len(stmt.Rhs) != 1 || !isIdent(stmt.Lhs[0], "startedAt") {
		return false
	}

	call, ok := stmt.Rhs[0].(*ast.CallExpr)

	return ok && len(call.Args) == 0 && isSelector(call.Fun, "time", "Now")
}

func calls(node ast.Node, match func(call *ast.CallExpr) bool) []*ast.CallExpr {
	var found []*ast.CallExpr

	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && match(call) {
			found = append(found, call)
		}

		return true
	})

	slices.SortFunc(found, func(a, b *ast.CallExpr) int { return int(a.Pos() - b.Pos()) })

	return found
}

func isSetLogger(stmt ast.Stmt) bool {
	expr, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false
	}

	call, ok := expr.X.(*ast.CallExpr)

	return ok && isSelector(call.Fun, "ctrllog", "SetLogger")
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)

	return ok && ident.Name == name
}

func isSelector(expr ast.Expr, receiver, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)

	return ok && sel.Sel.Name == name && isIdent(sel.X, receiver)
}

func assignString(stmt *ast.AssignStmt) string {
	return exprList(stmt.Lhs) + " " + stmt.Tok.String() + " " + exprList(stmt.Rhs)
}

func exprList(exprs []ast.Expr) string {
	parts := make([]string, 0, len(exprs))
	for _, expr := range exprs {
		parts = append(parts, types.ExprString(expr))
	}

	return strings.Join(parts, ", ")
}
