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

package cpnplanner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

const (
	testGeneration = int64(2)
	testNodeName   = "master-0"
)

func baseCPN() *controlplanev1alpha1.ControlPlaneNode {
	return &controlplanev1alpha1.ControlPlaneNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testNodeName,
			Namespace:  constants.KubeSystemNamespace,
			Generation: testGeneration,
		},
		Spec: controlplanev1alpha1.ControlPlaneNodeSpec{
			CAChecksum: "ca-1",
			Components: controlplanev1alpha1.ComponentsSpec{
				Etcd:                  componentSpec("cfg-etcd-1", "pki-etcd-1"),
				KubeAPIServer:         componentSpec("cfg-apiserver-1", "pki-apiserver-1"),
				KubeControllerManager: componentSpec("cfg-cm-1", ""),
				KubeScheduler:         componentSpec("cfg-sched-1", ""),
			},
		},
	}
}

func allComponents() []controlplanev1alpha1.OperationComponent {
	return []controlplanev1alpha1.OperationComponent{
		controlplanev1alpha1.OperationComponentEtcd,
		controlplanev1alpha1.OperationComponentKubeAPIServer,
		controlplanev1alpha1.OperationComponentKubeControllerManager,
		controlplanev1alpha1.OperationComponentKubeScheduler,
	}
}

// readyPods returns a ready mirror pod for each of the given components on the test node.
func readyPods(components ...controlplanev1alpha1.OperationComponent) map[controlplanev1alpha1.OperationComponent]*corev1.Pod {
	pods := make(map[controlplanev1alpha1.OperationComponent]*corev1.Pod, len(components))
	for _, c := range components {
		pod := staticPod(c)
		pod.Status.Conditions = []corev1.PodCondition{{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		}}
		pods[c] = pod
	}
	return pods
}

// staticPod returns a running (but not necessarily ready) mirror pod for the component.
func staticPod(c controlplanev1alpha1.OperationComponent) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.MirrorPodName(testNodeName),
			Namespace: constants.KubeSystemNamespace,
		},
		Spec: corev1.PodSpec{NodeName: testNodeName},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

func componentSpec(config, pki string) controlplanev1alpha1.ComponentSpec {
	return controlplanev1alpha1.ComponentSpec{
		Checksums: controlplanev1alpha1.Checksums{Config: config, PKI: pki},
	}
}

func componentStatus(config, pki, ca string, certExpiry time.Time) controlplanev1alpha1.ComponentStatus {
	st := controlplanev1alpha1.ComponentStatus{
		Checksums: controlplanev1alpha1.Checksums{Config: config, PKI: pki, CA: ca},
	}
	if !certExpiry.IsZero() {
		st.CertificatesExpirationTime = map[string]metav1.Time{
			"server.crt": metav1.NewTime(certExpiry),
		}
	}
	return st
}

// inSyncStatus returns the status matching baseCPN spec.
// Zero certsExpiry means "no certificate observation data yet".
func inSyncStatus(certsExpiry time.Time) controlplanev1alpha1.ControlPlaneNodeStatus {
	return controlplanev1alpha1.ControlPlaneNodeStatus{
		CAChecksum: "ca-1",
		Components: controlplanev1alpha1.ComponentsStatus{
			Etcd:                  componentStatus("cfg-etcd-1", "pki-etcd-1", "ca-1", certsExpiry),
			KubeAPIServer:         componentStatus("cfg-apiserver-1", "pki-apiserver-1", "ca-1", certsExpiry),
			KubeControllerManager: componentStatus("cfg-cm-1", "", "ca-1", certsExpiry),
			KubeScheduler:         componentStatus("cfg-sched-1", "", "ca-1", certsExpiry),
		},
	}
}

func requireCondition(t *testing.T, conds []metav1.Condition, condType string, status metav1.ConditionStatus, reason, message string) {
	t.Helper()
	cond := meta.FindStatusCondition(conds, condType)
	require.NotNil(t, cond, "condition %s not found in %+v", condType, conds)
	require.Equal(t, status, cond.Status, "%s.status", condType)
	require.Equal(t, reason, cond.Reason, "%s.reason", condType)
	require.Equal(t, message, cond.Message, "%s.message", condType)
	require.Equal(t, testGeneration, cond.ObservedGeneration, "%s.observedGeneration", condType)
}

