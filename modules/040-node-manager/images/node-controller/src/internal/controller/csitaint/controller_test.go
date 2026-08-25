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

package csitaint

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func reconcileNode(t *testing.T, objs ...client.Object) *corev1.Node {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, storagev1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	r := &Reconciler{}
	r.InjectClient(c)
	_, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "worker"}})
	require.NoError(t, err)

	node := &corev1.Node{}
	require.NoError(t, c.Get(t.Context(), types.NamespacedName{Name: "worker"}, node))
	return node
}

func taintedNode(taints ...corev1.Taint) *corev1.Node {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}
	node.Spec.Taints = taints
	return node
}

func csiNode(drivers ...string) *storagev1.CSINode {
	csi := &storagev1.CSINode{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}
	for _, name := range drivers {
		csi.Spec.Drivers = append(csi.Spec.Drivers, storagev1.CSINodeDriver{Name: name, NodeID: "worker"})
	}
	return csi
}

func TestReconcile_RemovesTheTaintOnceADriverRegisters(t *testing.T) {
	csi := corev1.Taint{Key: csiNotBootstrappedTaintKey, Effect: corev1.TaintEffectNoSchedule}
	other := corev1.Taint{Key: "dedicated", Value: "system", Effect: corev1.TaintEffectNoSchedule}

	t.Run("driver registered: the csi taint goes, the others stay", func(t *testing.T) {
		node := reconcileNode(t, taintedNode(other, csi), csiNode("ebs.csi.aws.com"))
		require.Equal(t, []corev1.Taint{other}, node.Spec.Taints)
	})

	t.Run("csi taint is the only one", func(t *testing.T) {
		node := reconcileNode(t, taintedNode(csi), csiNode("ebs.csi.aws.com"))
		require.Empty(t, node.Spec.Taints)
	})

	t.Run("CSINode registers no driver yet: the taint stays", func(t *testing.T) {
		node := reconcileNode(t, taintedNode(csi), csiNode())
		require.Equal(t, []corev1.Taint{csi}, node.Spec.Taints)
	})

	t.Run("no CSINode at all: the taint stays", func(t *testing.T) {
		node := reconcileNode(t, taintedNode(csi))
		require.Equal(t, []corev1.Taint{csi}, node.Spec.Taints)
	})
}

func TestHasCSITaint(t *testing.T) {
	tests := []struct {
		name   string
		taints []corev1.Taint
		expHas bool
	}{
		{name: "no taints", taints: nil, expHas: false},
		{name: "only other taints", taints: []corev1.Taint{{Key: "somekey"}}, expHas: false},
		{name: "csi taint present", taints: []corev1.Taint{{Key: "somekey"}, {Key: csiNotBootstrappedTaintKey}}, expHas: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node := &corev1.Node{}
			node.Spec.Taints = tc.taints
			require.Equal(t, tc.expHas, hasCSITaint(node))
		})
	}
}
