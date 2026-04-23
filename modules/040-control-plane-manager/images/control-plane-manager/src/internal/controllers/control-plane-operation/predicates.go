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

package controlplaneoperation

import (
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// approvedCPOPredicate triggers on CPO create and on approval transition.
func approvedCPOPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, ok := e.Object.(*controlplanev1alpha1.ControlPlaneOperation)
			return ok
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldOp, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
			newOp, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)
			return okOld && okNew && !oldOp.Spec.Approved && newOp.Spec.Approved
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// controlPlanePodPredicate triggers on pod condition/annotation changes for control-plane pods on given node.
func controlPlanePodPredicate(nodeName string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isNodeControlPlanePod(e.Object, nodeName)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !isNodeControlPlanePod(e.ObjectNew, nodeName) {
				return false
			}
			oldPod, okOld := e.ObjectOld.(*corev1.Pod)
			newPod, okNew := e.ObjectNew.(*corev1.Pod)
			if !okOld || !okNew {
				return false
			}
			return !reflect.DeepEqual(oldPod.Status.Conditions, newPod.Status.Conditions) ||
				oldPod.Annotations[constants.ConfigChecksumAnnotationKey] != newPod.Annotations[constants.ConfigChecksumAnnotationKey] ||
				oldPod.Annotations[constants.PKIChecksumAnnotationKey] != newPod.Annotations[constants.PKIChecksumAnnotationKey] ||
				oldPod.Annotations[constants.CAChecksumAnnotationKey] != newPod.Annotations[constants.CAChecksumAnnotationKey]
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

func isNodeControlPlanePod(obj client.Object, nodeName string) bool {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return false
	}
	if pod.Namespace != constants.KubeSystemNamespace {
		return false
	}
	if pod.Spec.NodeName != nodeName {
		return false
	}
	component := pod.Labels[constants.StaticPodComponentLabelKey]
	_, known := controlplanev1alpha1.OperationComponentFromPodName(component)
	return known
}
