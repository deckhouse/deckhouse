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
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
	"github.com/deckhouse/node-controller/internal/task"
)

var errBoom = errors.New("boom")

const (
	nodeName      = "node-1"
	nodeGroupName = "worker"
	taskWait      = 10 * time.Second
)

// harness is a Reconciler wired to fake clients, plus the two channels the tests
// read: the wake channel a finished eviction writes to, and the recorded events.
type harness struct {
	*Reconciler
	events chan string
}

func newHarness(t *testing.T, kubeClient kubernetes.Interface, objs ...runtime.Object) *harness {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1 scheme: %v", err)
	}
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()

	recorder := record.NewFakeRecorder(10)

	return &harness{
		Reconciler: &Reconciler{
			Base:   register.Base{Client: cl, Recorder: recorder},
			drains: newDrainer(t.Context(), kubeClient),
		},
		events: recorder.Events,
	}
}

// node builds a node the controller's predicate admits.
func node(annotations map[string]string, unschedulable bool) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nodeName,
			Labels:      map[string]string{nodecommon.NodeGroupLabel: nodeGroupName},
			Annotations: annotations,
		},
		Spec: corev1.NodeSpec{Unschedulable: unschedulable},
	}
}

func nodeGroup(drainTimeout *int) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: nodeGroupName},
		Spec: v1.NodeGroupSpec{
			NodeType:               v1.NodeTypeStatic,
			NodeDrainTimeoutSecond: drainTimeout,
		},
	}
}

func (h *harness) reconcile(t *testing.T) {
	t.Helper()
	if _, err := h.Reconcile(context.Background(), request()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func request() ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: nodeName}}
}

func (h *harness) node(t *testing.T) *corev1.Node {
	t.Helper()
	fresh := &corev1.Node{}
	if err := h.Client.Get(context.Background(), types.NamespacedName{Name: nodeName}, fresh); err != nil {
		t.Fatalf("get node: %v", err)
	}
	return fresh
}

// drain runs the reconciles a background eviction takes and returns the error of
// the last one, because that is where a failed eviction surfaces. A node that
// arrives schedulable needs one pass more: the pass that writes the cordon is
// not the pass that starts the eviction.
func (h *harness) drain(t *testing.T) error {
	t.Helper()

	cordoned := h.node(t).Spec.Unschedulable
	h.reconcile(t)
	if !cordoned {
		h.reconcile(t)
	}

	select {
	case <-h.drains.wake:
	case <-time.After(taskWait):
		t.Fatal("timed out waiting for the eviction to finish")
	}

	_, err := h.Reconcile(context.Background(), request())

	return err
}

func (h *harness) mustDrain(t *testing.T) {
	t.Helper()
	if err := h.drain(t); err != nil {
		t.Fatalf("drain: %v", err)
	}
}

// blockUntilCancelled registers an eviction that does nothing but wait to be
// cancelled — the in-memory state a reconcile sees while a real one is in
// flight, without having to stall a fake clientset to get there.
func (h *harness) blockUntilCancelled(t *testing.T) {
	t.Helper()
	err := h.drains.tasks.Start(t.Context(), task.TaskID(nodeName),
		func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}, nil)
	if err != nil {
		t.Fatalf("start a blocking eviction: %v", err)
	}
}

func (h *harness) awaitEvent(t *testing.T, reason string) {
	t.Helper()
	select {
	case ev := <-h.events:
		if !strings.Contains(ev, reason) {
			t.Fatalf("got event %q, want one mentioning %q", ev, reason)
		}
	case <-time.After(taskWait):
		t.Fatalf("no %s event was recorded", reason)
	}
}

func assertAnnotations(t *testing.T, got *corev1.Node, want map[string]string) {
	t.Helper()
	if !maps.Equal(got.Annotations, want) {
		t.Fatalf("annotations = %v, want %v", got.Annotations, want)
	}
}

