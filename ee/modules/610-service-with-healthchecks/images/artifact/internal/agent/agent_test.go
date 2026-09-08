/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package agent

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/pkg/log"

	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"
)

const (
	testNamespace = "team-d8-ui-demo"
	testSWHName   = "afb6b6179f7a240379b969366a6f6a75"
	testNodeName  = "hv-06"
	testPodIP     = "10.12.5.86"
)

func newTestReconciler() *ServiceWithHealthchecksReconciler {
	return &ServiceWithHealthchecksReconciler{
		nodeName: testNodeName,
		logger:   log.NewNop(),
		healthchecksResultsByServiceWithHealthchecks: make(map[types.NamespacedName][]HealthcheckTarget),
	}
}

func newTestSWH() networkv1alpha1.ServiceWithHealthchecks {
	return networkv1alpha1.ServiceWithHealthchecks{
		ObjectMeta: metav1.ObjectMeta{Name: testSWHName, Namespace: testNamespace},
	}
}

func newPod(name string, phase corev1.PodPhase, ready bool, ip string) corev1.Pod {
	readyStatus := corev1.ConditionFalse
	if ready {
		readyStatus = corev1.ConditionTrue
	}
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         testNamespace,
			UID:               types.UID("uid-" + name),
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
		},
		Status: corev1.PodStatus{
			Phase:  phase,
			PodIP:  ip,
			PodIPs: []corev1.PodIP{{IP: ip}},
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: readyStatus},
			},
			ContainerStatuses: []corev1.ContainerStatus{{Ready: ready}},
		},
	}
}

func newTerminatingPod(name string, ip string) corev1.Pod {
	pod := newPod(name, corev1.PodRunning, true, ip)
	now := metav1.Now()
	pod.DeletionTimestamp = &now
	pod.Finalizers = []string{"test.deckhouse.io/keep"}
	return pod
}

func successfulProbeDetails() []ProbeResultDetail {
	return []ProbeResultDetail{
		{
			id:               "tcp:" + testPodIP + ":32412",
			mode:             "tcp",
			targetPort:       32412,
			successful:       true,
			successCount:     1,
			successThreshold: 1,
			failureThreshold: 3,
		},
	}
}

