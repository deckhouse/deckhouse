/*
Copyright 2025 Flant JSC

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
	"fmt"
	"net/url"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1a1 "cluster-api-provider-dvp/api/v1alpha1"
	dvpapi "dvp-common/api"
)

// DeckhouseClusterReconciler reconciles a DeckhouseCluster object
type DeckhouseClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
	DVP    *dvpapi.DVPCloudAPI
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=deckhouseclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=deckhouseclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=deckhouseclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *DeckhouseClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling DeckhouseCluster")

	dvpCluster := &infrastructurev1a1.DeckhouseCluster{}
	err := r.Get(ctx, req.NamespacedName, dvpCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cluster, err := util.GetOwnerCluster(ctx, r.Client, dvpCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		logger.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Handle deleted cluster
	if !dvpCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, dvpCluster)
}

func (r *DeckhouseClusterReconciler) reconcile(
	ctx context.Context,
	dvpCluster *infrastructurev1a1.DeckhouseCluster,
) (ctrl.Result, error) {
	patchHelper, err := patch.NewHelper(dvpCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	controlPlaneEndpointURL, err := url.Parse(r.Config.Host)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse api server host: %w", err)
	}

	port, err := strconv.ParseInt(controlPlaneEndpointURL.Port(), 10, 32)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse api server port: %w", err)
	}

	dvpCluster.Status.Ready = true
	dvpCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: controlPlaneEndpointURL.Hostname(),
		Port: int32(port),
	}

	if err = patchHelper.Patch(ctx, dvpCluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch DeckhouseCluster: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeckhouseClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1a1.DeckhouseCluster{}).
		Named("deckhousecluster").
		Complete(r)
}
