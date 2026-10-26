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

package controlplaneapprover

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

func TestNodeReadyPredicate(t *testing.T) {
	t.Parallel()
	notReady := func(node *corev1.Node) *corev1.Node {
		node.Status.Conditions[0].Status = corev1.ConditionFalse
		return node
	}
	worker := func() *corev1.Node {
		node := testControlPlaneNode("w1")
		node.Labels = map[string]string{}
		return node
	}

	for name, tc := range map[string]struct {
		old, new *corev1.Node
		want     bool
	}{
		"a master turns Ready":    {notReady(testControlPlaneNode("n1")), testControlPlaneNode("n1"), true},
		"a master turns NotReady": {testControlPlaneNode("n1"), notReady(testControlPlaneNode("n1")), true},
		"an arbiter turns Ready":  {notReady(testArbiterNode("a1")), testArbiterNode("a1"), true},
		"a heartbeat of a master": {testControlPlaneNode("n1"), testControlPlaneNode("n1"), false},
		"a worker turns Ready":    {notReady(worker()), worker(), false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, NodeReadyPredicate().Update(event.UpdateEvent{ObjectOld: tc.old, ObjectNew: tc.new}))
		})
	}
}

func TestNodeDeletePredicate(t *testing.T) {
	t.Parallel()
	worker := testControlPlaneNode("w1")
	worker.Labels = map[string]string{}

	require.True(t, NodeReadyPredicate().Delete(event.DeleteEvent{Object: testControlPlaneNode("n1")}))
	require.True(t, NodeReadyPredicate().Delete(event.DeleteEvent{Object: testArbiterNode("a1")}))
	require.False(t, NodeReadyPredicate().Delete(event.DeleteEvent{Object: worker}))
}

func TestOperationDeletePredicate(t *testing.T) {
	t.Parallel()
	inFlight := newOperation("etcd-running", "n1", controlplanev1alpha1.OperationComponentEtcd, true)
	completed := newOperation("etcd-done", "n1", controlplanev1alpha1.OperationComponentEtcd, true)
	completed.Status.Conditions = []metav1.Condition{{
		Type:   controlplanev1alpha1.CPOConditionCompleted,
		Status: metav1.ConditionTrue,
		Reason: controlplanev1alpha1.CPOReasonOperationCompleted,
	}}

	require.True(t, OperationPredicate().Delete(event.DeleteEvent{Object: &inFlight}))
	require.False(t, OperationPredicate().Delete(event.DeleteEvent{Object: &completed}))
}
