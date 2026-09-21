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

package nodeconfig

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// bashible configures the rest of such a node, so the document carries nothing
// bashible applies: a field both wrote would be rewritten by each in turn.
func TestAMutableDocumentCarriesOnlyWhatBashibleDoesNotApply(t *testing.T) {
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0", UID: "uid-1"}}
	in := mutableInputs{
		APIServerEndpoints:         []string{"10.0.0.5:6443"},
		RegistryPackagesProxyToken: "dG9rZW4=",
	}

	got := newMutableNodeConfig(ng, node, in)

	want := internalv1alpha1.NodeSpec{
		SystemType:                          internalv1alpha1.SystemTypeMutable,
		NodeName:                            "worker-0",
		APIServerEndpoints:                  []string{"10.0.0.5:6443"},
		RegistryPackagesProxyAccessTokenB64: "dG9rZW4=",
	}
	require.Equal(t, want, got.Spec)
	require.Equal(t, managedByValue, got.Labels[managedByLabel], "the object must be ours")
	require.Len(t, got.OwnerReferences, 1, "the object must be owned by its Node")
}

// Left to the API default, the comparison "is the object up to date" would see a
// difference on every pass and rewrite every Engine node's object for ever.
func TestAnEngineDocumentNamesItsSystemType(t *testing.T) {
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}
	spec := renderSpec(ng, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}, clusterInputs{})
	require.Equal(t, internalv1alpha1.SystemTypeImmutable, spec.SystemType,
		"Immutable must be written explicitly, not left to the API default")
}

// A Mutable document carries the fields below and nothing else, on the wire as
// well as in Go. The API server fills in the defaults inside kubelet and
// containerRuntime whenever those keys are present at all, and the agent reports
// every spec field outside its own six as not applicable — every such node would
// be Degraded for ever over two blocks that were sent empty.
func TestAMutableDocumentSendsNoEmptyBlocks(t *testing.T) {
	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0", UID: "uid-1"}}
	in := mutableInputs{
		APIServerEndpoints:         []string{"10.0.0.5:6443"},
		RegistryPackagesProxyToken: "dG9rZW4=",
	}
	base := []string{"systemType", "nodeName", "apiServerEndpoints", "registryPackagesProxyAccessTokenB64"}

	keysOf := func(t *testing.T, in mutableInputs) ([]string, []byte) {
		t.Helper()

		raw, err := json.Marshal(newMutableNodeConfig(ng, node, in).Spec)
		require.NoError(t, err)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &fields))
		return slices.Collect(maps.Keys(fields)), raw
	}

	t.Run("no static pods", func(t *testing.T) {
		keys, raw := keysOf(t, in)
		require.ElementsMatch(t, base, keys, "on the wire: %s", raw)
	})

	t.Run("one static pod", func(t *testing.T) {
		withPod := in
		withPod.NodeStaticPodRequests = []*deckhousev1alpha1.NodeStaticPodRequest{{
			ObjectMeta: metav1.ObjectMeta{Name: "probe"},
			Spec:       deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: "apiVersion: v1\nkind: Pod\n"},
		}}
		keys, raw := keysOf(t, withPod)
		require.ElementsMatch(t, append(base, "staticPods"), keys, "on the wire: %s", raw)
	})
}

// spec.systemType is immutable on the object, so a node relabelled into a group
// of the other kind can never be patched: the object has to go first.
func TestAnObjectOfTheOtherSystemTypeIsRemoved(t *testing.T) {
	newReconciler := func(t *testing.T, stored *internalv1alpha1.NodeConfig) *Reconciler {
		t.Helper()

		scheme := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(scheme))
		require.NoError(t, internalv1alpha1.AddToScheme(scheme))

		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(stored).Build()
		r := &Reconciler{}
		r.Client = cl
		return r
	}
	gone := func(t *testing.T, r *Reconciler, name string) {
		t.Helper()

		err := r.Client.Get(t.Context(), types.NamespacedName{Name: name}, &internalv1alpha1.NodeConfig{})
		require.True(t, apierrors.IsNotFound(err), "the object must be removed so the next pass creates the right one, got %v", err)
	}

	t.Run("a Mutable object in the way of an Engine node", func(t *testing.T) {
		stored := &internalv1alpha1.NodeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "worker-0", Labels: map[string]string{managedByLabel: managedByValue}},
			Spec:       internalv1alpha1.NodeSpec{SystemType: internalv1alpha1.SystemTypeMutable, NodeName: "worker-0"},
		}
		r := newReconciler(t, stored)

		ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Spec: v1.NodeGroupSpec{SystemType: v1.SystemTypeImmutable}}
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}
		desired := &internalv1alpha1.NodeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
			Spec:       internalv1alpha1.NodeSpec{SystemType: internalv1alpha1.SystemTypeImmutable, NodeName: "worker-0"},
		}

		current, err := r.apply(t.Context(), ng, node, desired, logr.Discard(), newPass())
		require.NoError(t, err)
		require.Nil(t, current, "nothing may be decided about an object that has just been removed")
		gone(t, r, "worker-0")
	})

	t.Run("an Immutable object in the way of a bashible node", func(t *testing.T) {
		stored := &internalv1alpha1.NodeConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "worker-0", Labels: map[string]string{managedByLabel: managedByValue}},
			// Written before the field existed, which reads as Immutable.
			Spec: internalv1alpha1.NodeSpec{NodeName: "worker-0"},
		}
		r := newReconciler(t, stored)

		ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Spec: v1.NodeGroupSpec{SystemType: v1.SystemTypeMutable}}
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0"}}
		p := newPass()
		p.mutable = &mutableInputsResult{inputs: mutableInputs{APIServerEndpoints: []string{"10.0.0.5:6443"}}}

		require.NoError(t, r.reconcileMutableNode(t.Context(), ng, node, logr.Discard(), p))
		gone(t, r, "worker-0")
	})
}
