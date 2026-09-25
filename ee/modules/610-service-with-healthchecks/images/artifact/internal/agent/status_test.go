/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"
)

// TestProbeSuccessfulHonoursThresholds pins the semantics the CRD documents for successThreshold
// and failureThreshold. The previous implementation recomputed the state from the counters with a
// default of "failed", which meant failureThreshold never delayed anything: one failed check
// withdrew the endpoint however high the threshold was.
func TestProbeSuccessfulHonoursThresholds(t *testing.T) {
	const (
		successThreshold = 1
		failureThreshold = 3
	)

	tests := []struct {
		name         string
		previous     bool
		successCount int32
		failureCount int32
		want         bool
	}{
		{name: "never probed yet", previous: false, want: false},
		{name: "first success publishes", previous: false, successCount: 1, want: true},
		{name: "one failure below threshold keeps it up", previous: true, failureCount: 1, want: true},
		{name: "two failures below threshold keep it up", previous: true, failureCount: 2, want: true},
		{name: "third consecutive failure withdraws it", previous: true, failureCount: 3, want: false},
		{name: "further failures keep it down", previous: false, failureCount: 7, want: false},
		{name: "one success restores it", previous: false, successCount: 1, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := probeSuccessful(tt.previous, tt.successCount, tt.failureCount, successThreshold, failureThreshold)
			if got != tt.want {
				t.Errorf("probeSuccessful(previous=%v, success=%d, failure=%d) = %v, want %v",
					tt.previous, tt.successCount, tt.failureCount, got, tt.want)
			}
		})
	}
}

// TestProbeSuccessfulRequiresConsecutiveSuccesses covers a successThreshold above one, where a
// recovering probe must not be republished on its first success.
func TestProbeSuccessfulRequiresConsecutiveSuccesses(t *testing.T) {
	const (
		successThreshold = 2
		failureThreshold = 3
	)

	if got := probeSuccessful(false, 1, 0, successThreshold, failureThreshold); got {
		t.Error("one success met a successThreshold of 2")
	}
	if got := probeSuccessful(false, 2, 0, successThreshold, failureThreshold); !got {
		t.Error("two consecutive successes did not meet a successThreshold of 2")
	}
}

// runRounds replays a sequence of probe outcomes through the same pair of functions the worker
// uses, returning the state after each round. true in outcomes means the check succeeded.
func runRounds(outcomes []bool, successThreshold, failureThreshold int32) []bool {
	var (
		successCount, failureCount int32
		successful                 bool
	)

	states := make([]bool, 0, len(outcomes))
	for _, ok := range outcomes {
		var err error
		if !ok {
			err = fmt.Errorf("probe failed")
		}
		successCount, failureCount = calculateCounts(err, successCount, failureCount)
		successful = probeSuccessful(successful, successCount, failureCount, successThreshold, failureThreshold)
		states = append(states, successful)
	}

	return states
}

// TestEndpointSurvivesFailuresBelowThreshold is the scenario behind the defect: a probe with
// failureThreshold 3 has to fail three times in a row before the endpoint is withdrawn, and a
// success in between resets the run.
func TestEndpointSurvivesFailuresBelowThreshold(t *testing.T) {
	tests := []struct {
		name     string
		outcomes []bool
		want     []bool
	}{
		{
			name:     "three consecutive failures are needed",
			outcomes: []bool{true, false, false, false},
			want:     []bool{true, true, true, false},
		},
		{
			name:     "a success resets the failure run",
			outcomes: []bool{true, false, false, true, false, false},
			want:     []bool{true, true, true, true, true, true},
		},
		{
			name:     "recovery is immediate at successThreshold 1",
			outcomes: []bool{false, false, false, true},
			want:     []bool{false, false, false, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runRounds(tt.outcomes, 1, 3)
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("after round %d (outcomes %v): state = %v, want %v (full sequence %v)",
						i+1, tt.outcomes[:i+1], got[i], tt.want[i], got)
				}
			}
		})
	}
}

// TestFailedProbesMatchesProbeState keeps failedProbes and probesSuccessful from disagreeing: a
// probe is listed only while it is actually in the failed state, not merely because its last check
// missed while the threshold is still unmet.
func TestFailedProbesMatchesProbeState(t *testing.T) {
	target := HealthcheckTarget{
		targetHost: testPodIP,
		probeResultDetails: []ProbeResultDetail{
			{
				id: "tcp:" + testPodIP + ":32412", mode: "tcp", targetPort: 32412,
				// Failing, but only once out of the three the threshold requires.
				successful: true, failureCount: 1, successThreshold: 1, failureThreshold: 3,
			},
			{
				id: "http:" + testPodIP + ":8090", mode: "http", targetPort: 8090,
				successful: false, failureCount: 3, successThreshold: 1, failureThreshold: 3,
			},
		},
	}

	failed := target.FailedProbes()
	want := []string{"http:" + testPodIP + ":8090"}

	if len(failed) != len(want) || failed[0] != want[0] {
		t.Fatalf("FailedProbes() = %v, want %v", failed, want)
	}

	// The same set must be what makes probesSuccessful false, or the two fields contradict.
	if *areAllProbesSucceed(target.probeResultDetails) {
		t.Error("areAllProbesSucceed() = true while a probe is listed in FailedProbes()")
	}
}

