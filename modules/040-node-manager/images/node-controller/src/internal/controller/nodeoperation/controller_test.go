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

package nodeoperation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

// The spec of a NodeOperation is CEL-immutable, so Failed is the end of it: an
// operation failed because a cache had not caught up can never be retried, and
// the node it names stays uninterrupted. The node is therefore read past the
// cache, like every other decision-before-write in this controller.
func TestANodeMissingFromTheCacheDoesNotKillTheOperation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "op",
			CreationTimestamp: metav1.Now(),
			Labels:            map[string]string{v1alpha1.NodeOperationNodeLabel: "worker"},
		},
		Spec: v1alpha1.NodeOperationSpec{
			Type:     v1alpha1.NodeOperationTypeReboot,
			NodeName: "worker",
			// The eviction is skipped so the pass reaches the hand-over without
			// writing to the node: what is under test is the read, not the drain.
			Drain: &v1alpha1.NodeOperationDrainSpec{Skip: true},
		},
	}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}}

	// The cache holds the operation but not the node it names — the state
	// between a node joining and the informer catching up with it.
	cached := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(op.DeepCopy()).
		WithStatusSubresource(&v1alpha1.NodeOperation{}).
		Build()
	live := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(op.DeepCopy(), node).
		Build()

	r := &Reconciler{apiReader: live}
	r.InjectClient(cached)

	ctx := context.Background()
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: op.Name}})
	require.NoError(t, err)

	fresh := &v1alpha1.NodeOperation{}
	require.NoError(t, cached.Get(ctx, types.NamespacedName{Name: op.Name}, fresh))
	require.NotEqual(t, v1alpha1.NodeOperationPhaseFailed, fresh.Status.Phase,
		"a node the cache has not seen yet must not fail an operation for good")
}