func TestPodShouldBeTracked(t *testing.T) {
	tests := []struct {
		name string
		pod  func() corev1.Pod
		want bool
	}{
		{
			name: "running pod with IP is tracked",
			pod:  func() corev1.Pod { return newPod("running", corev1.PodRunning, true, testPodIP) },
			want: true,
		},
		{
			name: "failed pod is not tracked even though it keeps its IP",
			pod:  func() corev1.Pod { return newPod("failed", corev1.PodFailed, false, testPodIP) },
			want: false,
		},
		{
			name: "succeeded pod is not tracked",
			pod:  func() corev1.Pod { return newPod("succeeded", corev1.PodSucceeded, false, testPodIP) },
			want: false,
		},
		{
			name: "pod without IP is not tracked",
			pod:  func() corev1.Pod { return newPod("pending", corev1.PodPending, false, "") },
			want: false,
		},
		{
			name: "terminating pod is still tracked to be published as terminating endpoint",
			pod: func() corev1.Pod {
				return newTerminatingPod("terminating", testPodIP)
			},
			want: true,
		},
		{
			name: "terminating pod which already failed is not tracked",
			pod: func() corev1.Pod {
				pod := newTerminatingPod("terminating-failed", testPodIP)
				pod.Status.Phase = corev1.PodFailed
				return pod
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := tt.pod()
			if got := podShouldBeTracked(&pod); got != tt.want {
				t.Errorf("podShouldBeTracked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name string
		pod  func() corev1.Pod
		want bool
	}{
		{
			name: "running pod with Ready condition",
			pod:  func() corev1.Pod { return newPod("ready", corev1.PodRunning, true, testPodIP) },
			want: true,
		},
		{
			name: "running pod without Ready condition",
			pod:  func() corev1.Pod { return newPod("not-ready", corev1.PodRunning, false, testPodIP) },
			want: false,
		},
		{
			name: "pod in Error phase is never ready",
			pod:  func() corev1.Pod { return newPod("error", corev1.PodFailed, false, testPodIP) },
			want: false,
		},
		{
			name: "evicted pod without container statuses is not ready",
			pod: func() corev1.Pod {
				pod := newPod("evicted", corev1.PodFailed, false, testPodIP)
				pod.Status.ContainerStatuses = nil
				pod.Status.Conditions = nil
				return pod
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := tt.pod()
			if got := isPodReady(&pod); got != tt.want {
				t.Errorf("isPodReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPodsStateMapSkipsNotPublishablePods(t *testing.T) {
	podList := corev1.PodList{Items: []corev1.Pod{
		newPod("running", corev1.PodRunning, true, testPodIP),
		newTerminatingPod("terminating", testPodIP),
		newPod("error", corev1.PodFailed, false, testPodIP),
		newPod("pending", corev1.PodPending, false, ""),
	}}

	stateMap := getPodsStateMap(podList)

	if len(stateMap) != 2 {
		t.Fatalf("expected the running and the terminating pods in the map, got %d entries: %v", len(stateMap), stateMap)
	}
	if got := stateMap[types.NamespacedName{Namespace: testNamespace, Name: "running"}]; !got.ready || got.terminating {
		t.Errorf("running pod state = %+v, want {ready: true, terminating: false}", got)
	}
	if got := stateMap[types.NamespacedName{Namespace: testNamespace, Name: "terminating"}]; !got.terminating {
		t.Errorf("terminating pod state = %+v, want terminating: true", got)
	}
}

// The reported case: a VM pod failed on this node while another pod serves the very same IP
// on another node, so its probes keep succeeding and only the phase check removes it.
func TestSyncDropsTerminalPodWithStaleSuccessfulProbes(t *testing.T) {
	r := newTestReconciler()
	swh := newTestSWH()
	swhKey := types.NamespacedName{Namespace: testNamespace, Name: testSWHName}

	errorPod := newPod("d8v-vm-commander-demo-worker-gmgrd", corev1.PodFailed, false, testPodIP)

	// the pod used to be ready with all its probes succeeding
	r.healthchecksResultsByServiceWithHealthchecks[swhKey] = []HealthcheckTarget{
		{
			targetHost:         testPodIP,
			podName:            errorPod.Name,
			podNamespace:       testNamespace,
			podUID:             errorPod.UID,
			podReady:           true,
			creationTime:       errorPod.CreationTimestamp.Time,
			lastCheck:          time.Now(),
			probeResultDetails: successfulProbeDetails(),
		},
	}

	r.syncResultsMapWithPodList(swh, corev1.PodList{Items: []corev1.Pod{errorPod}})

	if got := len(r.healthchecksResultsByServiceWithHealthchecks[swhKey]); got != 0 {
		t.Fatalf("expected the terminal pod to be dropped from targets, got %d targets", got)
	}

	if endpoints := r.buildEndpoints(swh); len(endpoints) != 0 {
		t.Fatalf("expected no endpoints for the terminal pod, got %+v", endpoints)
	}
}

// A still-running pod which lost its readiness is no longer probed, so the previous results
// must not keep it published.
func TestSyncResetsProbeResultsForNotReadyPod(t *testing.T) {
	r := newTestReconciler()
	swh := newTestSWH()
	swhKey := types.NamespacedName{Namespace: testNamespace, Name: testSWHName}

	pod := newPod("worker", corev1.PodRunning, false, testPodIP)
	r.healthchecksResultsByServiceWithHealthchecks[swhKey] = []HealthcheckTarget{
		{
			targetHost:         testPodIP,
			podName:            pod.Name,
			podNamespace:       testNamespace,
			podUID:             pod.UID,
			podReady:           true,
			creationTime:       pod.CreationTimestamp.Time,
			probeResultDetails: successfulProbeDetails(),
		},
	}

	r.syncResultsMapWithPodList(swh, corev1.PodList{Items: []corev1.Pod{pod}})

	targets := r.healthchecksResultsByServiceWithHealthchecks[swhKey]
	if len(targets) != 1 {
		t.Fatalf("expected the pod to stay in targets, got %d targets", len(targets))
	}
	if targets[0].podReady {
		t.Error("expected the target to become not ready")
	}
	if len(targets[0].probeResultDetails) != 0 {
		t.Errorf("expected stale probe results to be reset, got %+v", targets[0].probeResultDetails)
	}
	if endpoints := r.buildEndpoints(swh); len(endpoints) != 0 {
		t.Fatalf("expected no endpoints for the not ready pod, got %+v", endpoints)
	}
}

func TestSyncResetsProbeResultsOnPodRecreation(t *testing.T) {
	r := newTestReconciler()
	swh := newTestSWH()
	swhKey := types.NamespacedName{Namespace: testNamespace, Name: testSWHName}

	pod := newPod("worker", corev1.PodRunning, true, testPodIP)
	r.healthchecksResultsByServiceWithHealthchecks[swhKey] = []HealthcheckTarget{
		{
			targetHost:         "10.12.5.10",
			podName:            pod.Name,
			podNamespace:       testNamespace,
			podUID:             "old-uid",
			podReady:           true,
			creationTime:       pod.CreationTimestamp.Time,
			probeResultDetails: successfulProbeDetails(),
		},
	}

	r.syncResultsMapWithPodList(swh, corev1.PodList{Items: []corev1.Pod{pod}})

	targets := r.healthchecksResultsByServiceWithHealthchecks[swhKey]
	if len(targets) != 1 {
		t.Fatalf("expected a single target, got %d", len(targets))
	}
	if targets[0].targetHost != testPodIP || targets[0].podUID != pod.UID {
		t.Errorf("expected the target to be updated, got host %q uid %q", targets[0].targetHost, targets[0].podUID)
	}
	if len(targets[0].probeResultDetails) != 0 {
		t.Errorf("expected probe results of the previous pod instance to be reset, got %+v", targets[0].probeResultDetails)
	}
}

func TestBuildEndpointsPublishesReadyPod(t *testing.T) {
	r := newTestReconciler()
	swh := newTestSWH()
	swhKey := types.NamespacedName{Namespace: testNamespace, Name: testSWHName}

	pod := newPod("worker", corev1.PodRunning, true, testPodIP)
	r.healthchecksResultsByServiceWithHealthchecks[swhKey] = []HealthcheckTarget{
		{
			targetHost:         testPodIP,
			podName:            pod.Name,
			podNamespace:       testNamespace,
			podUID:             pod.UID,
			podReady:           true,
			probeResultDetails: successfulProbeDetails(),
		},
	}

	endpoints := r.buildEndpoints(swh)
	if len(endpoints) != 1 {
		t.Fatalf("expected the ready pod to be published, got %d endpoints", len(endpoints))
	}
	if !endpointIsReady(endpoints[0]) {
		t.Error("expected the endpoint to be ready")
	}
}

// Graceful shutdown: a pod being deleted keeps serving traffic until it disappears.
func TestTerminatingPodStaysPublishedAsServing(t *testing.T) {
	r := newTestReconciler()
	swh := newTestSWH()
	swhKey := types.NamespacedName{Namespace: testNamespace, Name: testSWHName}

	pod := newTerminatingPod("worker", testPodIP)
	r.healthchecksResultsByServiceWithHealthchecks[swhKey] = []HealthcheckTarget{
		{
			targetHost:         testPodIP,
			podName:            pod.Name,
			podNamespace:       testNamespace,
			podUID:             pod.UID,
			podReady:           true,
			probeResultDetails: successfulProbeDetails(),
		},
	}

	r.syncResultsMapWithPodList(swh, corev1.PodList{Items: []corev1.Pod{pod}})

	targets := r.healthchecksResultsByServiceWithHealthchecks[swhKey]
	if len(targets) != 1 {
		t.Fatalf("expected the terminating pod to stay in targets, got %d targets", len(targets))
	}
	if !targets[0].podTerminating {
		t.Error("expected the target to be marked as terminating")
	}
	if len(targets[0].probeResultDetails) == 0 {
		t.Error("expected probe results of the terminating pod to be preserved")
	}

	endpoints := r.buildEndpoints(swh)
	if len(endpoints) != 1 {
		t.Fatalf("expected the terminating pod to stay published, got %d endpoints", len(endpoints))
	}
	if endpointIsReady(endpoints[0]) {
		t.Error("terminating endpoint must not be ready")
	}
	if !endpointIsServing(endpoints[0]) {
		t.Error("terminating endpoint with succeeding probes must be serving")
	}
	if !endpointIsTerminating(endpoints[0]) {
		t.Error("terminating endpoint must be marked as terminating")
	}
}

// The shutting down pod stopped accepting connections: still published, but not serving.
func TestTerminatingPodStopsServingWhenProbesFail(t *testing.T) {
	r := newTestReconciler()
	swh := newTestSWH()
	swhKey := types.NamespacedName{Namespace: testNamespace, Name: testSWHName}

	r.healthchecksResultsByServiceWithHealthchecks[swhKey] = []HealthcheckTarget{
		{
			targetHost:         testPodIP,
			podName:            "worker",
			podNamespace:       testNamespace,
			podUID:             "uid-worker",
			podReady:           false,
			podTerminating:     true,
			probeResultDetails: []ProbeResultDetail{},
		},
	}

	endpoints := r.buildEndpoints(swh)
	if len(endpoints) != 1 {
		t.Fatalf("expected the terminating pod to stay published, got %d endpoints", len(endpoints))
	}
	if endpointIsServing(endpoints[0]) || endpointIsReady(endpoints[0]) {
		t.Error("terminating endpoint with failing probes must be neither serving nor ready")
	}
	if !endpointIsTerminating(endpoints[0]) {
		t.Error("terminating endpoint must be marked as terminating")
	}
}

func newEndpoint(uid types.UID, address string, ready bool) discoveryv1.Endpoint {
	terminating := false
	return discoveryv1.Endpoint{
		Addresses: []string{address},
		TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: string(uid), Namespace: testNamespace, UID: uid},
		Conditions: discoveryv1.EndpointConditions{
			Ready:       &ready,
			Serving:     &ready,
			Terminating: &terminating,
		},
	}
}

func newTerminatingEndpoint(uid types.UID, address string, serving bool) discoveryv1.Endpoint {
	endpoint := newEndpoint(uid, address, false)
	endpoint.Conditions.Serving = &serving
	endpoint.Conditions.Terminating = ptrTo(true)
	return endpoint
}

func TestEndpointsAreEqual(t *testing.T) {
	tests := []struct {
		name string
		old  []discoveryv1.Endpoint
		new  []discoveryv1.Endpoint
		want bool
	}{
		{
			name: "identical endpoints",
			old:  []discoveryv1.Endpoint{newEndpoint("uid-a", testPodIP, true)},
			new:  []discoveryv1.Endpoint{newEndpoint("uid-a", testPodIP, true)},
			want: true,
		},
		{
			name: "readiness change is detected",
			old:  []discoveryv1.Endpoint{newEndpoint("uid-a", testPodIP, true)},
			new:  []discoveryv1.Endpoint{newEndpoint("uid-a", testPodIP, false)},
			want: false,
		},
		{
			name: "serving change of a terminating endpoint is detected",
			old:  []discoveryv1.Endpoint{newTerminatingEndpoint("uid-a", testPodIP, true)},
			new:  []discoveryv1.Endpoint{newTerminatingEndpoint("uid-a", testPodIP, false)},
			want: false,
		},
		{
			name: "terminating change is detected",
			old:  []discoveryv1.Endpoint{newEndpoint("uid-a", testPodIP, false)},
			new:  []discoveryv1.Endpoint{newTerminatingEndpoint("uid-a", testPodIP, false)},
			want: false,
		},
		{
			name: "address change is detected",
			old:  []discoveryv1.Endpoint{newEndpoint("uid-a", testPodIP, true)},
			new:  []discoveryv1.Endpoint{newEndpoint("uid-a", "10.12.5.87", true)},
			want: false,
		},
		{
			name: "endpoints count change is detected",
			old:  []discoveryv1.Endpoint{newEndpoint("uid-a", testPodIP, true)},
			new:  []discoveryv1.Endpoint{},
			want: false,
		},
		{
			name: "order does not matter",
			old: []discoveryv1.Endpoint{
				newEndpoint("uid-b", "10.12.5.87", true),
				newEndpoint("uid-a", testPodIP, true),
			},
			new: []discoveryv1.Endpoint{
				newEndpoint("uid-a", testPodIP, true),
				newEndpoint("uid-b", "10.12.5.87", true),
			},
			want: true,
		},
		{
			name: "missing Ready condition is treated as ready",
			old:  []discoveryv1.Endpoint{{Addresses: []string{testPodIP}}},
			new:  []discoveryv1.Endpoint{{Addresses: []string{testPodIP}, Conditions: discoveryv1.EndpointConditions{Ready: ptrTo(true)}}},
			want: true,
		},
		{
			name: "endpoints without TargetRef do not panic",
			old:  []discoveryv1.Endpoint{{Addresses: []string{testPodIP}, Conditions: discoveryv1.EndpointConditions{Ready: ptrTo(true)}}},
			new:  []discoveryv1.Endpoint{{Addresses: []string{testPodIP}, Conditions: discoveryv1.EndpointConditions{Ready: ptrTo(false)}}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := endpointsAreEqual(tt.old, tt.new); got != tt.want {
				t.Errorf("endpointsAreEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEndpointsAreEqualKeepsArgumentsIntact(t *testing.T) {
	old := []discoveryv1.Endpoint{
		newEndpoint("uid-b", "10.12.5.87", true),
		newEndpoint("uid-a", testPodIP, true),
	}
	new := []discoveryv1.Endpoint{
		newEndpoint("uid-a", testPodIP, true),
		newEndpoint("uid-b", "10.12.5.87", true),
	}

	endpointsAreEqual(old, new)

	if old[0].TargetRef.UID != "uid-b" || new[0].TargetRef.UID != "uid-a" {
		t.Error("endpointsAreEqual() must not reorder its arguments")
	}
}

func ptrTo[T any](v T) *T {
	return &v
}
