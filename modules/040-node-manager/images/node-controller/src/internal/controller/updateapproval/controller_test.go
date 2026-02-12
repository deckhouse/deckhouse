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

package updateapproval

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

func newTestReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	t.Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1 scheme: %v", err)
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
	return &Reconciler{
		Base: dynctrl.Base{Client: cl, Recorder: record.NewFakeRecorder(10)},
	}
}

func reconcileUA(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
	return res
}

func getNode(t *testing.T, r *Reconciler, name string) *corev1.Node {
	t.Helper()
	node := &corev1.Node{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, node); err != nil {
		t.Fatalf("get node %s: %v", name, err)
	}
	return node
}

func makeChecksumSecret(data map[string]string) *corev1.Secret {
	secretData := make(map[string][]byte, len(data))
	for k, v := range data {
		secretData[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ua.ConfigurationChecksumsSecretName,
			Namespace: ua.MachineNamespace,
		},
		Data: secretData,
	}
}

func makeReadyNode(name, ngName string, annotations map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      map[string]string{ua.NodeGroupLabel: ngName},
			Annotations: annotations,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
}

func boolPtr(b bool) *bool { return &b }

func TestReconcile_NodeGroupNotFound_NoError(t *testing.T) {
	r := newTestReconciler(t)
	reconcileUA(t, r, "nonexistent")
}

func TestReconcile_NoChecksumSecret_EarlyReturn(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
	node := makeReadyNode("n1", "worker", map[string]string{
		ua.WaitingForApprovalAnnotation: "",
	})

	r := newTestReconciler(t, ng, node)
	reconcileUA(t, r, "worker")

	updated := getNode(t, r, "n1")
	if _, ok := updated.Annotations[ua.WaitingForApprovalAnnotation]; !ok {
		t.Fatal("expected waiting-for-approval annotation to remain")
	}
}

func TestReconcile_NoWaitingNodes_NoChanges(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
	secret := makeChecksumSecret(map[string]string{"worker": "abc123"})
	node := makeReadyNode("n1", "worker", map[string]string{
		ua.ConfigurationChecksumAnnotation: "abc123",
	})

	r := newTestReconciler(t, ng, secret, node)
	reconcileUA(t, r, "worker")

	updated := getNode(t, r, "n1")
	if _, ok := updated.Annotations[ua.ApprovedAnnotation]; ok {
		t.Fatal("node should not have approved annotation")
	}
}

func TestReconcile_WaitingNode_GetsApproved(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
	secret := makeChecksumSecret(map[string]string{"worker": "abc123"})
	node := makeReadyNode("n1", "worker", map[string]string{
		ua.ConfigurationChecksumAnnotation: "old-checksum",
		ua.WaitingForApprovalAnnotation:    "",
	})

	r := newTestReconciler(t, ng, secret, node)
	reconcileUA(t, r, "worker")

	updated := getNode(t, r, "n1")
	if _, ok := updated.Annotations[ua.ApprovedAnnotation]; !ok {
		t.Fatal("expected approved annotation to be set")
	}
	if _, ok := updated.Annotations[ua.WaitingForApprovalAnnotation]; ok {
		t.Fatal("expected waiting-for-approval annotation to be removed")
	}
}

func TestReconcile_UpToDate_CleanedUp(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
	secret := makeChecksumSecret(map[string]string{"worker": "abc123"})
	node := makeReadyNode("n1", "worker", map[string]string{
		ua.ConfigurationChecksumAnnotation: "abc123",
		ua.ApprovedAnnotation:              "",
		ua.WaitingForApprovalAnnotation:    "",
	})

	r := newTestReconciler(t, ng, secret, node)
	reconcileUA(t, r, "worker")

	updated := getNode(t, r, "n1")
	if _, ok := updated.Annotations[ua.ApprovedAnnotation]; ok {
		t.Fatal("expected approved annotation to be removed")
	}
	if _, ok := updated.Annotations[ua.WaitingForApprovalAnnotation]; ok {
		t.Fatal("expected waiting-for-approval annotation to be removed")
	}
	if _, ok := updated.Annotations[ua.DisruptionRequiredAnnotation]; ok {
		t.Fatal("expected disruption-required annotation to be removed")
	}
	if _, ok := updated.Annotations[ua.DisruptionApprovedAnnotation]; ok {
		t.Fatal("expected disruption-approved annotation to be removed")
	}
	if _, ok := updated.Annotations[ua.DrainedAnnotation]; ok {
		t.Fatal("expected drained annotation to be removed")
	}
}

