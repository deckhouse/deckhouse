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

// approvedCPOPredicate triggers on CPO that become Approved.
func approvedCPOPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			op, ok := e.Object.(*controlplanev1alpha1.ControlPlaneOperation)
			return ok && op.Spec.Approved
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
