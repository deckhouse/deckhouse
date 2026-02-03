// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const masterNodeGroupName = "master"

// masterNodeGroupReplicas represents desired replicas count from NodeGroup
type masterNodeGroupReplicas struct {
	DesiredReplicas int32
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_node_names",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			FilterFunc: applyMasterNodeFilter,
		},
	},
}, dependency.WithExternalDependencies(isHighAvailabilityCluster))

func applyMasterNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func applyMasterNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// Extract desired replicas from NodeGroup spec
	// For cloud clusters: minPerZone * number of zones
	// For static clusters: staticInstances.count

	var ng struct {
		Spec struct {
			CloudInstances struct {
				MinPerZone *int32   `json:"minPerZone"`
				Zones      []string `json:"zones"`
			} `json:"cloudInstances"`
			StaticInstances struct {
				Count int32 `json:"count"`
			} `json:"staticInstances"`
		} `json:"spec"`
	}

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	replicas := masterNodeGroupReplicas{DesiredReplicas: 0}

	// For cloud clusters, calculate: minPerZone * number of zones
	if ng.Spec.CloudInstances.MinPerZone != nil {
		zonesCount := len(ng.Spec.CloudInstances.Zones)
		if zonesCount == 0 {
			// If zones are not specified in NodeGroup, they will be filled with default zones
			// by node-manager. For our purposes, we assume at least 1 zone exists.
			// This is a conservative approach - if zones are not specified, we can't determine
			// the exact count, so we use minPerZone as-is (which represents 1 zone minimum).
			zonesCount = 1
		}
		replicas.DesiredReplicas = *ng.Spec.CloudInstances.MinPerZone * int32(zonesCount)
	} else if ng.Spec.StaticInstances.Count >= 0 {
		// For static clusters, use count
		// Count = 0 is invalid for master nodes but we return it and let caller handle it
		replicas.DesiredReplicas = ng.Spec.StaticInstances.Count
	}

	return replicas, nil
}

func isHighAvailabilityCluster(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	masterNodesSnap := input.Snapshots.Get("master_node_names")
	mastersCount := len(masterNodesSnap)

	input.Values.Set("global.discovery.clusterMasterCount", mastersCount)

	desiredMasterReplicas := getDesiredMasterReplicasFromNodeGroup(ctx, input, dc)

	if desiredMasterReplicas == 0 {
		desiredMasterReplicas = getDesiredMasterReplicasFromClusterConfig(input)
	}

	// Determine HA mode based on desired replicas
	// If desired replicas = 1, but current masters > 1, don't enable HA
	// because we must avoid false HA mode during temporary scaling to multi-master
	// Example case: destructive change in single-master cluster
	isHA := desiredMasterReplicas > 1
	if desiredMasterReplicas == 0 {
		// if we couldn't determine desired master replicas, fallback to current masters count
		isHA = mastersCount > 1
	}

	input.Values.Set("global.discovery.clusterControlPlaneIsHighlyAvailable", isHA)

	// Log for debugging (Logger may be nil in tests)
	input.Logger.Info(fmt.Sprintf("HA mode determination: desiredReplicas=%d, currentMasters=%d, isHA=%v",
		desiredMasterReplicas, mastersCount, isHA))

	return nil
}

func getDesiredMasterReplicasFromNodeGroup(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) int32 {
	kubeCl, err := dc.GetK8sClient()
	if err != nil {
		input.Logger.Warn(fmt.Sprintf("failed to init Kubernetes client for NodeGroup: %v", err))
		return 0
	}

	nodeGroupGVR := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}
	nodeGroup, err := kubeCl.Dynamic().Resource(nodeGroupGVR).Get(ctx, masterNodeGroupName, v1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			// NodeGroup CRD is not installed yet or master NodeGroup doesn't exist.
			return 0
		}
		input.Logger.Warn(fmt.Sprintf("failed to get master NodeGroup: %v", err))
		return 0
	}

	filtered, err := applyMasterNodeGroupFilter(nodeGroup)
	if err != nil {
		input.Logger.Warn(fmt.Sprintf("failed to parse master NodeGroup: %v", err))
		return 0
	}

	replicas, ok := filtered.(masterNodeGroupReplicas)
	if !ok {
		input.Logger.Warn("unexpected master NodeGroup filter result type")
		return 0
	}

	return replicas.DesiredReplicas
}

func getDesiredMasterReplicasFromClusterConfig(input *go_hook.HookInput) int32 {
	clusterConfig := input.Values.Get("global.clusterConfiguration")
	if !clusterConfig.Exists() {
		return 0
	}

	masterNG := clusterConfig.Get("masterNodeGroup")
	if !masterNG.Exists() {
		return 0
	}

	replicas := masterNG.Get("replicas")
	if !replicas.Exists() {
		return 0
	}

	return int32(replicas.Int())
}
