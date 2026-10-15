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

package controlplaneapprover

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

func OperationPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldOperation, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
			newOperation, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)

			if !okOld || !okNew {
				return false
			}

			return !oldOperation.IsTerminal() && newOperation.IsTerminal()
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// ControlPlaneNodePredicate reacts when a node reports new applied checksums: the leader's
// operation may be waiting for that node to be updated (see markWaitingLeaders).
func ControlPlaneNodePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNode, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneNode)
			newNode, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneNode)
			if !okOld || !okNew {
				return false
			}
			return appliedChecksums(oldNode) != appliedChecksums(newNode)
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// ControlPlanePodPredicate reacts when a control-plane-manager Pod or a component's static pod starts
// or stops running an image: an operation may be waiting for its node to hold it (see
// holdBackUnpulledImages).
func ControlPlanePodPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPod, okOld := e.ObjectOld.(*corev1.Pod)
			newPod, okNew := e.ObjectNew.(*corev1.Pod)
			if !okOld || !okNew || !isControlPlanePod(newPod) {
				return false
			}
			return !maps.Equal(runningImages(oldPod), runningImages(newPod))
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(event.GenericEvent) bool {
			return false
		},
	}
}

// isControlPlanePod reports whether pod is a control-plane-manager Pod or a component's static pod.
func isControlPlanePod(pod *corev1.Pod) bool {
	component := pod.Labels[constants.StaticPodComponentLabelKey]
	_, isStaticPod := controlplanev1alpha1.OperationComponentFromPodName(component)
	return isStaticPod || component == constants.ControlPlaneManagerComponentLabelValue
}

func appliedChecksums(cpn *controlplanev1alpha1.ControlPlaneNode) [4]controlplanev1alpha1.Checksums {
	components := cpn.Status.Components
	return [4]controlplanev1alpha1.Checksums{
		components.Etcd.Checksums,
		components.KubeAPIServer.Checksums,
		components.KubeControllerManager.Checksums,
		components.KubeScheduler.Checksums,
	}
}
