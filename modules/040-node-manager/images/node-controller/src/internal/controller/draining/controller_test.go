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
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

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

	// drainPasses caps how many reconciles a drain is given to get going, and
	// wakePoll is how long each one waits for the drain to report back.
	drainPasses = 6
	wakePoll    = 500 * time.Millisecond
)

// harness is a Reconciler wired to fake clients, plus the two channels the tests
// read: the wake channel a finished drain writes to, and the recorded events.
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

// drain drives a full drain and returns the error of the pass that collects the
// result, because that is where a failed drain surfaces.
//
// The flow spends a pass or two getting the node ready — a stale drained=user to
// strip, a cordon to write — so this reconciles until the drain reports back
// instead of assuming a fixed number of passes.
func (h *harness) drain(t *testing.T) error {
	t.Helper()

	for range drainPasses {
		h.reconcile(t)

		select {
		case <-h.drains.wake:
			_, err := h.Reconcile(context.Background(), request())

			return err
		case <-time.After(wakePoll):
		}
	}

	t.Fatal("the eviction never reported back")

	return nil
}

func (h *harness) mustDrain(t *testing.T) {
	t.Helper()
	if err := h.drain(t); err != nil {
		t.Fatalf("drain: %v", err)
	}
}

// blockUntilCancelled registers a drain that does nothing but wait to be
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

// TestReconcile_Drain walks a request from the annotation that asks for a
// drain to the annotations that record it.
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
			name:        "a hand drain records its own source",
			annotations: map[string]string{nodecommon.DrainingAnnotation: userSource},
			want:        map[string]string{nodecommon.DrainedAnnotation: userSource},
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
			name: "a stale drained=user is replaced by the new drain's own result",
			annotations: map[string]string{
				nodecommon.DrainingAnnotation: bashibleSource,
				nodecommon.DrainedAnnotation:  userSource,
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
// gets from a single pass: nothing, except a stale drained=user being dropped
// once the node is back in service.
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
			name:        "a stale drained=user is dropped from a schedulable node",
			annotations: map[string]string{nodecommon.DrainedAnnotation: userSource},
		},
		{
			// On a cordoned node it still marks a drain somebody is dealing
			// with, so it is left where it is.
			name:              "drained=user stays on a cordoned node",
			annotations:       map[string]string{nodecommon.DrainedAnnotation: userSource},
			unschedulable:     true,
			want:              map[string]string{nodecommon.DrainedAnnotation: userSource},
			wantUnschedulable: true,
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

// TestReconcile_EvictionEndsBadly covers ordinary failures and deadlines. Both
// leave the request in place, write the error to the drain-failed annotation,
// and raise the gauge NodeStuckInDraining reads.
func TestReconcile_EvictionEndsBadly(t *testing.T) {
	for _, tc := range []struct {
		name            string
		drainTimeout    *int
		wantErrContains string
	}{
		{
			name:            "a failure keeps the request and records the error",
			wantErrContains: errBoom.Error(),
		},
		{
			name:            "a deadline keeps the request and records the error",
			drainTimeout:    ptr.To(0),
			wantErrContains: "drain deadline exceeded",
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
			if err == nil || !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Fatalf("drain err = %v, want an error containing %q", err, tc.wantErrContains)
			}

			updated := h.node(t)
			if got := updated.Annotations[nodecommon.DrainingAnnotation]; got != bashibleSource {
				t.Fatalf("draining annotation = %q, want %q", got, bashibleSource)
			}
			if got, drained := updated.Annotations[nodecommon.DrainedAnnotation]; drained {
				t.Fatalf("drained annotation = %q, want no drained annotation at all", got)
			}
			if got := updated.Annotations[nodecommon.DrainFailedAnnotation]; !strings.Contains(got, tc.wantErrContains) {
				t.Fatalf("drain-failed annotation = %q, want it to contain %q", got, tc.wantErrContains)
			}
			if !updated.Spec.Unschedulable {
				t.Fatal("a node whose eviction failed must stay cordoned")
			}
			// Both outcomes have to reach NodeStuckInDraining. This gauge is the
			// only term of that alert's expression a drain moves: the other two
			// are node_group_node_status, whose max over the statuses is 1 for
			// every node updateapproval tracks, and kube_node_status_condition.
			if got := metricValue(t, nodeName); got != 1 {
				t.Fatalf("failure gauge = %v, want 1", got)
			}
		})
	}
}

