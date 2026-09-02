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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	klient "github.com/flant/kube-client/client"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

var nodeGroupGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}

func nodeGroup(name, systemType string) *unstructured.Unstructured {
	spec := map[string]any{"nodeType": "CloudPermanent"}
	if systemType != "" {
		spec["systemType"] = systemType
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata":   map[string]any{"name": name},
		"spec":       spec,
	}}
}

func gateClient(t *testing.T, nodeGroups ...*unstructured.Unstructured) (*kubernetes.SimpleKubeClientGetter, *dynamicfake.FakeDynamicClient) {
	t.Helper()

	kubeCl := client.NewFakeKubernetesClientWithListGVR(
		map[schema.GroupVersionResource]string{nodeGroupGVR: "NodeGroupList"},
	)

	fakeClient, ok := kubeCl.KubeClient.(*klient.Client)
	require.True(t, ok, "fake kube client is not backed by kube-client")

	dyn, ok := fakeClient.Dynamic().(*dynamicfake.FakeDynamicClient)
	require.True(t, ok, "fake kube client is not backed by a fake dynamic client")

	for _, ng := range nodeGroups {
		_, err := dyn.Resource(nodeGroupGVR).Create(t.Context(), ng, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	return kubernetes.NewSimpleKubeClientGetter(kubeCl), dyn
}

// The control plane decides whether converge may use SSH at all: an immutable master
// answers no sshd, and the node-user switch that every other cluster needs would hang
// on a port nothing listens on.
func TestMasterGroupIsImmutable(t *testing.T) {
	t.Run("a classic control plane keeps the SSH path", func(t *testing.T) {
		kubeGetter, _ := gateClient(t, nodeGroup("master", ""), nodeGroup("worker", "Immutable"))

		immutableMasters, err := masterGroupIsImmutable(t.Context(), kubeGetter)

		require.NoError(t, err)
		require.False(t, immutableMasters)
	})

	t.Run("an immutable control plane drops it", func(t *testing.T) {
		kubeGetter, _ := gateClient(t, nodeGroup("master", "Immutable"), nodeGroup("worker", ""))

		immutableMasters, err := masterGroupIsImmutable(t.Context(), kubeGetter)

		require.NoError(t, err)
		require.True(t, immutableMasters)
	})

	// A cluster whose master group was never written keeps the path every other
	// cluster needs.
	t.Run("no master group at all", func(t *testing.T) {
		kubeGetter, _ := gateClient(t, nodeGroup("worker", "Immutable"))

		immutableMasters, err := masterGroupIsImmutable(t.Context(), kubeGetter)

		require.NoError(t, err)
		require.False(t, immutableMasters)
	})

	// Converge restarts the apiservers it reads through. One 503 answered "classic",
	// and the very next reader of the same object answered "immutable".
	t.Run("a transient list failure is retried", func(t *testing.T) {
		kubeGetter, dyn := gateClient(t, nodeGroup("master", "Immutable"))

		failed := false
		dyn.PrependReactor("list", "nodegroups", func(k8stesting.Action) (bool, runtime.Object, error) {
			if failed {
				return false, nil, nil
			}
			failed = true

			return true, nil, apierrors.NewInternalError(fmt.Errorf("apiserver is restarting"))
		})

		immutableMasters, err := masterGroupIsImmutable(t.Context(), kubeGetter)

		require.True(t, failed, "the transient failure was never served")
		require.NoError(t, err)
		require.True(t, immutableMasters)
	})

	// Guessing "classic" here creates a NodeUser and waits for bashible on machines
	// that run none, so an unreadable list stops the converge instead.
	t.Run("a list that keeps failing stops the converge", func(t *testing.T) {
		// Collapse the 600-attempt budget so the persistent failure is observable now.
		wasInTestEnvironment := retry.InTestEnvironment
		retry.InTestEnvironment = true
		t.Cleanup(func() { retry.InTestEnvironment = wasInTestEnvironment })

		kubeGetter, dyn := gateClient(t, nodeGroup("master", "Immutable"))

		dyn.PrependReactor("list", "nodegroups", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(
				nodeGroupGVR.GroupResource(), "", fmt.Errorf("converge may not list nodegroups"))
		})

		_, err := masterGroupIsImmutable(t.Context(), kubeGetter)

		require.ErrorContains(t, err, "converge may not list nodegroups")
	})
}

// A NodeGroup that has no infrastructure state is created straight from the runner, and
// an immutable group needs the same per-node document there as the group the controller
// converges: seeded with the group-wide bashible bundle its machines never register.
func TestNodeGroupsWithoutStateGetTheImmutablePayloadBuilder(t *testing.T) {
	t.Parallel()

	const file = "runner.go"

	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
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
			"%s creates the NodeGroups that have no state with another payload builder than the one the controller uses: an immutable group added after bootstrap is then seeded with the bashible bundle its machines cannot run", file)

		return true
	})

	require.Equal(t, 1, calls, "the call that creates the NodeGroups without state must exist in %s", file)
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