func TestReconcile_DisruptionRequired_Automatic_Approved(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeStatic,
			Disruptions: &v1.DisruptionsSpec{
				ApprovalMode: v1.DisruptionApprovalModeAutomatic,
				Automatic:    &v1.AutomaticDisruptionSpec{DrainBeforeApproval: boolPtr(false)},
			},
		},
	}
	secret := makeChecksumSecret(map[string]string{"worker": "abc123"})
	node := makeReadyNode("n1", "worker", map[string]string{
		ua.ConfigurationChecksumAnnotation: "old-checksum",
		ua.ApprovedAnnotation:              "",
		ua.DisruptionRequiredAnnotation:    "",
	})

	r := newTestReconciler(t, ng, secret, node)
	reconcileUA(t, r, "worker")

	updated := getNode(t, r, "n1")
	if _, ok := updated.Annotations[ua.DisruptionApprovedAnnotation]; !ok {
		t.Fatal("expected disruption-approved annotation to be set")
	}
	if _, ok := updated.Annotations[ua.DisruptionRequiredAnnotation]; ok {
		t.Fatal("expected disruption-required annotation to be removed")
	}
}

func TestReconcile_DisruptionRequired_Manual_NotApproved(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeStatic,
			Disruptions: &v1.DisruptionsSpec{
				ApprovalMode: v1.DisruptionApprovalModeManual,
			},
		},
	}
	secret := makeChecksumSecret(map[string]string{"worker": "abc123"})
	node := makeReadyNode("n1", "worker", map[string]string{
		ua.ConfigurationChecksumAnnotation: "old-checksum",
		ua.ApprovedAnnotation:              "",
		ua.DisruptionRequiredAnnotation:    "",
	})

	r := newTestReconciler(t, ng, secret, node)
	reconcileUA(t, r, "worker")

	updated := getNode(t, r, "n1")
	if _, ok := updated.Annotations[ua.DisruptionRequiredAnnotation]; !ok {
		t.Fatal("expected disruption-required annotation to remain")
	}
	if _, ok := updated.Annotations[ua.DisruptionApprovedAnnotation]; ok {
		t.Fatal("expected disruption-approved annotation to NOT be set")
	}
}

func TestReconcile_ConcurrencyLimit_OnlyOneApproved(t *testing.T) {
	maxConcurrent := intstr.FromInt32(1)
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeStatic,
			Update:   &v1.UpdateSpec{MaxConcurrent: &maxConcurrent},
		},
	}
	secret := makeChecksumSecret(map[string]string{"worker": "abc123"})
	n1 := makeReadyNode("n1", "worker", map[string]string{
		ua.ConfigurationChecksumAnnotation: "old",
		ua.WaitingForApprovalAnnotation:    "",
	})
	n2 := makeReadyNode("n2", "worker", map[string]string{
		ua.ConfigurationChecksumAnnotation: "old",
		ua.WaitingForApprovalAnnotation:    "",
	})

	r := newTestReconciler(t, ng, secret, n1, n2)
	reconcileUA(t, r, "worker")

	updated1 := getNode(t, r, "n1")
	updated2 := getNode(t, r, "n2")

	n1Approved := false
	n2Approved := false
	if _, ok := updated1.Annotations[ua.ApprovedAnnotation]; ok {
		n1Approved = true
	}
	if _, ok := updated2.Annotations[ua.ApprovedAnnotation]; ok {
		n2Approved = true
	}

	approvedCount := 0
	if n1Approved {
		approvedCount++
	}
	if n2Approved {
		approvedCount++
	}

	if approvedCount != 1 {
		t.Fatalf("expected exactly 1 node approved, got %d (n1=%v, n2=%v)", approvedCount, n1Approved, n2Approved)
	}
}
