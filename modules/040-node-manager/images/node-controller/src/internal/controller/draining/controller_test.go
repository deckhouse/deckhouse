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

package draining

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

var errBoom = errors.New("boom")

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1 scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{
		Base:       register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)},
		kubeClient: fake.NewSimpleClientset(),
	}
}

func getNode(t *testing.T, r *Reconciler, name string) *corev1.Node {
	t.Helper()
	node := &corev1.Node{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: name}, node); err != nil {
		t.Fatalf("get node %s: %v", name, err)
	}
	return node
}

func reconcile(t *testing.T, r *Reconciler, name string) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("reconcile %s: %v", name, err)
	}
	return res
}

func TestReconcile_NoAnnotations_Noop(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{nodecommon.NodeGroupLabel: "worker"},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if updated.Spec.Unschedulable {
		t.Fatal("node should not be cordoned without draining annotation")
	}
}

func TestReconcile_DrainingAnnotation_CordonsAndDrains(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainingAnnotation: "bashible"},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")

	// Should be cordoned
	if !updated.Spec.Unschedulable {
		t.Fatal("node should be cordoned after draining")
	}

	// Draining annotation should be removed, drained should be set
	if _, exists := updated.Annotations[nodecommon.DrainingAnnotation]; exists {
		t.Fatal("draining annotation should be removed")
	}
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatalf("expected drained annotation 'bashible', got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
}

func TestReconcile_DrainedUserSchedulable_RemovesDrained(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainedAnnotation: "user"},
		},
		Spec: corev1.NodeSpec{Unschedulable: false},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if _, exists := updated.Annotations[nodecommon.DrainedAnnotation]; exists {
		t.Fatal("drained annotation should be removed for schedulable node with user source")
	}
}

func TestReconcile_DrainedNonUser_Noop(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainedAnnotation: "bashible"},
		},
		Spec: corev1.NodeSpec{Unschedulable: false},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatal("drained annotation should remain for non-user source")
	}
}

func TestReconcile_AlreadyCordoned_StillDrains(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainingAnnotation: "bashible"},
		},
		Spec: corev1.NodeSpec{Unschedulable: true},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if _, exists := updated.Annotations[nodecommon.DrainingAnnotation]; exists {
		t.Fatal("draining annotation should be removed")
	}
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatalf("expected drained annotation 'bashible', got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
}

func TestReconcile_DrainingWithUserDrained_RemovesDrainedFirst(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{
				nodecommon.DrainingAnnotation: "bashible",
				nodecommon.DrainedAnnotation:  "user",
			},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatalf("expected drained annotation to be set to 'bashible', got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
}

func TestGetDrainTimeout_FromNodeGroup(t *testing.T) {
	timeout := 300
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType:               v1.NodeTypeStatic,
			NodeDrainTimeoutSecond: &timeout,
		},
	}

	r := newReconciler(t, ng)
	got := r.getDrainTimeout(context.Background(), "worker")

	expected := 300 * time.Second
	if got != expected {
		t.Fatalf("expected timeout %v, got %v", expected, got)
	}
}

func TestGetDrainTimeout_Default(t *testing.T) {
	r := newReconciler(t)
	got := r.getDrainTimeout(context.Background(), "nonexistent")

	if got != defaultDrainTimeout {
		t.Fatalf("expected default timeout %v, got %v", defaultDrainTimeout, got)
	}
}

func TestReconcile_NodeNotFound_NoError(t *testing.T) {
	r := newReconciler(t)
	reconcile(t, r, "nonexistent")
}

func TestGetDrainTimeout_NodeGroupWithoutTimeout_Default(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}

	r := newReconciler(t, ng)
	got := r.getDrainTimeout(context.Background(), "worker")

	if got != defaultDrainTimeout {
		t.Fatalf("expected default timeout %v when NodeDrainTimeoutSecond is nil, got %v", defaultDrainTimeout, got)
	}
}

func TestGetDrainTimeout_EmptyNodeGroup_Default(t *testing.T) {
	r := newReconciler(t)
	got := r.getDrainTimeout(context.Background(), "")

	if got != defaultDrainTimeout {
		t.Fatalf("expected default timeout %v for empty nodegroup, got %v", defaultDrainTimeout, got)
	}
}

func TestReconcile_DrainTimeoutFromNodeGroup_DrainsWithConfiguredTimeout(t *testing.T) {
	timeout := 42
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: v1.NodeGroupSpec{
			NodeType:               v1.NodeTypeStatic,
			NodeDrainTimeoutSecond: &timeout,
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainingAnnotation: "bashible"},
		},
	}

	r := newReconciler(t, ng, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatalf("expected drained=bashible, got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
}

// Empty draining annotation value is treated as "bashible" for backward compatibility.
func TestReconcile_EmptyDrainingAnnotation_TreatedAsBashible(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainingAnnotation: ""},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if !updated.Spec.Unschedulable {
		t.Fatal("node should be cordoned for empty draining annotation")
	}
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatalf("expected drained=bashible, got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
}

// Custom source is preserved verbatim into the drained annotation.
func TestReconcile_CustomSource_PreservedInDrained(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainingAnnotation: "machine-controller"},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if updated.Annotations[nodecommon.DrainedAnnotation] != "machine-controller" {
		t.Fatalf("expected drained=machine-controller, got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
}

// An empty drained annotation value is normalized to "bashible", so it is not
// treated as a user drain and the pre-drain removal step is skipped.
func TestReconcile_EmptyDrainedAnnotation_NormalizedNotUser(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{
				nodecommon.DrainingAnnotation: "bashible",
				nodecommon.DrainedAnnotation:  "",
			},
		},
	}

	r := newReconciler(t, node)
	reconcile(t, r, "node-1")

	updated := getNode(t, r, "node-1")
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatalf("expected drained=bashible, got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
	if !updated.Spec.Unschedulable {
		t.Fatal("node should be cordoned")
	}
}

// A non-timeout drain failure must return the error, set the failure gauge, and
// leave the draining annotation untouched (no drained annotation written).
func TestReconcile_DrainFails_ReturnsErrorAndSetsMetric(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainingAnnotation: "bashible"},
		},
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(node).Build()

	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "pods", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errBoom
	})
	r := &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}, kubeClient: cs}

	t.Cleanup(func() { clearDrainMetric("node-1") })

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "node-1"}})
	if err == nil {
		t.Fatal("expected drain error, got nil")
	}

	if got := metricValue(t, "node-1"); got != 1 {
		t.Fatalf("expected drain failure gauge=1, got %v", got)
	}

	updated := getNode(t, r, "node-1")
	if updated.Annotations[nodecommon.DrainingAnnotation] != "bashible" {
		t.Fatal("draining annotation should remain after failed drain")
	}
	if _, exists := updated.Annotations[nodecommon.DrainedAnnotation]; exists {
		t.Fatal("drained annotation should not be set after failed drain")
	}
	if !updated.Spec.Unschedulable {
		t.Fatal("node should remain cordoned after failed drain")
	}
}

