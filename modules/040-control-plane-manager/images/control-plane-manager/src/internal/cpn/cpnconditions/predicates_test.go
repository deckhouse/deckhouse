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

package cpnconditions

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

func TestConditionInputsChangedPredicate(t *testing.T) {
	p := ConditionInputsChangedPredicate()

	newCPN := func(mutate func(*controlplanev1alpha1.ControlPlaneNode)) *controlplanev1alpha1.ControlPlaneNode {
		cpn := minimalCPN()
		if mutate != nil {
			mutate(cpn)
		}
		return cpn
	}

	t.Run("create fires", func(t *testing.T) {
		require.True(t, p.Create(event.CreateEvent{Object: minimalCPN()}))
	})

	t.Run("generation change fires", func(t *testing.T) {
		require.True(t, p.Update(event.UpdateEvent{
			ObjectOld: newCPN(nil),
			ObjectNew: newCPN(func(c *controlplanev1alpha1.ControlPlaneNode) { c.Generation = 3 }),
		}))
	})

	t.Run("status.components change fires", func(t *testing.T) {
		// control-plane-node-controller patches status.components after the CPO event:
		// CertificatesHealthy must be recomputed from the new status.
		require.True(t, p.Update(event.UpdateEvent{
			ObjectOld: newCPN(nil),
			ObjectNew: newCPN(func(c *controlplanev1alpha1.ControlPlaneNode) {
				c.Status.Components.Etcd.Checksums.CA = "ca-1"
			}),
		}))
	})

	t.Run("status.caChecksum-only change does not fire", func(t *testing.T) {
		// Not an input of the conditions: CertificatesHealthy reads the per-component CA.
		require.False(t, p.Update(event.UpdateEvent{
			ObjectOld: newCPN(nil),
			ObjectNew: newCPN(func(c *controlplanev1alpha1.ControlPlaneNode) { c.Status.CAChecksum = "ca-2" }),
		}))
	})

	t.Run("certificate expiration change fires", func(t *testing.T) {
		require.True(t, p.Update(event.UpdateEvent{
			ObjectOld: newCPN(nil),
			ObjectNew: newCPN(func(c *controlplanev1alpha1.ControlPlaneNode) {
				c.Status.Components.Etcd.CertificatesExpirationTime = map[string]metav1.Time{"etcd-server": metav1.Now()}
			}),
		}))
	})

	t.Run("cert observe time-only change does not fire", func(t *testing.T) {
		require.False(t, p.Update(event.UpdateEvent{
			ObjectOld: newCPN(nil),
			ObjectNew: newCPN(func(c *controlplanev1alpha1.ControlPlaneNode) {
				c.Status.Components.Etcd.LastCertObserveTime = metav1.Now()
			}),
		}))
	})

	t.Run("generic and wrong type do not fire", func(t *testing.T) {
		require.False(t, p.Generic(event.GenericEvent{Object: minimalCPN()}))
		require.False(t, p.Update(event.UpdateEvent{ObjectOld: readyEtcdPod(), ObjectNew: readyEtcdPod()}))
	})

	t.Run("conditions-only change does not fire", func(t *testing.T) {
		// status.conditions are this controller's own writes (self-loop).
		require.False(t, p.Update(event.UpdateEvent{
			ObjectOld: newCPN(nil),
			ObjectNew: newCPN(func(c *controlplanev1alpha1.ControlPlaneNode) {
				c.ResourceVersion = "42"
				c.Status.Conditions = []metav1.Condition{{
					Type:   controlplanev1alpha1.CPNConditionEtcdPodReady,
					Status: metav1.ConditionTrue,
					Reason: controlplanev1alpha1.CPNReasonReady,
				}}
			}),
		}))
	})

	t.Run("delete does not fire", func(t *testing.T) {
		require.False(t, p.Delete(event.DeleteEvent{Object: minimalCPN()}))
	})
}

