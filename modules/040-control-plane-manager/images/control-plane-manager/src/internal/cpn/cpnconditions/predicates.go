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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/pkg/podstatus"
)

// ConditionInputsChangedPredicate fires on creation, spec changes and status.components changes.
// status.components is watched because control-plane-node-controller patches it after the CPO
// event, so the CPO event alone may run against the old status.
func ConditionInputsChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCPN, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneNode)
			newCPN, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneNode)
			if !okOld || !okNew {
				return false
			}
			return oldCPN.Generation != newCPN.Generation ||
				!equality.Semantic.DeepEqual(conditionInputsOf(oldCPN.Status.Components), conditionInputsOf(newCPN.Status.Components))
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// conditionInputsOf drops LastCertObserveTime: it is bumped on every cert observation but is
// not an input of the conditions.
func conditionInputsOf(s controlplanev1alpha1.ComponentsStatus) controlplanev1alpha1.ComponentsStatus {
	for c := range controlplanev1alpha1.ComponentRegistry() {
		s.Component(c).LastCertObserveTime = metav1.Time{}
	}
	return s
}

// OperationPresenceChangedPredicate fires when an operation appears, disappears, gets approved
// (OperationInQueue -> OperationInFlight is a spec change) or becomes terminal.
func OperationPresenceChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, ok := e.Object.(*controlplanev1alpha1.ControlPlaneOperation)
			return ok
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldOp, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
			newOp, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)
			if !okOld || !okNew {
				return false
			}
			return oldOp.Spec.Approved != newOp.Spec.Approved ||
				oldOp.IsTerminal() != newOp.IsTerminal()
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// ControlPlanePodPredicate fires for the node's control plane mirror pods (matched by name,
// "<component>-<node>"); updates only when an input of the Ready condition changed.
func ControlPlanePodPredicate(nodeName string) predicate.Predicate {
	names := make(map[string]bool, len(controlplanev1alpha1.ComponentRegistry()))
	for component := range controlplanev1alpha1.ComponentRegistry() {
		names[component.MirrorPodName(nodeName)] = true
	}
	isNodeControlPlanePod := func(obj client.Object) bool {
		return obj != nil && obj.GetNamespace() == constants.KubeSystemNamespace && names[obj.GetName()]
	}
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isNodeControlPlanePod(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !isNodeControlPlanePod(e.ObjectNew) {
				return false
			}
			oldPod, okOld := e.ObjectOld.(*corev1.Pod)
			newPod, okNew := e.ObjectNew.(*corev1.Pod)
			if !okOld || !okNew {
				return false
			}
			return podReadinessInputsOf(oldPod) != podReadinessInputsOf(newPod)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isNodeControlPlanePod(e.Object)
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// podReadinessInputs are the pod fields the <Component>PodReady condition is computed from.
type podReadinessInputs struct {
	phase       corev1.PodPhase
	ready       bool
	crashLoop   bool
	terminating bool
}

func podReadinessInputsOf(pod *corev1.Pod) podReadinessInputs {
	return podReadinessInputs{
		phase:       pod.Status.Phase,
		ready:       podstatus.IsReady(pod),
		crashLoop:   podstatus.IsCrashLooping(pod),
		terminating: pod.DeletionTimestamp != nil,
	}
}