// TestReconcile_Drain walks a request from the annotation that asks for an
// eviction to the annotations that record it.
func TestReconcile_Drain(t *testing.T) {
	for _, tc := range []struct {
		name          string
		annotations   map[string]string
		unschedulable bool
		drainTimeout  *int
		want          map[string]string
	}{
		{
			name:        "a bashible request is recorded",
			annotations: map[string]string{nodecommon.DrainingAnnotation: bashibleSource},
			want:        map[string]string{nodecommon.DrainedAnnotation: bashibleSource},
		},
		{
			name:        "an empty request value means bashible",
			annotations: map[string]string{nodecommon.DrainingAnnotation: ""},
			want:        map[string]string{nodecommon.DrainedAnnotation: bashibleSource},
		},
		{
			name:        "a custom source is preserved",
			annotations: map[string]string{nodecommon.DrainingAnnotation: "machine-controller"},
			want:        map[string]string{nodecommon.DrainedAnnotation: "machine-controller"},
		},
		{
			name:        "a hand drain records nothing, since nobody polls for it",
			annotations: map[string]string{nodecommon.DrainingAnnotation: userSource},
			want:        nil,
		},
		{
			name:          "an already cordoned node still drains",
			annotations:   map[string]string{nodecommon.DrainingAnnotation: bashibleSource},
			unschedulable: true,
			want:          map[string]string{nodecommon.DrainedAnnotation: bashibleSource},
		},
		{
			name: "an empty result value is not the user source, so it is overwritten",
			annotations: map[string]string{
				nodecommon.DrainingAnnotation: bashibleSource,
				nodecommon.DrainedAnnotation:  "",
			},
			want: map[string]string{nodecommon.DrainedAnnotation: bashibleSource},
		},
		{
			name:         "the node group's drain timeout is honoured",
			annotations:  map[string]string{nodecommon.DrainingAnnotation: bashibleSource},
			drainTimeout: ptr.To(42),
			want:         map[string]string{nodecommon.DrainedAnnotation: bashibleSource},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, fake.NewSimpleClientset(),
				nodeGroup(tc.drainTimeout), node(tc.annotations, tc.unschedulable))

			h.mustDrain(t)

			updated := h.node(t)
			assertAnnotations(t, updated, tc.want)
			if !updated.Spec.Unschedulable {
				t.Fatal("a drained node must stay cordoned")
			}
		})
	}
}

// TestReconcile_NoLiveRequest covers what a node without a request in flight
// gets from a single pass: nothing, except an orphan result being dropped.
func TestReconcile_NoLiveRequest(t *testing.T) {
	for _, tc := range []struct {
		name              string
		annotations       map[string]string
		unschedulable     bool
		want              map[string]string
		wantUnschedulable bool
	}{
		{
			name: "a node nobody asked about is left alone",
		},
		{
			name:        "an orphan result is dropped from a schedulable node",
			annotations: map[string]string{nodecommon.DrainedAnnotation: userSource},
		},
		{
			name:              "an orphan result is dropped from a cordoned node too",
			annotations:       map[string]string{nodecommon.DrainedAnnotation: userSource},
			unschedulable:     true,
			wantUnschedulable: true,
		},
		{
			name: "an orphan result is dropped before a new request is looked at",
			annotations: map[string]string{
				nodecommon.DrainingAnnotation: bashibleSource,
				nodecommon.DrainedAnnotation:  userSource,
			},
			want: map[string]string{nodecommon.DrainingAnnotation: bashibleSource},
		},
		{
			name:        "a recorded bashible result is left alone",
			annotations: map[string]string{nodecommon.DrainedAnnotation: bashibleSource},
			want:        map[string]string{nodecommon.DrainedAnnotation: bashibleSource},
		},
		{
			// The cordon of a drained node belongs to updateapproval until the
			// configuration checksums match, so it must survive this controller.
			name:              "a cordon with no eviction behind it is left alone",
			annotations:       map[string]string{nodecommon.DrainedAnnotation: bashibleSource},
			unschedulable:     true,
			want:              map[string]string{nodecommon.DrainedAnnotation: bashibleSource},
			wantUnschedulable: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, fake.NewSimpleClientset(), node(tc.annotations, tc.unschedulable))

			h.reconcile(t)

			updated := h.node(t)
			assertAnnotations(t, updated, tc.want)
			if updated.Spec.Unschedulable != tc.wantUnschedulable {
				t.Fatalf("unschedulable = %v, want %v", updated.Spec.Unschedulable, tc.wantUnschedulable)
			}
		})
	}
}