func TestFailedProbesEmptyWhenAllSucceed(t *testing.T) {
	target := HealthcheckTarget{targetHost: testPodIP, probeResultDetails: successfulProbeDetails()}

	if failed := target.FailedProbes(); len(failed) != 0 {
		t.Errorf("FailedProbes() = %v, want empty", failed)
	}
	if !*areAllProbesSucceed(target.probeResultDetails) {
		t.Error("areAllProbesSucceed() = false for successful probes")
	}
}

// statusForTarget runs buildEndpointStatuses over a single target on this node.
func statusForTarget(t *testing.T, target HealthcheckTarget) networkv1alpha1.EndpointStatus {
	t.Helper()

	r := newTestReconciler()
	swh := newTestSWH()
	key := types.NamespacedName{Name: swh.GetName(), Namespace: swh.GetNamespace()}
	r.healthchecksResultsByServiceWithHealthchecks[key] = []HealthcheckTarget{target}

	statuses := r.buildEndpointStatuses(&swh)
	if len(statuses) != 1 {
		t.Fatalf("buildEndpointStatuses() returned %d statuses, want 1", len(statuses))
	}

	return statuses[0]
}

// TestEndpointReadyReflectsProbes covers the contradiction that started this: a pod passing its
// kubelet readiness probe while a ServiceWithHealthchecks probe fails used to be reported as
// ready: true next to a non-empty failedProbes, under a resource-level "Not all endpoints are
// ready".
func TestEndpointReadyReflectsProbes(t *testing.T) {
	failing := []ProbeResultDetail{
		{
			id: "http:" + testPodIP + ":8090", mode: "http", targetPort: 8090,
			successful: false, failureCount: 3, successThreshold: 1, failureThreshold: 3,
		},
	}

	tests := []struct {
		name                 string
		podReady             bool
		details              []ProbeResultDetail
		wantReady            bool
		wantProbesSuccessful bool
		wantFailedProbes     int
	}{
		{
			name: "pod ready and probes pass", podReady: true, details: successfulProbeDetails(),
			wantReady: true, wantProbesSuccessful: true,
		},
		{
			name: "pod ready but probes fail", podReady: true, details: failing,
			wantReady: false, wantProbesSuccessful: false, wantFailedProbes: 1,
		},
		{
			name: "pod not ready while probes pass", podReady: false, details: successfulProbeDetails(),
			wantReady: false, wantProbesSuccessful: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := statusForTarget(t, HealthcheckTarget{
				targetHost:         testPodIP,
				podName:            "pod-a",
				podReady:           tt.podReady,
				probeResultDetails: tt.details,
			})

			if status.Ready != tt.wantReady {
				t.Errorf("Ready = %v, want %v", status.Ready, tt.wantReady)
			}
			if status.ProbesSuccessful != tt.wantProbesSuccessful {
				t.Errorf("ProbesSuccessful = %v, want %v", status.ProbesSuccessful, tt.wantProbesSuccessful)
			}
			if len(status.FailedProbes) != tt.wantFailedProbes {
				t.Errorf("FailedProbes = %v, want %d entries", status.FailedProbes, tt.wantFailedProbes)
			}
			// Whatever the combination, a ready endpoint must never carry failed probes.
			if status.Ready && len(status.FailedProbes) > 0 {
				t.Errorf("Ready = true alongside FailedProbes = %v", status.FailedProbes)
			}
		})
	}
}

// TestReadyEndpointsCountMatchesEndpointStatuses ties the per-endpoint field to the aggregate the
// resource-level Ready condition is built from, so the two cannot drift apart again.
func TestReadyEndpointsCountMatchesEndpointStatuses(t *testing.T) {
	r := newTestReconciler()
	swh := newTestSWH()
	key := types.NamespacedName{Name: swh.GetName(), Namespace: swh.GetNamespace()}
	r.healthchecksResultsByServiceWithHealthchecks[key] = []HealthcheckTarget{
		{targetHost: testPodIP, podName: "pod-ok", podReady: true, probeResultDetails: successfulProbeDetails()},
		{targetHost: testPodIP, podName: "pod-failing", podReady: true, probeResultDetails: []ProbeResultDetail{
			{
				id: "http:" + testPodIP + ":8090", mode: "http", targetPort: 8090,
				successful: false, failureCount: 3, successThreshold: 1, failureThreshold: 3,
			},
		}},
	}

	status := r.buildRenewedStatus(&swh)

	if status.HealthcheckCondition.Endpoints != 2 {
		t.Errorf("Endpoints = %d, want 2", status.HealthcheckCondition.Endpoints)
	}
	if status.HealthcheckCondition.ReadyEndpoints != 1 {
		t.Errorf("ReadyEndpoints = %d, want 1", status.HealthcheckCondition.ReadyEndpoints)
	}

	readyInStatuses := int32(0)
	for _, endpoint := range status.EndpointStatuses {
		if endpoint.Ready {
			readyInStatuses++
		}
	}
	if readyInStatuses != status.HealthcheckCondition.ReadyEndpoints {
		t.Errorf("%d endpoints marked ready, but ReadyEndpoints = %d",
			readyInStatuses, status.HealthcheckCondition.ReadyEndpoints)
	}

	if len(status.Conditions) != 1 || status.Conditions[0].Reason != "NotAllEndpointsAreReady" {
		t.Errorf("conditions = %+v, want a single NotAllEndpointsAreReady condition", status.Conditions)
	}
}
