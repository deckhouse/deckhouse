package controller

import (
	"context"
	"errors"
	"fmt"
	"time"
	"update-observer/internal/constant"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueInterval         = 3 * time.Minute
	nodeListPageSize        = 50
)

type reconciler struct {
	client                 client.Client
	apiServerVersionGetter ApiServerVersionGetter
}

func RegisterController(manager manager.Manager) error {
	manager.GetConfig()
	inClusterVersionGetter, err := newInClusterVersionGetter(manager.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create kube client: %w", err)
	}

	r := &reconciler{
		client:                 manager.GetClient(),
		apiServerVersionGetter: inClusterVersionGetter,
	}

	return ctrl.NewControllerManagedBy(manager).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(true),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named(constant.ControllerName).
		Watches(
			&corev1.Secret{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(
				getSecretPredicate(),
			),
		).
		Complete(r)
}

func getSecretPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			secret, ok := e.Object.(*corev1.Secret)
			if !ok {
				return false
			}
			return secret.Name == constant.SecretName
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			secret, ok := e.ObjectNew.(*corev1.Secret)
			if !ok {
				return false
			}
			return secret.Name == constant.SecretName
		},

		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},

		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	// TODO: log reconcile started
	clusterCfg, err := r.getClusterConfiguration(ctx)
	if err != nil {
		// TODO: log error
		return reconcile.Result{}, nil
	}

	nodesStatus, err := r.collectNodesUpdateStatus(ctx, "") // TODO desiredVersion
	if err != nil {
		// TODO: log error
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}
	println(nodesStatus)

	controlPlaneStatus, err := r.collectControlPlaneUpdateStatus(ctx, "")
	if err != nil {
		// TODO: log error
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}
	println(controlPlaneStatus)

	// ----------------------------------------------------------------------

	desiredVersion := clusterCfg.KubernetesVersion

	// 2. Получаем или создаём d8-cluster-kubernetes
	cm := &corev1.ConfigMap{}
	err = r.client.Get(ctx, client.ObjectKey{
		Name:      constant.ConfigMapName,
		Namespace: constant.KubeSystemNamespace,
	}, cm)

	if client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}

	if err != nil {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constant.ConfigMapName,
				Namespace: constant.KubeSystemNamespace,
				Labels: map[string]string{
					"heritage": "deckhouse",
				},
			},
			Data: map[string]string{},
		}
	}

	spec := map[string]string{
		"desiredVersion": desiredVersion,
		"updateMode":     "", // TODO
	}

	specBytes, _ := yaml.Marshal(spec)
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data["spec"] = string(specBytes)

	if cm.ResourceVersion == "" {
		return reconcile.Result{}, r.client.Create(ctx, cm)
	}

	return reconcile.Result{}, r.client.Update(ctx, cm)
}

type ClusterConfiguration struct {
	KubernetesVersion string `yaml:"kubernetesVersion"`
	UpdateMode        string
}

func (r *reconciler) getClusterConfiguration(ctx context.Context) (ClusterConfiguration, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      constant.SecretName,
		Namespace: constant.KubeSystemNamespace,
	}, secret)
	if err != nil {
		return ClusterConfiguration{}, client.IgnoreNotFound(err)
	}

	rawCfg, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return ClusterConfiguration{}, errors.New("'cluster-configuration.yaml' not found")
	}

	var clusterCfg ClusterConfiguration
	if err := yaml.Unmarshal(rawCfg, &clusterCfg); err != nil {
		return ClusterConfiguration{}, fmt.Errorf("failed to unmarshal cluster-configuration: %w", err)
	}

	if clusterCfg.KubernetesVersion == "Automatic" {
		clusterCfg.UpdateMode = "Automatic"
	}

	return clusterCfg, nil
}

type NodesUpdateStatus struct {
	DesiredCount  int `yaml:"desiredCount"`
	UpToDateCount int `yaml:"upToDateCount"`
}

func (r *reconciler) collectNodesUpdateStatus(ctx context.Context, desiredVersion string) (NodesUpdateStatus, error) {
	var (
		continueToken string
		res           NodesUpdateStatus
	)

	for {
		list := &corev1.NodeList{}
		err := r.client.List(ctx, list, &client.ListOptions{
			Limit:    nodeListPageSize,
			Continue: continueToken,
		})
		if err != nil {
			return NodesUpdateStatus{}, err
		}

		for _, node := range list.Items {
			res.DesiredCount++
			v := node.Status.NodeInfo.KubeletVersion
			if v == desiredVersion {
				res.UpToDateCount++
			}
		}

		if list.Continue == "" {
			break
		}
		continueToken = list.Continue
	}

	return res, nil
}

type ControlPlaneUpdateStatus struct {
	DesiredCount  int `yaml:"desiredCount"`
	UpToDateCount int `yaml:"upToDateCount"`
}

func (r *reconciler) collectControlPlaneUpdateStatus(ctx context.Context, desiredVersion string) (ControlPlaneUpdateStatus, error) {
	var res ControlPlaneUpdateStatus

	podList := &corev1.PodList{}
	err := r.client.List(
		ctx,
		podList,
		client.InNamespace(constant.KubeSystemNamespace),
		client.MatchingLabels{
			"component": "kube-apiserver",
		},
	)
	if err != nil {
		return ControlPlaneUpdateStatus{}, fmt.Errorf("failed to fetch pod list: %w", err)
	}

	res.DesiredCount = len(podList.Items)

	for _, pod := range podList.Items {
		if pod.Status.PodIP == "" {
			continue
		}

		v, err := r.apiServerVersionGetter.Get(ctx, pod.Status.PodIP)
		if err != nil {
			// TODO log
			//klog.Infof()
			// r.log.V(1).Info(
			// 	"failed to get kube-apiserver version",
			// 	"pod", pod.Name,
			// 	"err", err,
			// )
			continue
		}

		if v == desiredVersion {
			res.UpToDateCount++
		}
	}

	return res, nil
}
