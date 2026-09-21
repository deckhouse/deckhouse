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

package noderename

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	systemUUID = "4C4C4544-0042-3010-8036-B4C04F4E3233"
	machineID  = "f4e1c2b0a9d84e7f9c3b1a0d5e6f7a8b"
)

type nodeOpt func(*corev1.Node)

func ready(v bool) nodeOpt {
	status := corev1.ConditionFalse
	if v {
		status = corev1.ConditionTrue
	}
	return func(n *corev1.Node) {
		n.Status.Conditions = append(n.Status.Conditions, corev1.NodeCondition{Type: corev1.NodeReady, Status: status})
	}
}

func renamedFrom(old string) nodeOpt {
	return func(n *corev1.Node) {
		if n.Annotations == nil {
			n.Annotations = map[string]string{}
		}
		n.Annotations[RenamedFromAnnotation] = old
	}
}

func nodeType(t string) nodeOpt {
	return func(n *corev1.Node) { n.Labels[nodecommon.NodeTypeLabel] = t }
}

func machine(uuid, id string) nodeOpt {
	return func(n *corev1.Node) {
		n.Status.NodeInfo.SystemUUID = uuid
		n.Status.NodeInfo.MachineID = id
	}
}

func newNode(name string, opts ...nodeOpt) *corev1.Node {
	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{nodecommon.NodeTypeLabel: nodeTypeStatic}},
	}
	n.Status.NodeInfo.SystemUUID = systemUUID
	n.Status.NodeInfo.MachineID = machineID
	for _, o := range opts {
		o(n)
	}
	return n
}

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func reconcile(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
	return res
}

func exists(t *testing.T, r *Reconciler, name string) bool {
	t.Helper()
	err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, &corev1.Node{})
	if apierrors.IsNotFound(err) {
		return false
	}
	if err != nil {
		t.Fatalf("get node %s: %v", name, err)
	}
	return true
}

func annotation(t *testing.T, r *Reconciler, name string) string {
	t.Helper()
	node := &corev1.Node{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, node); err != nil {
		t.Fatalf("get node %s: %v", name, err)
	}
	return node.Annotations[RenamedFromAnnotation]
}

func TestCollectsThePreviousNodeOfTheSameMachine(t *testing.T) {
	r := newReconciler(t,
		newNode("new-name", renamedFrom("old-name"), ready(true)),
		newNode("old-name", ready(false)),
	)

	reconcile(t, r, "new-name")

	if exists(t, r, "old-name") {
		t.Fatal("the Node object left behind by the rename is still there")
	}
	if got := annotation(t, r, "new-name"); got != "" {
		t.Fatalf("the annotation should be cleared once the old Node is gone, got %q", got)
	}
}

func TestWaitsWhileTheRenamedNodeIsNotReady(t *testing.T) {
	r := newReconciler(t,
		newNode("new-name", renamedFrom("old-name"), ready(false)),
		newNode("old-name", ready(false)),
	)

	res := reconcile(t, r, "new-name")

	if !exists(t, r, "old-name") {
		t.Fatal("the old Node was removed before the rename was known to have worked")
	}
	if res.RequeueAfter != retryInterval {
		t.Fatalf("expected a requeue after %s, got %+v", retryInterval, res)
	}
	if annotation(t, r, "new-name") == "" {
		t.Fatal("the annotation was cleared while the rename was still unfinished")
	}
}

func TestLeavesAPreviousNodeThatIsStillReady(t *testing.T) {
	r := newReconciler(t,
		newNode("new-name", renamedFrom("old-name"), ready(true)),
		newNode("old-name", ready(true)),
	)

	res := reconcile(t, r, "new-name")

	if !exists(t, r, "old-name") {
		t.Fatal("a Ready node was deleted")
	}
	if res.RequeueAfter != retryInterval {
		t.Fatalf("expected a requeue after %s, got %+v", retryInterval, res)
	}
}

func TestRefusesToDeleteADifferentMachine(t *testing.T) {
	r := newReconciler(t,
		newNode("new-name", renamedFrom("someone-else"), ready(true)),
		newNode("someone-else", ready(false), machine("11111111-2222-3333-4444-555555555555", "0123456789abcdef0123456789abcdef")),
	)

	reconcile(t, r, "new-name")

	if !exists(t, r, "someone-else") {
		t.Fatal("a node the annotation merely pointed at was deleted")
	}
	if got := annotation(t, r, "new-name"); got != "" {
		t.Fatalf("a rejected claim should not be retried forever, annotation is still %q", got)
	}
}

func TestRefusesWhenTheMachineIdentityIsUnknown(t *testing.T) {
	r := newReconciler(t,
		newNode("new-name", renamedFrom("old-name"), ready(true)),
		newNode("old-name", ready(false), machine("", "")),
	)

	reconcile(t, r, "new-name")

	if !exists(t, r, "old-name") {
		t.Fatal("a node with no reported machine identity was deleted on an unverifiable claim")
	}
}

func TestIgnoresNodesThatAreNotStatic(t *testing.T) {
	r := newReconciler(t,
		newNode("new-name", renamedFrom("old-name"), ready(true), nodeType("CloudEphemeral")),
		newNode("old-name", ready(false)),
	)

	reconcile(t, r, "new-name")

	if !exists(t, r, "old-name") {
		t.Fatal("a CloudEphemeral node was allowed to collect another Node object")
	}
	if got := annotation(t, r, "new-name"); got != "" {
		t.Fatalf("expected the annotation to be cleared, got %q", got)
	}
}

func TestClearsAnAnnotationNamingTheNodeItself(t *testing.T) {
	r := newReconciler(t, newNode("new-name", renamedFrom("new-name"), ready(true)))

	reconcile(t, r, "new-name")

	if got := annotation(t, r, "new-name"); got != "" {
		t.Fatalf("expected the annotation to be cleared, got %q", got)
	}
	if !exists(t, r, "new-name") {
		t.Fatal("the node deleted itself")
	}
}

func TestClearsTheAnnotationWhenThePreviousNodeIsAlreadyGone(t *testing.T) {
	r := newReconciler(t, newNode("new-name", renamedFrom("old-name"), ready(true)))

	reconcile(t, r, "new-name")

	if got := annotation(t, r, "new-name"); got != "" {
		t.Fatalf("expected the annotation to be cleared, got %q", got)
	}
}

func TestDoesNothingWithoutTheAnnotation(t *testing.T) {
	r := newReconciler(t, newNode("plain", ready(true)), newNode("other", ready(false)))

	reconcile(t, r, "plain")

	if !exists(t, r, "other") {
		t.Fatal("an unrelated node was deleted")
	}
}