func TestOperationPresenceChangedPredicate(t *testing.T) {
	p := OperationPresenceChangedPredicate()

	newOp := func(mutate func(*controlplanev1alpha1.ControlPlaneOperation)) *controlplanev1alpha1.ControlPlaneOperation {
		op := runningEtcdOperation()
		if mutate != nil {
			mutate(op)
		}
		return op
	}

	t.Run("create fires", func(t *testing.T) {
		// Creation of a pending operation must wake the controller: OperationInQueue must appear on time.
		op := runningEtcdOperation()
		op.Spec.Approved = false
		op.Status.Conditions = nil
		require.True(t, p.Create(event.CreateEvent{Object: op}))
	})

	t.Run("approval flip fires", func(t *testing.T) {
		// OperationInQueue -> OperationInFlight is a Spec.Approved change, not a Status change.
		require.True(t, p.Update(event.UpdateEvent{
			ObjectOld: newOp(func(op *controlplanev1alpha1.ControlPlaneOperation) { op.Spec.Approved = false }),
			ObjectNew: newOp(nil),
		}))
	})

	t.Run("transition to terminal fires", func(t *testing.T) {
		require.True(t, p.Update(event.UpdateEvent{
			ObjectOld: newOp(nil),
			ObjectNew: newOp(func(op *controlplanev1alpha1.ControlPlaneOperation) {
				op.Status.Conditions[0].Status = metav1.ConditionTrue
				op.Status.Conditions[0].Reason = controlplanev1alpha1.CPOReasonOperationCompleted
			}),
		}))
	})

	t.Run("transition to abandoned fires", func(t *testing.T) {
		require.True(t, p.Update(event.UpdateEvent{
			ObjectOld: newOp(nil),
			ObjectNew: newOp(func(op *controlplanev1alpha1.ControlPlaneOperation) {
				op.Status.Conditions[0].Reason = controlplanev1alpha1.CPOReasonOperationAbandoned
			}),
		}))
	})

	t.Run("generic and wrong type do not fire", func(t *testing.T) {
		require.False(t, p.Generic(event.GenericEvent{Object: runningEtcdOperation()}))
		require.False(t, p.Update(event.UpdateEvent{ObjectOld: minimalCPN(), ObjectNew: minimalCPN()}))
	})

	t.Run("non-terminal status change does not fire", func(t *testing.T) {
		// Step progress does not change operation presence.
		require.False(t, p.Update(event.UpdateEvent{
			ObjectOld: newOp(nil),
			ObjectNew: newOp(func(op *controlplanev1alpha1.ControlPlaneOperation) {
				op.Status.Conditions = append(op.Status.Conditions, metav1.Condition{
					Type:   string(controlplanev1alpha1.StepSyncManifests),
					Status: metav1.ConditionFalse,
					Reason: "InProgress",
				})
			}),
		}))
	})

	t.Run("no relevant change does not fire", func(t *testing.T) {
		require.False(t, p.Update(event.UpdateEvent{
			ObjectOld: newOp(nil),
			ObjectNew: newOp(func(op *controlplanev1alpha1.ControlPlaneOperation) {
				op.ResourceVersion = "42"
				op.Spec.DesiredConfigChecksum = "cfg-changed" // presence does not depend on checksums
			}),
		}))
	})

	t.Run("delete fires", func(t *testing.T) {
		// Manual removal or rotation of an active operation must retract the condition.
		require.True(t, p.Delete(event.DeleteEvent{Object: runningEtcdOperation()}))
	})
}

func TestControlPlanePodPredicate(t *testing.T) {
	p := ControlPlanePodPredicate(testNodeName)

	t.Run("pod of this node fires", func(t *testing.T) {
		require.True(t, p.Create(event.CreateEvent{Object: readyEtcdPod()}))
	})

	t.Run("mirror pod of another node does not fire", func(t *testing.T) {
		pod := readyEtcdPod()
		pod.Name, pod.Spec.NodeName = "etcd-master-1", "master-1"
		require.False(t, p.Create(event.CreateEvent{Object: pod}))
	})

	t.Run("pod with a component label but another name does not fire", func(t *testing.T) {
		// Matching is by mirror pod name, the label is not trusted.
		oldPod, newPod := readyEtcdPod(), readyEtcdPod()
		oldPod.Name, newPod.Name = "etcd-backup-"+testNodeName, "etcd-backup-"+testNodeName
		newPod.Status.Conditions[0].Status = corev1.ConditionFalse
		require.False(t, p.Update(event.UpdateEvent{ObjectOld: oldPod, ObjectNew: newPod}))
	})

	t.Run("pod in another namespace does not fire", func(t *testing.T) {
		pod := readyEtcdPod()
		pod.Namespace = "default"
		require.False(t, p.Create(event.CreateEvent{Object: pod}))
	})

	t.Run("generic does not fire", func(t *testing.T) {
		require.False(t, p.Generic(event.GenericEvent{Object: readyEtcdPod()}))
	})

	t.Run("update without readiness change does not fire", func(t *testing.T) {
		newPod := readyEtcdPod()
		newPod.ResourceVersion = "42"
		newPod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "etcd", RestartCount: 1}}
		require.False(t, p.Update(event.UpdateEvent{ObjectOld: readyEtcdPod(), ObjectNew: newPod}))
	})

	t.Run("readiness change fires", func(t *testing.T) {
		newPod := readyEtcdPod()
		newPod.Status.Conditions[0].Status = corev1.ConditionFalse
		require.True(t, p.Update(event.UpdateEvent{ObjectOld: readyEtcdPod(), ObjectNew: newPod}))
	})

	t.Run("crash loop fires", func(t *testing.T) {
		newPod := readyEtcdPod()
		newPod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name:  "etcd",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		}}
		require.True(t, p.Update(event.UpdateEvent{ObjectOld: readyEtcdPod(), ObjectNew: newPod}))
	})

	t.Run("termination fires", func(t *testing.T) {
		newPod := readyEtcdPod()
		newPod.DeletionTimestamp = ptr.To(metav1.Now())
		require.True(t, p.Update(event.UpdateEvent{ObjectOld: readyEtcdPod(), ObjectNew: newPod}))
	})

	t.Run("update of another node pod does not fire", func(t *testing.T) {
		oldPod, newPod := readyEtcdPod(), readyEtcdPod()
		oldPod.Name, newPod.Name = "etcd-master-1", "etcd-master-1"
		newPod.Status.Conditions[0].Status = corev1.ConditionFalse
		require.False(t, p.Update(event.UpdateEvent{ObjectOld: oldPod, ObjectNew: newPod}))
	})

	t.Run("delete of this node pod fires", func(t *testing.T) {
		// Static pods are recreated on node restart: deletion flips the Ready condition.
		require.True(t, p.Delete(event.DeleteEvent{Object: readyEtcdPod()}))
	})
}