func TestConditions(t *testing.T) {
	farFuture := time.Now().Add(200 * 24 * time.Hour)
	soon := time.Now().Add(10 * 24 * time.Hour) // < CertRenewalThreshold (30d)
	created := time.Now().Add(-1 * time.Hour)

	pendingOp := func(cpn *controlplanev1alpha1.ControlPlaneNode) controlplanev1alpha1.ControlPlaneOperation {
		op := opFor(cpn, controlplanev1alpha1.OperationComponentEtcd, "etcd-op-pending", false, created)
		return op
	}
	runningOp := func(cpn *controlplanev1alpha1.ControlPlaneNode) controlplanev1alpha1.ControlPlaneOperation {
		return opFor(cpn, controlplanev1alpha1.OperationComponentEtcd, "etcd-op-running", true, created)
	}

	t.Run("ready pods, no operations, checksums applied: pods ready, in sync and ready", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		conds := ComputeConditions(cpn, readyPods(allComponents()...), nil)
		for _, c := range allComponents() {
			requireCondition(t, conds, podReadyType(c), metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, "")
			requireCondition(t, conds, inSyncType(c), metav1.ConditionTrue, controlplanev1alpha1.CPNReasonInSync, "")
			requireCondition(t, conds, readyType(c), metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, "")
		}
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionCertificatesHealthy,
			metav1.ConditionTrue, controlplanev1alpha1.CPNReasonHealthy, "")
	})

	t.Run("missing pod is not ready even with in sync status", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		pods := readyPods(allComponents()...)
		delete(pods, controlplanev1alpha1.OperationComponentEtcd) // etcd pod missing

		conds := ComputeConditions(cpn, pods, nil)
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdPodReady,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, "static pod not found")
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionAPIServerPodReady,
			metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, "")
	})

	t.Run("not ready and crash looping pods", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		notReady := staticPod(controlplanev1alpha1.OperationComponentEtcd)
		crashLooping := staticPod(controlplanev1alpha1.OperationComponentKubeAPIServer)
		crashLooping.Status.ContainerStatuses = []corev1.ContainerStatus{{
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		}}

		conds := ComputeConditions(cpn, map[controlplanev1alpha1.OperationComponent]*corev1.Pod{
			controlplanev1alpha1.OperationComponentEtcd:          notReady,
			controlplanev1alpha1.OperationComponentKubeAPIServer: crashLooping,
		}, nil)
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdPodReady,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady,
			"static pod etcd-master-0 is not ready")
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionAPIServerPodReady,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady,
			"static pod kube-apiserver-master-0 is in CrashLoopBackOff")
	})

	t.Run("ready summary: active operation makes a component with a ready pod not ready", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		conds := ComputeConditions(cpn, readyPods(allComponents()...),
			[]controlplanev1alpha1.ControlPlaneOperation{pendingOp(cpn)})
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdReady,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOperationInQueue, "etcd-op-pending")
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionAPIServerReady,
			metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, "")
	})

	t.Run("ready summary: not ready pod wins over an active operation", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		conds := ComputeConditions(cpn, nil, []controlplanev1alpha1.ControlPlaneOperation{runningOp(cpn)})
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdInSync,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOperationInFlight, "etcd-op-running")
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdReady,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, "static pod not found")
	})

	t.Run("no operation but checksums differ is OutOfSync", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		// Maintenance mode: the spec changed, no operation is created.
		cpn.Spec.Components.KubeAPIServer.Checksums.Config = "cfg-apiserver-2"
		cpn.Spec.Components.KubeAPIServer.Checksums.PKI = "pki-apiserver-2"

		conds := ComputeConditions(cpn, readyPods(allComponents()...), nil)
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionAPIServerInSync,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOutOfSync, "config checksum differs, pki checksum differs")
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionAPIServerReady,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOutOfSync, "config checksum differs, pki checksum differs")
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdInSync,
			metav1.ConditionTrue, controlplanev1alpha1.CPNReasonInSync, "")
	})

	t.Run("ca change is OutOfSync until applied", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		cpn.Spec.CAChecksum = "ca-2"

		conds := ComputeConditions(cpn, readyPods(allComponents()...), nil)
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionSchedulerInSync,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOutOfSync, "ca checksum differs")
	})

	t.Run("active operation wins over checksum comparison", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		cpn.Spec.Components.Etcd.Checksums.Config = "cfg-etcd-2"

		conds := ComputeConditions(cpn, readyPods(allComponents()...),
			[]controlplanev1alpha1.ControlPlaneOperation{runningOp(cpn)})
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdInSync,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOperationInFlight, "etcd-op-running")
	})

	t.Run("observe-only operation does not affect InSync", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		observe := runningOp(cpn)
		observe.Spec.Steps = []controlplanev1alpha1.StepName{controlplanev1alpha1.StepCertObserve}

		conds := ComputeConditions(cpn, readyPods(allComponents()...),
			[]controlplanev1alpha1.ControlPlaneOperation{observe})
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdInSync,
			metav1.ConditionTrue, controlplanev1alpha1.CPNReasonInSync, "")
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdReady,
			metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, "")
	})

	t.Run("pending operation is OperationInQueue", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		conds := ComputeConditions(cpn, readyPods(controlplanev1alpha1.OperationComponentEtcd),
			[]controlplanev1alpha1.ControlPlaneOperation{pendingOp(cpn)})
		// The pod stays healthy while an operation waits for approval: no false alarms.
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdPodReady,
			metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, "")
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdInSync,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOperationInQueue, "etcd-op-pending")
	})

	t.Run("approved operation is OperationInFlight", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		conds := ComputeConditions(cpn, readyPods(controlplanev1alpha1.OperationComponentEtcd),
			[]controlplanev1alpha1.ControlPlaneOperation{runningOp(cpn)})
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdInSync,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOperationInFlight, "etcd-op-running")
	})

	t.Run("in queue wins over in flight", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		older := opFor(cpn, controlplanev1alpha1.OperationComponentEtcd, "etcd-op-running", true, created.Add(-time.Hour))
		pendingNewer := opFor(cpn, controlplanev1alpha1.OperationComponentEtcd, "etcd-op-pending", false, created)

		conds := ComputeConditions(cpn, nil,
			[]controlplanev1alpha1.ControlPlaneOperation{older, pendingNewer})
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdInSync,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOperationInQueue,
			"etcd-op-running, etcd-op-pending")
	})

	t.Run("terminal operations do not count as active", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		completed := opFor(cpn, controlplanev1alpha1.OperationComponentEtcd, "etcd-op-completed", true, created)
		completed.Status.Conditions = []metav1.Condition{{
			Type:   controlplanev1alpha1.CPOConditionCompleted,
			Status: metav1.ConditionTrue,
			Reason: controlplanev1alpha1.CPOReasonOperationCompleted,
		}}
		abandoned := opFor(cpn, controlplanev1alpha1.OperationComponentEtcd, "etcd-op-abandoned", true, created.Add(-time.Hour))
		abandoned.Status.Conditions = []metav1.Condition{{
			Type:   controlplanev1alpha1.CPOConditionCompleted,
			Status: metav1.ConditionFalse,
			Reason: controlplanev1alpha1.CPOReasonOperationAbandoned,
		}}

		conds := ComputeConditions(cpn, nil, []controlplanev1alpha1.ControlPlaneOperation{completed, abandoned})
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdInSync,
			metav1.ConditionTrue, controlplanev1alpha1.CPNReasonInSync, "")
	})

	t.Run("retryable failed operation still counts as in flight", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		// OperationFailed is retryable and non-terminal: the operation controller keeps
		// executing it, so it is still presence: OperationInFlight.
		failed := opFor(cpn, controlplanev1alpha1.OperationComponentEtcd, "etcd-op-failed", true, created)
		failed.Status.Conditions = []metav1.Condition{{
			Type:    controlplanev1alpha1.CPOConditionCompleted,
			Status:  metav1.ConditionFalse,
			Reason:  controlplanev1alpha1.CPOReasonOperationFailed,
			Message: "boom",
		}}

		conds := ComputeConditions(cpn, nil, []controlplanev1alpha1.ControlPlaneOperation{failed})
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdInSync,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOperationInFlight, "etcd-op-failed")
	})

	t.Run("certificates expiring soon", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		cpn.Status.Components.Etcd.CertificatesExpirationTime = map[string]metav1.Time{
			"server.crt": metav1.NewTime(soon),
		}

		conds := ComputeConditions(cpn, nil, nil)
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionCertificatesHealthy,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonExpiredSoon, "Etcd")
	})

	t.Run("ca out of sync", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		cpn.Spec.CAChecksum = "ca-2" // drift

		conds := ComputeConditions(cpn, nil, nil)
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionCertificatesHealthy,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonWaitingForComponents,
			"Etcd, KubeAPIServer, KubeControllerManager, KubeScheduler")
	})

	t.Run("last transition time preserved when status unchanged", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		before := time.Now().Add(-24 * time.Hour)
		cpn.Status.Conditions = []metav1.Condition{{
			Type:               controlplanev1alpha1.CPNConditionEtcdPodReady,
			Status:             metav1.ConditionTrue,
			Reason:             controlplanev1alpha1.CPNReasonReady,
			LastTransitionTime: metav1.NewTime(before),
			ObservedGeneration: 1,
		}}

		conds := ComputeConditions(cpn, readyPods(controlplanev1alpha1.OperationComponentEtcd), nil)
		cond := meta.FindStatusCondition(conds, controlplanev1alpha1.CPNConditionEtcdPodReady)
		require.NotNil(t, cond)
		require.Equal(t, metav1.NewTime(before), cond.LastTransitionTime, "LastTransitionTime must be preserved")
		require.Equal(t, testGeneration, cond.ObservedGeneration, "ObservedGeneration must be refreshed")
	})

	t.Run("conditions of components removed from spec are pruned", func(t *testing.T) {
		cpn := baseCPN()
		// Arbiter node: only etcd in spec, but stale conditions exist for all components.
		cpn.Spec.Components.KubeAPIServer = controlplanev1alpha1.ComponentSpec{}
		cpn.Spec.Components.KubeControllerManager = controlplanev1alpha1.ComponentSpec{}
		cpn.Spec.Components.KubeScheduler = controlplanev1alpha1.ComponentSpec{}
		cpn.Status = inSyncStatus(farFuture)
		removed := []controlplanev1alpha1.OperationComponent{
			controlplanev1alpha1.OperationComponentKubeAPIServer,
			controlplanev1alpha1.OperationComponentKubeControllerManager,
			controlplanev1alpha1.OperationComponentKubeScheduler,
		}
		for _, c := range removed {
			for _, ct := range componentConditionTypes(c) {
				cpn.Status.Conditions = append(cpn.Status.Conditions, metav1.Condition{
					Type:               ct,
					Status:             metav1.ConditionTrue,
					Reason:             controlplanev1alpha1.CPNReasonReady,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-48 * time.Hour)),
					ObservedGeneration: 1,
				})
			}
		}
		cpn.Status.Conditions = append(cpn.Status.Conditions, metav1.Condition{
			Type: "SomeoneElsesCondition", Status: metav1.ConditionTrue, Reason: "Foreign",
		})

		conds := ComputeConditions(cpn, readyPods(controlplanev1alpha1.OperationComponentEtcd), nil)
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdPodReady,
			metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, "")
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdInSync,
			metav1.ConditionTrue, controlplanev1alpha1.CPNReasonInSync, "")
		for _, c := range removed {
			require.Nil(t, meta.FindStatusCondition(conds, podReadyType(c)))
			require.Nil(t, meta.FindStatusCondition(conds, inSyncType(c)))
			require.Nil(t, meta.FindStatusCondition(conds, readyType(c)))
		}
		require.NotNil(t, meta.FindStatusCondition(conds, "SomeoneElsesCondition"))
	})

	t.Run("foreign condition types are not pruned", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		foreign := metav1.Condition{
			Type:               "SomeoneElsesCondition",
			Status:             metav1.ConditionTrue,
			Reason:             "Foreign",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-48 * time.Hour)),
		}
		cpn.Status.Conditions = []metav1.Condition{foreign}

		conds := ComputeConditions(cpn, readyPods(allComponents()...), nil)
		got := meta.FindStatusCondition(conds, foreign.Type)
		require.NotNil(t, got, "foreign condition must be carried over")
		require.Equal(t, foreign, *got)
	})

	t.Run("terminating pod is not ready", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		pods := readyPods(controlplanev1alpha1.OperationComponentEtcd)
		pods[controlplanev1alpha1.OperationComponentEtcd].DeletionTimestamp = ptr.To(metav1.Now())

		conds := ComputeConditions(cpn, pods, nil)
		requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdPodReady,
			metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, "static pod etcd-master-0 is terminating")
	})

	t.Run("pod not in Running phase is not ready", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)

		for _, phase := range []corev1.PodPhase{corev1.PodPending, corev1.PodFailed} {
			pods := readyPods(controlplanev1alpha1.OperationComponentEtcd)
			pods[controlplanev1alpha1.OperationComponentEtcd].Status.Phase = phase
			conds := ComputeConditions(cpn, pods, nil)
			requireCondition(t, conds, controlplanev1alpha1.CPNConditionEtcdPodReady,
				metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, "static pod phase is "+string(phase))
		}
	})

	t.Run("last transition time bumped on status flip", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		before := time.Now().Add(-24 * time.Hour)
		cpn.Status.Conditions = []metav1.Condition{{
			Type:               controlplanev1alpha1.CPNConditionEtcdPodReady,
			Status:             metav1.ConditionTrue,
			Reason:             controlplanev1alpha1.CPNReasonReady,
			LastTransitionTime: metav1.NewTime(before),
		}}

		conds := ComputeConditions(cpn, nil, nil)
		cond := meta.FindStatusCondition(conds, controlplanev1alpha1.CPNConditionEtcdPodReady)
		require.NotNil(t, cond)
		require.Equal(t, metav1.ConditionFalse, cond.Status)
		require.True(t, cond.LastTransitionTime.After(before.Add(time.Hour)), "LastTransitionTime must be bumped")
	})

	t.Run("last transition time preserved when only the message changes", func(t *testing.T) {
		cpn := baseCPN()
		cpn.Status = inSyncStatus(farFuture)
		before := time.Now().Add(-24 * time.Hour)
		cpn.Status.Conditions = []metav1.Condition{{
			Type:               controlplanev1alpha1.CPNConditionEtcdPodReady,
			Status:             metav1.ConditionFalse,
			Reason:             controlplanev1alpha1.CPNReasonNotReady,
			Message:            "static pod etcd-master-0 is not ready",
			LastTransitionTime: metav1.NewTime(before),
		}}
		pods := map[controlplanev1alpha1.OperationComponent]*corev1.Pod{controlplanev1alpha1.OperationComponentEtcd: staticPod(controlplanev1alpha1.OperationComponentEtcd)}
		pods[controlplanev1alpha1.OperationComponentEtcd].Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name:  "etcd",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		}}

		conds := ComputeConditions(cpn, pods, nil)
		cond := meta.FindStatusCondition(conds, controlplanev1alpha1.CPNConditionEtcdPodReady)
		require.NotNil(t, cond)
		require.Equal(t, "static pod etcd-master-0 is in CrashLoopBackOff", cond.Message)
		require.Equal(t, metav1.NewTime(before), cond.LastTransitionTime)
	})
}

