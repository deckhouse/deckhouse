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
	"strings"
	"time"

	"go.yaml.in/yaml/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"control-plane-manager/internal/controllers/update-observer/cluster"
	"control-plane-manager/internal/controllers/update-observer/common"
	podstatus "control-plane-manager/internal/controllers/update-observer/pkg/pod-status"
	"control-plane-manager/internal/controllers/update-observer/pkg/version"
)

func (r *reconciler) getClusterState(ctx context.Context, cfg *cluster.Configuration, configmapLabels map[string]string, downgradeInProgress bool) (*cluster.State, error) {
	sourceVersion := configmapLabels[common.K8sVersionLabelKey]

	nodesState, err := r.getNodesState(ctx, cfg.DesiredVersion, sourceVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes state: %w", err)
	}

	controlPlaneState, err := r.getControlPlaneState(ctx, cfg.DesiredVersion, sourceVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane state: %w", err)
	}

	versionSettings, err := cluster.LoadVersionSettingsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to parse versions from env: %w", err)
	}

	// availableVersions is derived from the maxUsed this pass computed, not from the max-k8s-version
	// label or from the value still stored in the ConfigMap. Both of those describe the previous
	// pass, so the published list would lag a reconcile behind the floor the ModuleConfig admission
	// webhook enforces from the same quantity — and the two must not contradict each other.
	return cluster.GetState(cfg, nodesState, controlPlaneState, versionSettings, cfg.MaxUsedVersion, sourceVersion, downgradeInProgress), nil
}

// desiredConfiguration returns the data.spec block to write: the declared configuration from the
// container environment, with maxUsedKubernetesVersion raised to the value already recorded in the
// ConfigMap when that one is higher.
//
// The max() is what makes the value monotonic in practice rather than only in intent. The
// environment is a snapshot of what values held when this Pod's template was rendered, so during a
// DaemonSet rollout an older Pod can still be the one reconciling, and on an HA cluster leadership
// can move to it mid-roll. Writing the environment verbatim would let such a Pod walk the recorded
// maximum backwards and hand a downgrade one extra minor of room.
//
// Reads of ConfigMaps in this binary bypass the cache (internal/manager.go), so the stored value
// compared here is the live one.
func desiredConfiguration(configMap *corev1.ConfigMap) (*cluster.Configuration, error) {
	cfg, err := cluster.LoadConfigurationFromEnv()
	if err != nil {
		return nil, err
	}

	cfg.MaxUsedVersion = version.GetMax(storedMaxUsedVersion(configMap), cfg.MaxUsedVersion)

	return cfg, nil
}

// storedMaxUsedVersion returns spec.maxUsedKubernetesVersion as currently written, or "" when the
// block is absent or unreadable. Unreadable is not an error: this controller is about to overwrite
// data.spec anyway, and refusing to reconcile over a hand-mangled block would block the very write
// that repairs it. Losing the stored value costs at most the floor the environment already carries.
func storedMaxUsedVersion(configMap *corev1.ConfigMap) string {
	var spec Spec
	if err := yaml.Unmarshal([]byte(configMap.Data["spec"]), &spec); err != nil {
		return ""
	}

	return strings.TrimSpace(spec.MaxUsedVersion)
}

func (r *reconciler) getNodesState(ctx context.Context, desiredVersion, sourceVersion string) (*cluster.NodesState, error) {
	list := &corev1.NodeList{}
	err := r.client.List(ctx, list)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	return cluster.GetNodesState(list.Items, desiredVersion, sourceVersion)
}

func (r *reconciler) getControlPlaneState(ctx context.Context, desiredVersion, sourceVersion string) (*cluster.ControlPlaneState, error) {
	pods, err := r.getControlPlanePods(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane pods: %w", err)
	}

	return cluster.GetControlPlaneState(pods, desiredVersion, sourceVersion)
}

func (r *reconciler) getControlPlanePods(ctx context.Context, isRetry bool) (*corev1.PodList, error) {
	podList := &corev1.PodList{}

	labelSelector, err := labels.Parse(fmt.Sprintf(
		"component in (%s,%s,%s)",
		common.KubeAPIServer,
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
		return nil, fmt.Errorf("failed to fetch pod list: %w", err)
	}

	if isRetry {
		return podList, nil
	}

	// A simple single-cycle retry that solves:
	// 1) Getting just-created pods that would count as not ready;
	// 2) Incomplete List results from previous call.

	const retryDelay = 30 * time.Second
	unhealthyPods := 0
	nodes := make(map[string]struct{})
	for _, pod := range podList.Items {
		if _, exists := nodes[pod.Spec.NodeName]; !exists {
			nodes[pod.Spec.NodeName] = struct{}{}
		}

		if !podstatus.IsHealthy(pod) {
			unhealthyPods++
		}
	}

	var needRetry bool

	if unhealthyPods > 0 {
		logger.Warn("Pod readiness check failed", "unhealthy_pods", unhealthyPods)
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
		logger.Warn("Insufficient control plane pods found", "expected", expectedPodsCount, "found", len(podList.Items))
		needRetry = true
	}

	if needRetry {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryDelay):
			return r.getControlPlanePods(ctx, true)
		}
	}

	return podList, nil
}
