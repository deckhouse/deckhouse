/*
Copyright 2026 Flant JSC

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
	"errors"
	"fmt"
	"strings"
	"time"
	"update-observer/constant"

	"github.com/go-logr/logr"
	"golang.org/x/mod/semver"
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
	log                    logr.Logger
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
		log:                    manager.GetLogger(),
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
	r.log.Info("reconcile started", "request", req.NamespacedName)

	clusterCfg, err := r.getClusterConfiguration(ctx)
	if err != nil {
		r.log.Error(err, "failed to get cluster configuration")
		return reconcile.Result{}, nil
	}

	nodesStatus, err := r.getNodesStatus(ctx, clusterCfg.DesiredVersion)
	if err != nil {
		r.log.Error(err, "failed to get nodes status")
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	cpStatus, err := r.getControlPlaneStatus(ctx, clusterCfg.DesiredVersion)
	if err != nil {
		r.log.Error(err, "failed to get control plane status")
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	cm := &corev1.ConfigMap{}
	err = r.client.Get(ctx, client.ObjectKey{
		Name:      constant.ConfigMapName,
		Namespace: constant.KubeSystemNamespace,
	}, cm)

	if client.IgnoreNotFound(err) != nil {
		r.log.Error(err, "failed to get configMap")
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
		"desiredVersion": clusterCfg.DesiredVersion,
		"updateMode":     clusterCfg.UpdateMode,
	}

	specBytes, _ := yaml.Marshal(spec)
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}

	cm.Data["spec"] = string(specBytes)

	status := Status{
		ControlPlane: cpStatus,
		Nodes:        nodesStatus,
	}

	switch {
	case clusterCfg.UpdateMode == "Automatic" &&
		cpStatus.CurrentVersion != "" &&
		semver.Compare(cpStatus.CurrentVersion, clusterCfg.DesiredVersion) == 1:
		status.Phase = "VersionDrift"
		status.ControlPlane.State = "VersionDrift"
	case cpStatus.UpToDateCount < cpStatus.DesiredCount:
		status.Phase = "ControlPlaneUpdating"
		status.ControlPlane.State = "Updating"
	case nodesStatus.UpToDateCount < nodesStatus.DesiredCount:
		status.Phase = "NodesUpdating"
		status.ControlPlane.State = "UpToDate"
		status.ControlPlane.Progress = "100%"
	default:
		status.Phase = "UpToDate"
		status.ControlPlane.State = "UpToDate"
		status.ControlPlane.Progress = "100%"
	}

	statusBytes, err := yaml.Marshal(status)
	if err != nil {
		r.log.Error(err, "failed to marshal status")
		return reconcile.Result{}, err
	}

	cm.Data["status"] = string(statusBytes)

	if cm.ResourceVersion == "" {
		err = r.client.Create(ctx, cm)
		r.log.Error(err, "failed to create configmap")
	} else {
		err = r.client.Update(ctx, cm)
		r.log.Error(err, "failed to update configmap")
	}

	if status.Phase != "UpToDate" {
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) getClusterConfiguration(ctx context.Context) (*ClusterConfiguration, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      constant.SecretName,
		Namespace: constant.KubeSystemNamespace,
	}, secret)
	if err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	rawCfg, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return nil, errors.New("'cluster-configuration.yaml' not found")
	}

	var clusterCfg *ClusterConfiguration
	if err := yaml.Unmarshal(rawCfg, &clusterCfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster-configuration: %w", err)
	}

	if clusterCfg.KubernetesVersion == "Automatic" {
		clusterCfg.UpdateMode = "Automatic"

		rawDefault, ok := secret.Data["deckhouseDefaultKubernetesVersion"]
		if !ok {
			return nil, fmt.Errorf("deckhouseDefaultKubernetesVersion not found in secret")
		}

		clusterCfg.DesiredVersion = strings.TrimSpace(string(rawDefault))
		if clusterCfg.DesiredVersion == "" {
			return nil, fmt.Errorf("deckhouseDefaultKubernetesVersion is empty")
		}
	} else {
		clusterCfg.UpdateMode = "Manual"
		clusterCfg.DesiredVersion = clusterCfg.KubernetesVersion
	}

	return clusterCfg, nil
}

func (r *reconciler) getNodesStatus(ctx context.Context, desiredVersion string) (NodesStatus, error) {
	var (
		continueToken string
		res           NodesStatus
	)

	for {
		list := &corev1.NodeList{}
		err := r.client.List(ctx, list, &client.ListOptions{
			Limit:    nodeListPageSize,
			Continue: continueToken,
		})
		if err != nil {
			return NodesStatus{}, err
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

func (r *reconciler) getControlPlaneStatus(ctx context.Context, desiredVersion string) (ControlPlaneStatus, error) {
	var res ControlPlaneStatus

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
		return ControlPlaneStatus{}, fmt.Errorf("failed to fetch pod list: %w", err)
	}

	res.DesiredCount = len(podList.Items)

	for _, pod := range podList.Items {
		if pod.Status.PodIP == "" {
			continue
		}

		v, err := r.apiServerVersionGetter.Get(ctx, pod.Status.PodIP)
		if err != nil {
			r.log.Error(
				err,
				"failed to get kube-apiserver version", "pod", pod.Name)
			continue
		}

		if v == desiredVersion {
			res.UpToDateCount++
		}

		if res.CurrentVersion == "" || semver.Compare(res.CurrentVersion, v) == 1 {
			res.CurrentVersion = v
		}
	}

	if res.DesiredCount == 0 {
		res.Progress = "0%"
	}
	p := (res.UpToDateCount * 100) / res.DesiredCount
	res.Progress = fmt.Sprint(min(p, 100), "%")

	return res, nil
}
