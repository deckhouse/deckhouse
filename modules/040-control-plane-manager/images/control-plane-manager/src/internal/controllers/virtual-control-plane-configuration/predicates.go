package virtualcontrolplaneconfiguration

import (
	"context"
	"control-plane-manager/internal/constants"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func secretPredicate() predicate.Predicate {
	isTarget := func(o client.Object) bool {
		return o.GetNamespace() == constants.KubeSystemNamespace && o.GetName() == constants.VirtualControlPlaneConfigSecretName
	}

	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return isTarget(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return isTarget(e.ObjectNew) },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

func (r *reconciler) mapConfigSecretToVirtualControlPlanes(ctx context.Context, _ client.Object) []reconcile.Request {
	vcpList := &controlplanev1alpha1.VirtualControlPlaneList{}
	if err := r.client.List(ctx, vcpList); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(vcpList.Items))
	for i := range vcpList.Items {
		vcp := &vcpList.Items[i]
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKey{Name: vcp.Name},
		})
	}

	return requests
}
