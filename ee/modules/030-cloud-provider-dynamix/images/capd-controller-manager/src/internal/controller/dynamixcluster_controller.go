/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1alpha1 "github.com/deckhouse/deckhouse/api/v1alpha1"
	"github.com/deckhouse/deckhouse/internal/scopes"
)

// DynamixClusterReconciler reconciles a DynamixCluster object
type DynamixClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=dynamixclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=dynamixclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=dynamixclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DynamixCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *DynamixClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	dynamixCluster := &infrastructurev1alpha1.DynamixCluster{}
	err := r.Get(ctx, req.NamespacedName, dynamixCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cluster, err := util.GetOwnerCluster(ctx, r.Client, dynamixCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		logger.Info("Cluster Controller has not yet set OwnerRef")

		return ctrl.Result{}, nil
	}

	newScope, err := scopes.NewScope(r.Client, r.Config, ctrl.LoggerFrom(ctx))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create a scope: %w", err)
	}

	clusterScope, err := scopes.NewClusterScope(newScope, cluster, dynamixCluster)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create a cluster scope: %w", err)
	}

	// Handle deleted cluster
	if !dynamixCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, clusterScope)
}

func (r *DynamixClusterReconciler) reconcile(
	ctx context.Context,
	clusterScope *scopes.ClusterScope,
) (ctrl.Result, error) {
	controlPlaneEndpointURL, err := url.Parse(r.Config.Host)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse api server host: %w", err)
	}

	port, err := strconv.ParseInt(controlPlaneEndpointURL.Port(), 10, 32)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse api server port: %w", err)
	}

	clusterScope.DynamixCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: controlPlaneEndpointURL.Hostname(),
		Port: int32(port),
	}

	clusterScope.DynamixCluster.Status.Ready = true

	err = clusterScope.Patch(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch DynamixCluster: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DynamixClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.DynamixCluster{}).
		Complete(r)
}
