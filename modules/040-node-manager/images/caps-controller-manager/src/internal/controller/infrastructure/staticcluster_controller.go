/*
Copyright 2023 Flant JSC

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
	"context"
	"net/url"
	"strconv"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/scope"
)

// StaticClusterReconciler reconciles a StaticCluster object
type StaticClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticclusters,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=staticclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StaticCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *StaticClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling StaticCluster")

	staticCluster := &infrav1.StaticCluster{}
	err := r.Get(ctx, req.NamespacedName, staticCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, staticCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		logger.Info("Cluster Controller has not yet set OwnerRef")

		return ctrl.Result{}, nil
	}

	newScope, err := scope.NewScope(r.Client, r.Config, ctrl.LoggerFrom(ctx))
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create a scope")
	}

	clusterScope, err := scope.NewClusterScope(newScope, cluster, staticCluster)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create a cluster scope")
	}

	// Handle deleted cluster
	if !staticCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, clusterScope)
}

func (r *StaticClusterReconciler) reconcile(
	ctx context.Context,
	clusterScope *scope.ClusterScope,
) (ctrl.Result, error) {
	controlPlaneEndpointURL, err := url.Parse(r.Config.Host)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to parse api server host")
	}

	port, err := strconv.Atoi(controlPlaneEndpointURL.Port())
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to parse api server port")
	}

	clusterScope.StaticCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: controlPlaneEndpointURL.Hostname(),
		Port: int32(port),
	}

	clusterScope.StaticCluster.Status.Ready = true

	err = clusterScope.Patch(ctx)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch StaticCluster")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StaticClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.StaticCluster{}).
		Complete(r)
}
