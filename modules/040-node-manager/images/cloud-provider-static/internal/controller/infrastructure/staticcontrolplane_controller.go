/*
Copyright 2023.

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

package controller

import (
	infrav1 "cloud-provider-static/api/infrastructure/v1alpha1"
	"context"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// StaticControlPlaneReconciler reconciles a StaticControlPlane object
type StaticControlPlaneReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticcontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticcontrolplanes/finalizers,verbs=update

//+kubebuilder:rbac:groups=core,resources=serviceaccounts/token,verbs=create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StaticControlPlane object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *StaticControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling StaticControlPlane")

	staticControlPlane := &infrav1.StaticControlPlane{}
	err := r.Get(ctx, req.NamespacedName, staticControlPlane)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, staticControlPlane.ObjectMeta)
	if err != nil {
		logger.Info("StaticControlPlane has missing cluster label or cluster does not exist")

		return ctrl.Result{}, nil
	}

	// Handle deleted control plane
	if !staticControlPlane.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, cluster, staticControlPlane)
}

func (r *StaticControlPlaneReconciler) reconcile(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	staticControlPlane *infrav1.StaticControlPlane,
) (ctrl.Result, error) {
	patchHelper, err := patch.NewHelper(staticControlPlane, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}

	staticControlPlane.Status.Initialized = true
	staticControlPlane.Status.Ready = true
	staticControlPlane.Status.ExternalManagedControlPlane = true

	err = patchHelper.Patch(ctx, staticControlPlane)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch static control plane")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StaticControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.StaticControlPlane{}).
		Complete(r)
}