// A failed drain leaves a marker and a raised gauge on the node. The retry has to
// clear both on its own, or a transient API error would need an operator to delete
// an annotation by hand before the node could finish updating.
func TestReconcile_FailedDrainRecoversOnNextAttempt(t *testing.T) {
	cs := fake.NewSimpleClientset()
	var listCalls atomic.Int32
	cs.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		if listCalls.Add(1) == 1 {
			return true, nil, errBoom
		}

		return false, nil, nil
	})
	h := newHarness(t, cs, node(map[string]string{
		nodecommon.DrainingAnnotation: bashibleSource,
	}, false))
	t.Cleanup(func() { clearDrainMetric(nodeName) })

	if err := h.drain(t); err == nil || !strings.Contains(err.Error(), errBoom.Error()) {
		t.Fatalf("first drain err = %v, want an error containing %q", err, errBoom)
	}
	failed := h.node(t)
	if got, drained := failed.Annotations[nodecommon.DrainedAnnotation]; drained {
		t.Fatalf("drained annotation after the failed attempt = %q, want none", got)
	}
	if got := failed.Annotations[nodecommon.DrainFailedAnnotation]; !strings.Contains(got, errBoom.Error()) {
		t.Fatalf("drain-failed annotation = %q, want it to contain %q", got, errBoom)
	}
	if got := metricValue(t, nodeName); got != 1 {
		t.Fatalf("failure gauge after the failed attempt = %v, want 1", got)
	}

	// The API failure was transient. A normal retry now succeeds without an
	// operator changing annotations by hand.
	h.mustDrain(t)

	assertAnnotations(t, h.node(t), map[string]string{nodecommon.DrainedAnnotation: bashibleSource})
	if got := metricValue(t, nodeName); got != 0 {
		t.Fatalf("failure gauge = %v, want it cleared", got)
	}
}

// The stale marker is stripped on a pass of its own, before the cordon: left
// there it would read as the new drain's own result, and a second hand drain
// would write the same drained=user over it, so nothing would tell the two apart.
func TestReconcile_StaleUserResultIsClearedBeforeTheDrain(t *testing.T) {
	h := newHarness(t, fake.NewSimpleClientset(), nodeGroup(nil), node(map[string]string{
		nodecommon.DrainingAnnotation: userSource,
		nodecommon.DrainedAnnotation:  userSource,
	}, false))

	h.reconcile(t)

	afterFirst := h.node(t)
	assertAnnotations(t, afterFirst, map[string]string{nodecommon.DrainingAnnotation: userSource})
	if afterFirst.Spec.Unschedulable {
		t.Fatal("the pass that strips the marker should not also cordon the node")
	}

	h.reconcile(t)

	if !h.node(t).Spec.Unschedulable {
		t.Fatal("the next pass should have cordoned the node")
	}
}

// The cordon has to be durable before the drain starts: pods must stop being
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

// Withdrawing the request stops the drain instead of letting it run to completion
// and record a result nobody asked for. The last attempt's failure goes with it,
// marker and gauge: nothing is being attempted any more.
func TestReconcile_WithdrawnRequestCancelsEviction(t *testing.T) {
	h := newHarness(t, fake.NewSimpleClientset(), node(map[string]string{
		nodecommon.DrainFailedAnnotation: "boom",
	}, true))
	nodeDrainingGauge.WithLabelValues(nodeName).Set(1)
	t.Cleanup(func() { clearDrainMetric(nodeName) })

	h.blockUntilCancelled(t)
	h.reconcile(t)

	updated := h.node(t)
	assertAnnotations(t, updated, nil)
	if got := metricValue(t, nodeName); got != 0 {
		t.Fatalf("failure gauge = %v, want it cleared", got)
	}
	h.awaitEvent(t, "DrainCancelled")
}

