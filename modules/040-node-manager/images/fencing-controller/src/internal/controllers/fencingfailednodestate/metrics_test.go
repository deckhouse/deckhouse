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

package fencingfailednodestate

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// TestPhaseMetricHasOneSeriesPerIncident covers a change of the phase and the
// end of the incident.
func TestPhaseMetricHasOneSeriesPerIncident(t *testing.T) {
	for name, remove := range map[string]func(*harness){
		"the object is deleted": func(h *harness) {
			if err := h.api.Delete(h.t.Context(), h.get()); err != nil {
				h.t.Fatalf("delete: %v", err)
			}
		},
		"the object is being deleted": func(h *harness) {
			incident := h.get()
			incident.Finalizers = []string{"example.com/keep"}

			if err := h.api.Update(h.t.Context(), incident); err != nil {
				h.t.Fatalf("add finalizer: %v", err)
			}

			if err := h.api.Delete(h.t.Context(), incident); err != nil {
				h.t.Fatalf("delete: %v", err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, failedState())

			h.reconcileAt(observedAt)
			assertPhaseSeries(t, v1alpha1.PhaseSuspected, "true")

			h.writeHeartbeat(observedAt)
			h.reconcileAt(observedAt)
			assertPhaseSeries(t, v1alpha1.PhaseFallbackAlive, "true")

			remove(h)
			h.reconcileAt(observedAt)

			if got := testutil.CollectAndCount(phaseGauge); got != 0 {
				t.Errorf("the gauge holds %d series after the end of the incident, want none", got)
			}
		})
	}
}

// TestPhaseMetricStaysWhileTheIncidentIsBlocked covers the incidents whose
// phase does not change, because the controller cannot process them.
func TestPhaseMetricStaysWhileTheIncidentIsBlocked(t *testing.T) {
	t.Run("object of another node", func(t *testing.T) {
		incident := failedState()
		incident.Spec.Armed = false

		h := newHarness(t, incident)
		h.nodes.foreign = true

		h.reconcileAt(observedAt)

		// An empty phase reads as Suspected.
		assertPhaseSeries(t, v1alpha1.PhaseSuspected, "false")
	})

	t.Run("configuration error", func(t *testing.T) {
		incident := failedState()
		incident.Status.Phase = v1alpha1.PhaseProven

		h := newHarness(t, incident)
		h.profiles.err = missingProfile()

		if _, err := h.reconcile(); err == nil {
			t.Fatal("reconcile succeeded while the profile was missing")
		}

		assertPhaseSeries(t, v1alpha1.PhaseProven, "true")
	})
}

// TestOutOfServiceMetricFollowsTheTaint covers a taint that another writer set
// and removed, and a Node that is deleted with the taint.
func TestOutOfServiceMetricFollowsTheTaint(t *testing.T) {
	outOfServiceTaintedGauge.Reset()

	node := liveNode()
	node.Spec.Taints = []corev1.Taint{{Key: corev1.TaintNodeOutOfService, Value: "manual", Effect: corev1.TaintEffectNoSchedule}}

	other := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-4"}}

	api := newFakeAPI(t, node, other)
	reconcileNode := reconcileOutOfServiceMetric(api)

	update := func(change func(*corev1.Node)) {
		t.Helper()

		var current corev1.Node
		if err := api.Get(t.Context(), types.NamespacedName{Name: nodeName}, &current); err != nil {
			t.Fatalf("get node: %v", err)
		}

		change(&current)

		if err := api.Update(t.Context(), &current); err != nil {
			t.Fatalf("update node: %v", err)
		}
	}

	reconcile := func() {
		t.Helper()

		for _, name := range []string{nodeName, other.Name} {
			if _, err := reconcileNode(t.Context(), request(name)); err != nil {
				t.Fatalf("reconcile node %s: %v", name, err)
			}
		}
	}

	reconcile()
	assertOutOfServiceSeries(t, true)

	update(func(n *corev1.Node) { n.Spec.Taints = nil })
	reconcile()
	assertOutOfServiceSeries(t, false)

	update(func(n *corev1.Node) { n.Spec.Taints = []corev1.Taint{outOfServiceTaint} })
	reconcile()
	assertOutOfServiceSeries(t, true)

	if err := api.Delete(t.Context(), liveNode()); err != nil {
		t.Fatalf("delete node: %v", err)
	}

	reconcile()
	assertOutOfServiceSeries(t, false)
}

// assertPhaseSeries checks that the gauge has one series, and that it is the
// series of the incident with the given phase.
func assertPhaseSeries(t *testing.T, phase v1alpha1.FencingFailedNodeStatePhase, armed string) {
	t.Helper()

	if got := testutil.CollectAndCount(phaseGauge); got != 1 {
		t.Fatalf("the gauge holds %d series, want one for the incident", got)
	}

	if got := testutil.ToFloat64(phaseGauge.WithLabelValues(nodeName, string(phase), armed)); got != 1 {
		t.Errorf("the series of phase %s and armed %s reads %v, want 1", phase, armed, got)
	}
}

// assertOutOfServiceSeries checks that the gauge has the series of the tainted
// node only.
func assertOutOfServiceSeries(t *testing.T, tainted bool) {
	t.Helper()

	want := 0
	if tainted {
		want = 1
	}

	if got := testutil.CollectAndCount(outOfServiceTaintedGauge); got != want {
		t.Fatalf("the gauge holds %d series, want %d", got, want)
	}

	if tainted {
		if got := testutil.ToFloat64(outOfServiceTaintedGauge.WithLabelValues(nodeName)); got != 1 {
			t.Errorf("the series of the tainted node reads %v, want 1", got)
		}
	}
}
