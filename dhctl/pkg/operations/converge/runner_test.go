// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package converge

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const runnerFile = "runner.go"

// A NodeGroup that has no infrastructure state is created straight from the runner, and
// an immutable group needs the same per-node document there as the group the controller
// converges: seeded with the group-wide bashible bundle its machines never register.
func TestNodeGroupsWithoutStateGetTheImmutablePayloadBuilder(t *testing.T) {
	t.Parallel()

	parsed, err := parser.ParseFile(token.NewFileSet(), runnerFile, nil, 0)
	require.NoError(t, err)

	var calls int
	ast.Inspect(parsed, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		fn, ok := call.Fun.(*ast.Ident)
		if !ok || fn.Name != "bootstrapNewNodeGroups" {
			return true
		}

		calls++
		require.Equalf(t, "controller.NewImmutablePayloadBuilder", calleeName(call.Args[len(call.Args)-1]),
			"%s creates the NodeGroups that have no state with another payload builder than the one the controller uses: an immutable group added after bootstrap is then seeded with the bashible bundle its machines cannot run", runnerFile)

		return true
	})

	require.Equal(t, 1, calls, "the call that creates the NodeGroups without state must exist in %s", runnerFile)
}

// Nothing but the runner deletes the converge state, and it holds both the phase a run may
// have to resume and the masters still carrying the converge user. A cleanup that could
// not finish has to leave that list behind for the next converge.
func TestConvergeDeletesTheStateOnlyAfterCleanupSucceeded(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile(runnerFile)
	require.NoError(t, err)

	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, runnerFile, source, 0)
	require.NoError(t, err)

	text := func(n ast.Node) string {
		return string(source[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset])
	}

	var guards int

	ast.Inspect(parsed, func(n ast.Node) bool {
		block, ok := n.(*ast.BlockStmt)
		if !ok {
			return true
		}

		for i, stmt := range block.List {
			guard, ok := stmt.(*ast.IfStmt)
			if !ok || guard.Init == nil || !strings.Contains(text(guard.Init), "CleanupConvergeUser") {
				continue
			}

			guards++

			require.Len(t, guard.Body.List, 1)
			require.Equal(t, "return nil", text(guard.Body.List[0]),
				"a cleanup that could not finish must end the converge without deleting the state")

			require.Greater(t, len(block.List), i+1, "nothing follows the converge user cleanup")
			require.Contains(t, text(block.List[i+1]), "DeleteConvergeState",
				"the converge state must be deleted right after a cleanup that succeeded")
		}

		return true
	})

	require.Equal(t, 1, guards, "%s must remove the converge user exactly once", runnerFile)
}

// calleeName is the package-qualified name an argument calls, and "" for an argument
// that calls nothing.
func calleeName(arg ast.Expr) string {
	call, ok := arg.(*ast.CallExpr)
	if !ok {
		return ""
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}

	return pkg.Name + "." + sel.Sel.Name
}