// TestReconcile_EvictionEndsBadly covers the two ways an eviction fails, which
// are handled differently: a failure is retried, a deadline is not.
func TestReconcile_EvictionEndsBadly(t *testing.T) {
	for _, tc := range []struct {
		name         string
		drainTimeout *int
		wantErr      bool
		want         map[string]string
	}{
		{
			name:    "a failure keeps the request, so the requeue retries it",
			wantErr: true,
			want:    map[string]string{nodecommon.DrainingAnnotation: bashibleSource},
		},
		{
			// Retrying would get no further and the node's update must not
			// wedge, so the request is consumed and the result recorded.
			name:         "a deadline is recorded as drained anyway",
			drainTimeout: ptr.To(0),
			want:         map[string]string{nodecommon.DrainedAnnotation: bashibleSource},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cs := fake.NewSimpleClientset()
			cs.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, errBoom
			})
			h := newHarness(t, cs, nodeGroup(tc.drainTimeout),
				node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, false))
			t.Cleanup(func() { clearDrainMetric(nodeName) })

			err := h.drain(t)
			if (err != nil) != tc.wantErr {
				t.Fatalf("drain err = %v, want error: %v", err, tc.wantErr)
			}

			updated := h.node(t)
			assertAnnotations(t, updated, tc.want)
			if !updated.Spec.Unschedulable {
				t.Fatal("a node whose eviction failed must stay cordoned")
			}
			// Both outcomes have to reach NodeStuckInDraining.
			if got := metricValue(t, nodeName); got != 1 {
				t.Fatalf("failure gauge = %v, want 1", got)
			}
		})
	}
}

// The cordon has to be durable before the eviction starts: pods must stop being
// scheduled onto the node before anything begins emptying it.
func TestReconcile_CordonIsWrittenBeforeTheEvictionStarts(t *testing.T) {
	h := newHarness(t, fake.NewSimpleClientset(),
		node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, false))

	h.reconcile(t)

	if !h.node(t).Spec.Unschedulable {
		t.Fatal("the first pass should have written the cordon")
	}
	if running, err := h.drains.cancel(t.Context(), nodeName); err != nil || running {
		t.Fatalf("cancel = (%v, %v): no eviction should have started yet", running, err)
	}
}

// Withdrawing the request stops the eviction and gives the node back, instead of
// letting it run to completion and recording a result nobody asked for.
func TestReconcile_WithdrawnRequestCancelsAndUncordons(t *testing.T) {
	h := newHarness(t, fake.NewSimpleClientset(), node(nil, true))
	nodeDrainingGauge.WithLabelValues(nodeName, "boom").Set(1)
	t.Cleanup(func() { clearDrainMetric(nodeName) })

	h.blockUntilCancelled(t)
	h.reconcile(t)

	updated := h.node(t)
	if updated.Spec.Unschedulable {
		t.Fatal("a cancelled eviction should have uncordoned the node")
	}
	assertAnnotations(t, updated, nil)
	if got := metricValue(t, nodeName); got != 0 {
		t.Fatalf("failure gauge = %v, want it cleared", got)
	}
	h.awaitEvent(t, "DrainCancelled")
}