// opFor builds an operation for the component with the desired checksums matching the CPN spec.
func opFor(cpn *controlplanev1alpha1.ControlPlaneNode, component controlplanev1alpha1.OperationComponent, name string, approved bool, created time.Time) controlplanev1alpha1.ControlPlaneOperation {
	spec := cpn.Spec.Components.Component(component)
	return controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			NodeName:              cpn.Name,
			Component:             component,
			Steps:                 []controlplanev1alpha1.StepName{controlplanev1alpha1.StepSyncManifests},
			DesiredConfigChecksum: spec.Checksums.Config,
			DesiredPKIChecksum:    spec.Checksums.PKI,
			DesiredCAChecksum:     cpn.Spec.CAChecksum,
			Approved:              approved,
		},
	}
}

func TestComputeStatusReportCarriesConditionsUntouched(t *testing.T) {
	// Ownership contract guard: control-plane-node-controller must never write conditions.
	// Conditions set by the conditions controller (or anyone else) must survive the report.
	cpn := baseCPN()
	cpn.Status = inSyncStatus(time.Time{})
	cpn.Status.Conditions = []metav1.Condition{{
		Type:               controlplanev1alpha1.CPNConditionEtcdPodReady,
		Status:             metav1.ConditionFalse,
		Reason:             "ManualEdit",
		Message:            "must survive",
		LastTransitionTime: metav1.NewTime(time.Now()),
		ObservedGeneration: 42,
	}}

	report := ComputeStatusReport(cpn, nil)
	require.Equal(t, cpn.Status.Conditions, report.Conditions)
}
