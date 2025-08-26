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
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"
)

// ProviderClusterConfigurationAffinityReconciler reconciles a ProviderClusterConfigurationAffinity object
type ProviderClusterConfigurationAffinityReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	SecretName      string
	SecretNamespace string
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=providerclusterconfigurationaffinities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deckhouse.io,resources=providerclusterconfigurationaffinities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deckhouse.io,resources=providerclusterconfigurationaffinities/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ProviderClusterConfigurationAffinity object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *ProviderClusterConfigurationAffinityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	// TODO(user): your logic here

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ProviderClusterConfigurationAffinityReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		Named("providerclusterconfigurationaffinity").
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return e.Object.GetName() == r.SecretName && e.Object.GetNamespace() == r.SecretNamespace
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.ObjectNew.GetNamespace() == r.SecretNamespace && e.ObjectNew.GetName() == r.SecretName
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return e.Object.GetName() == r.SecretName && e.Object.GetNamespace() == r.SecretNamespace
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return e.Object.GetName() == r.SecretName && e.Object.GetNamespace() == r.SecretNamespace
			},
		}).
		Complete(r)
}
