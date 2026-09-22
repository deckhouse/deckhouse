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
	"strings"
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

func renameTo(name string) nodeOpt {
	return func(n *corev1.Node) {
		if n.Annotations == nil {
			n.Annotations = map[string]string{}
		}
		n.Annotations[RenameToAnnotation] = name
	}
}

func nodeType(t string) nodeOpt {
	return func(n *corev1.Node) { n.Labels[nodecommon.NodeTypeLabel] = t }
}

func newNode(name string, opts ...nodeOpt) *corev1.Node {
	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{nodecommon.NodeTypeLabel: nodeTypeStatic}},
	}
	for _, o := range opts {
		o(n)
	}
	return n
}

func newReconciler(t *testing.T, objs ...runtime.Object) (*Reconciler, *record.FakeRecorder) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	rec := record.NewFakeRecorder(10)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: rec}}, rec
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
	return node.Annotations[RenameToAnnotation]
}

func drainEvents(rec *record.FakeRecorder) string {
	var all []string
	for {
		select {
		case e := <-rec.Events:
			all = append(all, e)
		default:
			return strings.Join(all, "\n")
		}
	}
}

func TestRemovesTheNodeObjectOfANodeThatHasGoneQuiet(t *testing.T) {
	r, rec := newReconciler(t, newNode("old-name", renameTo("new-name"), ready(false)))

	reconcile(t, r, "old-name")

	if exists(t, r, "old-name") {
		t.Fatal("the Node object is still there, so the machine cannot come back under its new name")
	}
	if ev := drainEvents(rec); !strings.Contains(ev, "NodeRenameAccepted") {
		t.Fatalf("expected a NodeRenameAccepted event, got: %s", ev)
	}
}

// rename_node.sh stops kubelet before it asks, so a node still reporting either
// has not gone quiet yet or never meant to. Either way it keeps its Node object.
func TestWaitsWhileTheNodeIsStillReporting(t *testing.T) {
	r, _ := newReconciler(t, newNode("old-name", renameTo("new-name"), ready(true)))

	res := reconcile(t, r, "old-name")

	if !exists(t, r, "old-name") {
		t.Fatal("a node that is still Ready had its Node object removed")
	}
	if res.RequeueAfter != retryInterval {
		t.Fatalf("expected a requeue after %s, got %+v", retryInterval, res)
	}
	if annotation(t, r, "old-name") == "" {
		t.Fatal("the request was dropped instead of being waited on")
	}
}

func TestRefusesANodeNamedByItsInfrastructure(t *testing.T) {
	// CloudStatic belongs here too: it is given no providerID, so the cloud
	// controller manager has nothing but the node's name to find the machine it
	// has to initialize by.
	for _, nt := range []string{"CloudEphemeral", "CloudPermanent", "CloudStatic"} {
		t.Run(nt, func(t *testing.T) {
			r, rec := newReconciler(t, newNode("cloud-node", renameTo("new-name"), ready(false), nodeType(nt)))

			reconcile(t, r, "cloud-node")

			if !exists(t, r, "cloud-node") {
				t.Fatalf("a %s node was removed on a rename request", nt)
			}
			if got := annotation(t, r, "cloud-node"); got != "" {
				t.Fatalf("a refused request should be taken off the node, got %q", got)
			}
			if ev := drainEvents(rec); !strings.Contains(ev, "NodeRenameRejected") {
				t.Fatalf("expected a NodeRenameRejected event, got: %s", ev)
			}
		})
	}
}

func TestRefusesANameNoNodeCouldCarry(t *testing.T) {
	for _, name := range []string{"Not_A_Name", "-leading-dash", strings.Repeat("a", 254)} {
		t.Run(name[:min(len(name), 20)], func(t *testing.T) {
			r, _ := newReconciler(t, newNode("old-name", renameTo(name), ready(false)))

			reconcile(t, r, "old-name")

			if !exists(t, r, "old-name") {
				t.Fatalf("the Node object was removed for an impossible new name %q", name)
			}
			if got := annotation(t, r, "old-name"); got != "" {
				t.Fatalf("a refused request should be taken off the node, got %q", got)
			}
		})
	}
}

func TestRefusesARequestForTheNameTheNodeAlreadyHas(t *testing.T) {
	r, _ := newReconciler(t, newNode("same-name", renameTo("same-name"), ready(false)))

	reconcile(t, r, "same-name")

	if !exists(t, r, "same-name") {
		t.Fatal("the node deleted itself over a request that asked for nothing")
	}
	if got := annotation(t, r, "same-name"); got != "" {
		t.Fatalf("expected the request to be cleared, got %q", got)
	}
}

func TestDoesNothingWithoutARequest(t *testing.T) {
	r, _ := newReconciler(t, newNode("plain", ready(false)))

	reconcile(t, r, "plain")

	if !exists(t, r, "plain") {
		t.Fatal("a node that asked for nothing was removed")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
