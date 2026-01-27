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
	"fmt"
	"time"
	"update-observer/cluster"
	"update-observer/common"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *reconciler) getClusterState(ctx context.Context, cfg *cluster.Configuration, downgradeInProgress bool) (*cluster.State, error) {
	nodesState, err := r.getNodesState(ctx, cfg.DesiredVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes state: %w", err)
	}

	controlPlaneState, err := r.getControlPlaneState(ctx, cfg.DesiredVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane state: %w", err)
	}

	return cluster.GetState(cfg, nodesState, controlPlaneState, downgradeInProgress), nil
}

func (r *reconciler) getClusterConfiguration(ctx context.Context) (*cluster.Configuration, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      common.SecretName,
		Namespace: common.KubeSystemNamespace,
	}, secret)
	if err != nil {
		return nil, common.WrapIntoReconcileTolerantError(err, "failed to get secret")
	}

	return cluster.GetConfiguration(secret)
}

func (r *reconciler) getNodesState(ctx context.Context, desiredVersion string) (*cluster.NodesState, error) {
	var continueToken string

	var nodes []corev1.Node
	for {
		list := &corev1.NodeList{}
		err := r.client.List(ctx, list, &client.ListOptions{
			Limit:    nodeListPageSize,
			Continue: continueToken,
		})
		if err != nil {
			return nil, common.WrapIntoReconcileTolerantError(err, "failed to list nodes")
		}

		nodes = append(nodes, list.Items...)

		if list.Continue == "" {
			break
		}
		continueToken = list.Continue
	}

	return cluster.GetNodesState(nodes, desiredVersion)
}

func (r *reconciler) getControlPlaneState(ctx context.Context, desiredVersion string) (*cluster.ControlPlaneState, error) {
	pods, err := r.getControlPlanePods(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane pods: %w", err)
	}

	return cluster.GetControlPlaneState(pods, desiredVersion)
}

func (r *reconciler) getControlPlanePods(ctx context.Context, isRetry bool) (*corev1.PodList, error) {
	podList := &corev1.PodList{}

	labelSelector, err := labels.Parse(fmt.Sprintf(
		"component in (%s,%s,%s)",
		common.KubeApiServer,
		common.KubeScheduler,
		common.KubeControllerManager))
	if err != nil {
		return nil, fmt.Errorf("failed to parse components selector: %w", err)
	}

	err = r.client.List(ctx, podList, &client.ListOptions{
		LabelSelector: labelSelector,
		Namespace:     common.KubeSystemNamespace,
	})
	if err != nil {
		return nil, common.WrapIntoReconcileTolerantError(err, "failed to fetch pod list")
	}

	if isRetry {
		return podList, nil
	}

	// A simple single-cycle retry that solves:
	// 1) Getting just-created pods that would count as not ready;
	// 2) Incomplete List results from previous call.

	const retryDelay = 15 * time.Second
	notReadyPods := 0
	nodes := make(map[string]struct{})
	for _, pod := range podList.Items {
		if _, exists := nodes[pod.Spec.NodeName]; !exists {
			nodes[pod.Spec.NodeName] = struct{}{}
		}

		if pod.Status.Phase != corev1.PodRunning {
			notReadyPods++
			continue
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Running == nil || !containerStatus.Ready {
				notReadyPods++
				break
			}
		}
	}

	var needRetry bool

	if notReadyPods > 0 {
		klog.Warningf("Pod readiness check failed: %d instance(s) not ready", notReadyPods)
		needRetry = true
	}

	// Edge case: Received pods may be from only a subset of master nodes due to API server
	// batching or eventual consistency (e.g., 3 pods all from node-1 when cluster has 3 nodes).
	// We don't implement special handling for this because:
	// 1. The condition is temporary (eventual consistency converges)
	// 2. Next reconcile cycle will capture all pods
	// 3. Added complexity outweighs benefit for this self-correcting scenario
	expectedPodsCount := len(nodes) * cluster.ControlPlaneComponentsCount
	if len(podList.Items) == 0 || expectedPodsCount > len(podList.Items) {
		klog.Warningf("Insufficient control plane pods found. Expected: %d, found: %d", expectedPodsCount, len(podList.Items))
		needRetry = true
	}

	if needRetry {
		klog.Warningf("Incomplete control plane pod data, retry after %v", retryDelay)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryDelay):
			return r.getControlPlanePods(ctx, true)
		}
	}

	return podList, nil
}
