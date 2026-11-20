// Copyright 2021 Flant JSC
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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// masterNodeState carries the schedulability of a control-plane node.
// Cordon keeps the control-plane label and does not evict running pods,
// so a cordoned node stays in the snapshot and the flag is the only way
// to tell it apart.
type masterNodeState struct {
	Name          string `json:"name"`
	Unschedulable bool   `json:"unschedulable"`
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
}, isHighAvailabilityCluster)

func applyMasterNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := new(corev1.Node)

	if err := sdk.FromUnstructured(obj, node); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	return masterNodeState{Name: node.GetName(), Unschedulable: node.Spec.Unschedulable}, nil
}

func isHighAvailabilityCluster(_ context.Context, input *go_hook.HookInput) error {
	masterNodesSnap := input.Snapshots.Get("master_node_names")

	mastersCount := len(masterNodesSnap)

	// Counted separately from mastersCount: consumers that size a workload by the
	// nodes it can actually be placed on need the schedulable number, while etcd
	// and the registry storage need every master, cordoned ones included.
	schedulableMastersCount := 0

	for masterNode, err := range sdkobjectpatch.SnapshotIter[masterNodeState](masterNodesSnap) {
		if err != nil {
			// Undercounting keeps the workloads placeable, so skip the broken entry
			// instead of failing the hook and leaving every consumer without values.
			input.Logger.Warn("skip master node snapshot", log.Err(err))

			continue
		}

		if !masterNode.Unschedulable {
			schedulableMastersCount++
		}
	}

	input.Values.Set("global.discovery.clusterMasterCount", mastersCount)
	input.Values.Set("global.discovery.clusterSchedulableMasterCount", schedulableMastersCount)
	input.Values.Set("global.discovery.clusterControlPlaneIsHighlyAvailable", mastersCount > 1)

	return nil
}
