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
	capiutil "sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1alpha1 "cluster-api-provider-yandex/api/v1alpha1"
)

type YandexClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

func (r *YandexClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling YandexCluster")

	yandexCluster := &infrastructurev1alpha1.YandexCluster{}
	if err := r.Get(ctx, req.NamespacedName, yandexCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cluster, err := capiutil.GetOwnerCluster(ctx, r.Client, yandexCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		logger.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	if !yandexCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	patchHelper, err := patch.NewHelper(yandexCluster, r.Client)
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

	ready := true
	yandexCluster.Status.Initialization.Provisioned = &ready
	yandexCluster.Status.Ready = true
	yandexCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: controlPlaneEndpointURL.Hostname(),
		Port: int32(port),
	}

	if err := patchHelper.Patch(ctx, yandexCluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch YandexCluster: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *YandexClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.YandexCluster{}).
		Named("yandexcluster").
		Complete(r)
}
