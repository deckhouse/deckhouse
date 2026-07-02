/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

//nolint:goimports
//nolint:gci
import (
	"context"
	"fmt"
	"slices"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/pkg/log"

	"integrity-containerd-configurator/internal/configwriter"
)

// Reconciler watches ContainerdIntegrityPolicy resources and writes containerd config on the node.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Writer *configwriter.Writer
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=containerdintegritypolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=deckhouse.io,resources=containerdintegritypolicies/status,verbs=get

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.Default().With("reconcile", req.NamespacedName)

	policyList := &deckhousev1alpha1.ContainerdIntegrityPolicyList{}
	if err := r.List(ctx, policyList); err != nil {
		return reconcile.Result{}, fmt.Errorf("list ContainerdIntegrityPolicies: %w", err)
	}

	desired := configwriter.AggregatePolicies(logger, policyList.Items)

	if err := r.Writer.Apply(logger, desired); err != nil {
		return reconcile.Result{}, fmt.Errorf("apply containerd integrity config: %w", err)
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deckhousev1alpha1.ContainerdIntegrityPolicy{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(event.CreateEvent) bool {
				return true
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldPolicy, okOld := e.ObjectOld.(*deckhousev1alpha1.ContainerdIntegrityPolicy)
				newPolicy, okNew := e.ObjectNew.(*deckhousev1alpha1.ContainerdIntegrityPolicy)
				if !okOld || !okNew {
					return true
				}

				if oldPolicy.Spec.CA != newPolicy.Spec.CA {
					return true
				}

				return !slices.Equal(oldPolicy.Status.ProtectedNamespaces, newPolicy.Status.ProtectedNamespaces)
			},
			DeleteFunc: func(event.DeleteEvent) bool {
				return true
			},
			GenericFunc: func(event.GenericEvent) bool {
				return true
			},
		}).
		Complete(r)
}
