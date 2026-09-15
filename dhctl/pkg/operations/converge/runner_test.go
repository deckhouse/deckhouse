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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	convergecontext "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/context"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
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

		for _, stmt := range block.List {
			guard, ok := stmt.(*ast.IfStmt)
			if !ok || guard.Init == nil || !strings.Contains(text(guard.Init), "CleanupConvergeUser") {
				continue
			}

			guards++

			require.Len(t, guard.Body.List, 1, "the guard on an unfinished cleanup must do nothing but end the converge")
			require.Equal(t, "return nil", text(guard.Body.List[0]),
				"a cleanup that could not finish must end the converge without deleting the state")

			// The last statement of the very block the cleanup guards: anywhere else the
			// deletion runs on paths the cleanup never ran on, and a call whose error is
			// dropped reports a converge that kept its state as a converge that cleaned up.
			require.Equal(t, "return ctx.DeleteConvergeStateIfUserGone()", text(block.List[len(block.List)-1]),
				"the block that removes the converge user must end by returning the state deletion")
		}

		return true
	})

	require.Equal(t, 1, guards, "%s must remove the converge user exactly once", runnerFile)
}

// Commander stops a converge at a phase boundary, and the node phase then returns without
// touching a thing. Read as a finished run, it takes the resume marker of an interrupted
// master scaling down with it.
func TestAStoppedNodePhaseIsNotAConvergedRun(t *testing.T) {
	t.Parallel()

	phaseContext := phases.NewDefaultPhasedExecutionContext(
		phases.OperationConverge,
		func(phases.OnPhaseFuncData[phases.DefaultContextType]) error {
			return phases.ErrStopOperationCondition
		},
		nil,
	)

	convergeCtx := convergecontext.
		NewContext(t.Context(), convergecontext.Params{Cache: cache.NewTestCache()}).
		WithPhaseContext(phaseContext)

	converged, err := newRunner(nil, nil).convergeTerraNodes(convergeCtx, nil, nil)

	require.NoError(t, err)
	require.False(t, converged, "a phase stopped at its boundary reports the nodes as converged")
}

// A converge interrupted before this path was removed left the NodeUser behind, and
// bashible keeps provisioning the passwordless sudoer it describes on every master,
// including masters that join later. Nothing expires that account.
func TestTheNodeUserLeftByAnOlderDhctlIsDeleted(t *testing.T) {
	t.Parallel()

	kubeCl := client.NewFakeKubernetesClientWithListGVR(
		map[schema.GroupVersionResource]string{v1.NodeUserGVR: v1.NodeUserList},
	)
	kubeGetter := kubernetes.NewSimpleKubeClientGetter(kubeCl)

	_, err := kubeCl.Dynamic().Resource(v1.NodeUserGVR).Create(t.Context(), &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeUser",
		"metadata":   map[string]any{"name": global.ConvergeNodeUserName},
	}}, metav1.CreateOptions{})
	require.NoError(t, err)

	require.NoError(t, deleteLegacyNodeUser(t.Context(), kubeGetter))

	_, err = kubeCl.Dynamic().Resource(v1.NodeUserGVR).Get(t.Context(), global.ConvergeNodeUserName, metav1.GetOptions{})
	require.True(t, apierrors.IsNotFound(err), "the NodeUser of an older dhctl is still there: %v", err)

	// Every cluster but the interrupted one has none, and that must cost a converge nothing.
	require.NoError(t, deleteLegacyNodeUser(t.Context(), kubeGetter))
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