// The gauge lives in this process's memory and the marker on the Node, so after a
// restart the marker is all there is to rebuild the gauge from. The eviction is
// still blocked, so the cancel check proves no fresh failure raised it.
func TestReconcile_RestoresFailureMetricFromAnnotation(t *testing.T) {
	h := newHarness(t, fake.NewSimpleClientset(), node(map[string]string{
		nodecommon.DrainingAnnotation:    bashibleSource,
		nodecommon.DrainFailedAnnotation: "persisted failure",
	}, true))
	t.Cleanup(func() { clearDrainMetric(nodeName) })

	h.blockUntilCancelled(t)
	h.reconcile(t)

	if got := metricValue(t, nodeName); got != 1 {
		t.Fatalf("restored failure gauge = %v, want 1", got)
	}
	if running, err := h.drains.cancel(t.Context(), nodeName); err != nil || !running {
		t.Fatalf("cancel = (%v, %v): the running eviction should still be there", running, err)
	}
}

// A reconcile arriving while the drain runs must not disturb it.
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

// A collected result frees the id, so a failed drain is followed by a fresh
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

// A deleted node's drain is abandoned rather than left evicting pods on
// behalf of an object nobody can see.
func TestReconcile_DeletedNodeCancelsItsEviction(t *testing.T) {
	h := newHarness(t, fake.NewSimpleClientset())
	nodeDrainingGauge.WithLabelValues(nodeName).Set(1)
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

// TestSetupWatches covers the subscription: the wake channel is registered as a
// raw source, only nodes in a NodeGroup are admitted, and an update is admitted
// only when something a drain depends on has changed.
func TestSetupWatches(t *testing.T) {
	w := &captureWatcher{}
	(&Reconciler{drains: newDrainer(t.Context(), nil)}).SetupWatches(w)

	if len(w.rawSources) != 1 {
		t.Fatalf("got %d raw sources, want the wake channel", len(w.rawSources))
	}
	if w.predicate == nil {
		t.Fatal("no event filter was registered")
	}

	t.Run("create", func(t *testing.T) {
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
				if got := w.predicate.Create(event.CreateEvent{Object: obj}); got != tc.want {
					t.Fatalf("admitted = %v, want %v", got, tc.want)
				}
			})
		}
	})

	// A node is written constantly — kubelet refreshes its status every few
	// seconds — and reconciling all of that would be one wake-up per node per
	// heartbeat, for a controller with nothing to do about any of it.
	t.Run("update", func(t *testing.T) {
		for _, tc := range []struct {
			name          string
			before, after *corev1.Node
			want          bool
		}{
			{
				name:   "a status-only change is ignored",
				before: node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, false),
				after:  withReady(node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, false)),
			},
			{
				name:   "a request appearing is admitted",
				before: node(nil, false),
				after:  node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, false),
				want:   true,
			},
			{
				name:   "a request being withdrawn is admitted",
				before: node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, true),
				after:  node(nil, true),
				want:   true,
			},
			{
				name:   "a recorded result is admitted",
				before: node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, true),
				after:  node(map[string]string{nodecommon.DrainedAnnotation: bashibleSource}, true),
				want:   true,
			},
			{
				// This is what brings the drain's own pass about.
				name:   "the cordon being written is admitted",
				before: node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, false),
				after:  node(map[string]string{nodecommon.DrainingAnnotation: bashibleSource}, true),
				want:   true,
			},
			{
				name:   "a node in no group is never admitted",
				before: &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}},
				after: &corev1.Node{ObjectMeta: metav1.ObjectMeta{
					Name:        nodeName,
					Annotations: map[string]string{nodecommon.DrainingAnnotation: bashibleSource},
				}},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got := w.predicate.Update(event.UpdateEvent{ObjectOld: tc.before, ObjectNew: tc.after})
				if got != tc.want {
					t.Fatalf("admitted = %v, want %v", got, tc.want)
				}
			})
		}
	})
}