// A reconcile arriving while the eviction runs must not disturb it.
func TestReconcile_RunningEvictionIsLeftAlone(t *testing.T) {
	h := newHarness(t, fake.NewSimpleClientset(),
		node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, true))

	h.blockUntilCancelled(t)
	h.reconcile(t)

	assertAnnotations(t, h.node(t), map[string]string{nodecommon.DrainingAnnotation: bashibleSource})
	if running, err := h.drains.cancel(t.Context(), nodeName); err != nil || !running {
		t.Fatalf("cancel = (%v, %v): the running eviction should still be there", running, err)
	}
}

// A collected result frees the id, so a failed eviction is followed by a fresh
// one rather than by the same result for ever.
func TestReconcile_FailedEvictionIsRetriedWithAFreshTask(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errBoom
	})
	h := newHarness(t, cs, node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, false))
	t.Cleanup(func() { clearDrainMetric(nodeName) })

	if err := h.drain(t); err == nil {
		t.Fatal("expected the first eviction to fail")
	}
	if err := h.drain(t); err == nil {
		t.Fatal("expected the retried eviction to fail too")
	}
}

// A deleted node's eviction is abandoned rather than left evicting pods on
// behalf of an object nobody can see.
func TestReconcile_DeletedNodeCancelsItsEviction(t *testing.T) {
	h := newHarness(t, fake.NewSimpleClientset())
	nodeDrainingGauge.WithLabelValues(nodeName, "boom").Set(1)
	t.Cleanup(func() { clearDrainMetric(nodeName) })

	h.blockUntilCancelled(t)
	h.reconcile(t)

	if got := metricValue(t, nodeName); got != 0 {
		t.Fatalf("failure gauge = %v, want it cleared", got)
	}
	if running, err := h.drains.cancel(t.Context(), nodeName); err != nil || running {
		t.Fatalf("cancel = (%v, %v): the eviction should already be gone", running, err)
	}
}

func TestForPredicates_AdmitsOnlyNodesInANodeGroup(t *testing.T) {
	predicates := (&Reconciler{}).ForPredicates()
	if len(predicates) != 1 {
		t.Fatalf("got %d predicates, want 1", len(predicates))
	}

	for _, tc := range []struct {
		name   string
		labels map[string]string
		want   bool
	}{
		{name: "a node in a group", labels: map[string]string{nodecommon.NodeGroupLabel: nodeGroupName}, want: true},
		{name: "a node in no group", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName, Labels: tc.labels}}
			if got := predicates[0].Create(event.CreateEvent{Object: obj}); got != tc.want {
				t.Fatalf("admitted = %v, want %v", got, tc.want)
			}
		})
	}
}

// The wake channel must be a raw source and the group filter must not be a
// global event filter: a global one would also drop the name-only Node that
// carries a finished eviction back into the workqueue.
func TestSetupWatches_RegistersTheWakeChannelWithoutAGlobalFilter(t *testing.T) {
	r := &Reconciler{drains: newDrainer(t.Context(), nil)}
	w := &captureWatcher{}

	r.SetupWatches(w)

	if len(w.rawSources) != 1 {
		t.Fatalf("got %d raw sources, want 1", len(w.rawSources))
	}
	if w.predicate != nil {
		t.Fatal("the group filter must not be registered as a global event filter")
	}
}

// metricValue reads the current d8_node_draining gauge value for a node, summing
// across whatever message label is attached. Returns 0 when no series exists.
func metricValue(t *testing.T, name string) float64 {
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
			if lp.GetName() == "node" && lp.GetValue() == name {
				total += m.GetGauge().GetValue()
			}
		}
	}
	return total
}

type captureWatcher struct {
	predicate  predicate.Predicate
	rawSources []source.Source
}

func (w *captureWatcher) Owns(_ client.Object, _ ...builder.OwnsOption) {}
func (w *captureWatcher) Watches(_ client.Object, _ handler.EventHandler, _ ...builder.WatchesOption) {
}
func (w *captureWatcher) WatchesRawSource(src source.Source) {
	w.rawSources = append(w.rawSources, src)
}
func (w *captureWatcher) WithEventFilter(p predicate.Predicate) {
	w.predicate = p
}
