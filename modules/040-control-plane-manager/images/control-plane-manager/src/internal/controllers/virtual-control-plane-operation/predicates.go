package virtualcontrolplaneoperation

import (
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func approvedOperationPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldOp, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
			newOp, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)
			return okOld && okNew && !oldOp.Spec.Approved && newOp.Spec.Approved
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}