// withReady marks the node Ready, standing in for the status churn a kubelet
// produces without touching anything a drain reads.
func withReady(n *corev1.Node) *corev1.Node {
	n.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}

	return n
}

// The drain's context tells the drainer why the task ended: a cancelled one
// is skipped, a deadline still has to bring the node back to be recorded.
func TestWakeNode_SkipsOnlyCancelledEvictions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		deadline bool
		cancel   bool
		wantWake bool
	}{
		{name: "a finished eviction wakes the node", wantWake: true},
		{name: "an eviction that ran out of its deadline wakes the node", deadline: true, wantWake: true},
		{name: "a cancelled eviction does not", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newDrainer(t.Context(), nil)

			ctx := t.Context()
			switch {
			case tc.deadline:
				expired, stop := context.WithTimeout(context.Background(), 0)
				defer stop()
				<-expired.Done()
				ctx = expired
			case tc.cancel:
				cancelled, stop := context.WithCancel(context.Background())
				stop()
				ctx = cancelled
			}

			d.wakeNode(ctx, nodeName)

			select {
			case ev := <-d.wake:
				if !tc.wantWake {
					t.Fatalf("woken for %q, want no wake-up", ev.Object.GetName())
				}
				if ev.Object.GetName() != nodeName {
					t.Fatalf("woken for %q, want %q", ev.Object.GetName(), nodeName)
				}
			default:
				if tc.wantWake {
					t.Fatal("the eviction ended without waking the node")
				}
			}
		})
	}
}

// metricValue reads the current d8_node_draining gauge value for a node.
// Returns 0 when no series exists.
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

// syncDrainMetric only ever sets the gauge. The pass that starts a retry can read
// the Node from a cache written before the failure was recorded, and clearing the
// gauge there would drop NodeStuckInDraining for that attempt.
func TestSyncDrainMetric_OnlyEverRaisesTheGauge(t *testing.T) {
	t.Cleanup(func() { clearDrainMetric(nodeName) })

	syncDrainMetric(nodeName, map[string]string{nodecommon.DrainFailedAnnotation: "boom"})
	syncDrainMetric(nodeName, nil)

	if got := metricValue(t, nodeName); got != 1 {
		t.Fatalf("failure gauge = %v, want it left at 1", got)
	}
}

// The eviction error arrives as one line per pod. The alert tells an operator to
// print this annotation, so the value has to be a single line, and it has to be
// valid UTF-8 after the cut.
func TestTruncateForAnnotation(t *testing.T) {
	const wantFlat = "a b"
	if got := truncateForAnnotation("  a\n\n  b  "); got != wantFlat {
		t.Fatalf("flattened = %q, want %q", got, wantFlat)
	}

	// 1500 bytes of three-byte runes, so the cut at 1024 lands mid-rune.
	got := truncateForAnnotation(strings.Repeat("€", 500))
	if !utf8.ValidString(got) {
		t.Fatalf("truncated value is not valid UTF-8: %q", got)
	}
	const suffix = "\u2026 (truncated)"
	if !strings.HasSuffix(got, suffix) {
		t.Fatalf("truncated value = %q, want it to end with %q", got, suffix)
	}
	// The cut drops at most one partial rune, so the body lands just under the limit.
	if body := len(got) - len(suffix); body > 1024 || body <= 1024-utf8.UTFMax {
		t.Fatalf("truncated body = %d bytes, want the 1024-byte limit less at most one partial rune", body)
	}
}
