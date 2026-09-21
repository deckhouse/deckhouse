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
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
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

// The API server defaults kubelet and containerRuntime on every object it
// stores. A render that wrote them back empty would be defaulted again and
// patched again, once per pass for ever.
func TestAMutableDocumentLeavesTheAPIServersOwnDefaultsAlone(t *testing.T) {
	stored := &internalv1alpha1.NodeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "worker-0",
			Labels:          map[string]string{nodecommon.NodeGroupLabel: "worker", managedByLabel: managedByValue},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "v1", Kind: "Node", Name: "worker-0", UID: "uid-1"}},
		},
		Spec: internalv1alpha1.NodeSpec{
			SystemType:         internalv1alpha1.SystemTypeMutable,
			NodeName:           "worker-0",
			APIServerEndpoints: []string{"10.0.0.5:6443"},
			// What the CRD defaults on any object it stores, and what bashible
			// alone configures on such a node.
			Kubelet:          internalv1alpha1.Kubelet{MaxPods: 120, ContainerLogMaxSize: "50Mi", ContainerLogMaxFiles: 4},
			ContainerRuntime: internalv1alpha1.ContainerRuntime{MaxConcurrentDownloads: ptr.To(8), RegistryOwner: "nodelet", SandboxImage: "registry.k8s.io/pause:3.10"},
		},
	}

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, internalv1alpha1.AddToScheme(scheme))
	r := &Reconciler{}
	r.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(stored).Build()

	ng := &v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "worker"}, Spec: v1.NodeGroupSpec{SystemType: v1.SystemTypeMutable}}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-0", UID: "uid-1"}}
	p := newPass()
	p.mutable = &mutableInputsResult{inputs: mutableInputs{APIServerEndpoints: []string{"10.0.0.5:6443"}}}

	require.NoError(t, r.reconcileMutableNode(t.Context(), ng, node, logr.Discard(), p))

	after := &internalv1alpha1.NodeConfig{}
	require.NoError(t, r.Client.Get(t.Context(), types.NamespacedName{Name: "worker-0"}, after))
	require.Equal(t, stored.ResourceVersion, after.ResourceVersion, "a settled document must not be written again")
	require.Equal(t, stored.Spec.Kubelet, after.Spec.Kubelet)
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
