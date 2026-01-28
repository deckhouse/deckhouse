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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
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
		{
			Name:                      "master_node_group",
			ApiVersion:                "deckhouse.io/v1",
			Kind:                      "NodeGroup",
			NameSelector:              &types.NameSelector{MatchNames: []string{masterNodeGroupName}},
			FilterFunc:                applyMasterNodeGroupFilter,
			ExecuteHookOnEvents:       ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
		},
	},
}, isHighAvailabilityCluster)

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

func isHighAvailabilityCluster(ctx context.Context, input *go_hook.HookInput) error {
	masterNodesSnap := input.Snapshots.Get("master_node_names")
	mastersCount := len(masterNodesSnap)

	input.Values.Set("global.discovery.clusterMasterCount", mastersCount)

	desiredMasterReplicas := getDesiredMasterReplicasFromNodeGroup(input)

	if desiredMasterReplicas == 0 {
		desiredMasterReplicas = getDesiredMasterReplicasFromClusterConfig(input)
	}

	// Determine HA mode based on desired replicas
	// If desired replicas = 1, don't enable HA even if current masters > 1
	// This prevents false HA mode during temporary scaling
	var isHA bool
	switch {
	case desiredMasterReplicas > 1:
		isHA = true
	case desiredMasterReplicas == 1:
		isHA = false // Explicitly disable HA for single-master configuration
	case desiredMasterReplicas == 0:
		isHA = mastersCount > 1 //  if we couldn't determine desired master replicas, use current count
	}

	input.Values.Set("global.discovery.clusterControlPlaneIsHighlyAvailable", isHA)

	// Log for debugging (Logger may be nil in tests)
	if input.Logger != nil {
		input.Logger.Debug(fmt.Sprintf("HA mode determination: desiredReplicas=%d, currentMasters=%d, isHA=%v",
			desiredMasterReplicas, mastersCount, isHA))
	}

	return nil
}

func getDesiredMasterReplicasFromNodeGroup(input *go_hook.HookInput) int32 {
	// Check if snapshot exists before unmarshaling to avoid panics
	masterNodeGroupSnapRaw := input.Snapshots.Get("master_node_group")
	if len(masterNodeGroupSnapRaw) == 0 {
		return 0
	}

	masterNodeGroupSnap, err := sdkobjectpatch.UnmarshalToStruct[masterNodeGroupReplicas](input.Snapshots, "master_node_group")
	if err != nil {
		// Logger may be nil in tests, so check before using
		if input.Logger != nil {
			input.Logger.Debug(fmt.Sprintf("failed to unmarshal master_node_group snapshot: %v", err))
		}
		return 0
	}

	if len(masterNodeGroupSnap) == 0 {
		return 0
	}

	return masterNodeGroupSnap[0].DesiredReplicas
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

	// Try to get as int64 (Values.Get().Int() returns int64)
	// This handles most cases where replicas is stored as a number
	replicasInt := replicas.Int()
	replicasStr := replicas.String()

	// If string representation is empty or "null", value doesn't exist
	if replicasStr == "" || replicasStr == "null" {
		return 0
	}

	// If Int() returned non-zero, use it
	if replicasInt != 0 {
		return int32(replicasInt)
	}

	// If Int() returned 0, check if string is "0" (valid value) or something else
	// Try to parse string directly as fallback
	if replicasStr != "0" {
		var val int32
		if _, err := fmt.Sscanf(replicasStr, "%d", &val); err == nil {
			return val
		}
	}

	// If we got "0" as string or int, return 0 (which means "unknown" for our purposes)
	// The caller will use fallback logic
	return 0
}
