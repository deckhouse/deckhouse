/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	deckhouseiov1alpha1 "infra-controller-manager/api/v1alpha1"
)

// VCDAffinityRuleReconciler reconciles a VCDAffinityRule object
type VCDAffinityRuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=vcdaffinityrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deckhouse.io,resources=vcdaffinityrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deckhouse.io,resources=vcdaffinityrules/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VCDAffinityRule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *VCDAffinityRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VCDAffinityRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deckhouseiov1alpha1.VCDAffinityRule{}).
		Named("vcdaffinityrule").
		Complete(r)
}