// When the drain context is already expired, a drain failure is tolerated: the
// node is marked drained anyway and the draining annotation is removed.
func TestReconcile_DrainTimesOut_MarksDrainedAnyway(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "node-1",
			Labels:      map[string]string{nodecommon.NodeGroupLabel: "worker"},
			Annotations: map[string]string{nodecommon.DrainingAnnotation: "bashible"},
		},
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(node).Build()

	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "pods", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errBoom
	})
	r := &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}, kubeClient: cs}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "node-1"}})
	if err != nil {
		t.Fatalf("expected no error on drain timeout, got %v", err)
	}
	if res != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got %v", res)
	}

	updated := getNode(t, r, "node-1")
	if _, exists := updated.Annotations[nodecommon.DrainingAnnotation]; exists {
		t.Fatal("draining annotation should be removed after timeout")
	}
	if updated.Annotations[nodecommon.DrainedAnnotation] != "bashible" {
		t.Fatalf("expected drained=bashible after timeout, got %q", updated.Annotations[nodecommon.DrainedAnnotation])
	}
}

// Node deletion clears the per-node drain failure metric.
func TestReconcile_NodeNotFound_ClearsMetric(t *testing.T) {
	nodeDrainingGauge.WithLabelValues("gone", "boom").Set(1)
	t.Cleanup(func() { clearDrainMetric("gone") })

	r := newReconciler(t)
	reconcile(t, r, "gone")

	if got := metricValue(t, "gone"); got != 0 {
		t.Fatalf("expected gauge cleared for deleted node, got %v", got)
	}
}

func TestSetup_BuildsKubeClientFromManagerConfig(t *testing.T) {
	mgr := &configOnlyManager{cfg: &rest.Config{Host: "https://127.0.0.1:6443"}}
	r := &Reconciler{}

	if err := r.Setup(mgr); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if r.kubeClient == nil {
		t.Fatal("Setup did not initialize kubeClient")
	}
}

func TestSetup_InvalidConfig_ReturnsError(t *testing.T) {
	mgr := &configOnlyManager{cfg: &rest.Config{Host: "https://example.com", ExecProvider: &api.ExecConfig{}, AuthProvider: &api.AuthProviderConfig{}}}
	r := &Reconciler{}

	if err := r.Setup(mgr); err == nil {
		t.Fatal("expected error from invalid rest.Config, got nil")
	}
}

func TestSetupWatches_FiltersNodesByGroupLabel(t *testing.T) {
	r := &Reconciler{}
	w := &captureWatcher{}
	r.SetupWatches(w)

	if w.predicate == nil {
		t.Fatal("SetupWatches did not register an event filter predicate")
	}

	withLabel := &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:   "with",
		Labels: map[string]string{nodecommon.NodeGroupLabel: "worker"},
	}}
	withoutLabel := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "without"}}

	if !w.predicate.Create(event.CreateEvent{Object: withLabel}) {
		t.Fatal("expected node with group label to pass the filter")
	}
	if w.predicate.Create(event.CreateEvent{Object: withoutLabel}) {
		t.Fatal("expected node without group label to be filtered out")
	}
}

// metricValue reads the current d8_node_draining gauge value for a node, summing
// across whatever message label is attached. Returns 0 when no series exists.
func metricValue(t *testing.T, nodeName string) float64 {
	t.Helper()
	ch := make(chan prometheus.Metric, 16)
	nodeDrainingGauge.Collect(ch)
	close(ch)

	var total float64
	for metric := range ch {
		m := &dto.Metric{}
		if err := metric.Write(m); err != nil {
			t.Fatalf("write metric: %v", err)
		}
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "node" && lp.GetValue() == nodeName {
				total += m.GetGauge().GetValue()
			}
		}
	}
	return total
}

type captureWatcher struct {
	predicate predicate.Predicate
}

func (w *captureWatcher) Owns(_ client.Object, _ ...builder.OwnsOption) {}
func (w *captureWatcher) Watches(_ client.Object, _ handler.EventHandler, _ ...builder.WatchesOption) {
}
func (w *captureWatcher) WatchesRawSource(_ source.Source) {}
func (w *captureWatcher) WithEventFilter(p predicate.Predicate) {
	w.predicate = p
}

// configOnlyManager is a ctrl.Manager that only serves a rest.Config; every other
// method is inherited (and unused) from the embedded nil interface.
type configOnlyManager struct {
	manager.Manager
	cfg *rest.Config
}

func (m *configOnlyManager) GetConfig() *rest.Config { return m.cfg }
