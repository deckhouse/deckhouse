// Copyright 2024 Flant JSC
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
	"log/slog"
	"math"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type masterNodeInfo struct {
	Name                string
	VirtualizationLevel int
}

const (
	nodeRole               = "master"
	masterNodeGroup        = "node.deckhouse.io/group"
	virtualizationLevelKey = "node.deckhouse.io/dvp-nesting-level"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:          "master_nodes",
			ApiVersion:    "v1",
			Kind:          "Node",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{masterNodeGroup: nodeRole}},
			FilterFunc:    applyMasterNodesFilter,
		},
	},
}, setGlobalVirtualizationLevel)

func applyMasterNodesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := new(corev1.Node)

	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	virtualizationLevel := 0
	if value, exists := node.GetLabels()[virtualizationLevelKey]; exists {
		if virtualizationLevel, err = strconv.Atoi(value); err != nil {
			// exclude a node with a faulty label
			return nil, nil
		}
	}

	return masterNodeInfo{Name: node.GetName(), VirtualizationLevel: virtualizationLevel}, nil
}

func setGlobalVirtualizationLevel(_ context.Context, input *go_hook.HookInput) error {
	virtualizationLevel := getVirtualizationLevelFromMasterNodesLabels(input.Snapshots.Get("master_nodes"))
	input.Values.Set("global.discovery.dvpNestingLevel", virtualizationLevel)
	input.Logger.Info("set DVP nesting level", slog.Int("level", virtualizationLevel))

	return nil
}

func getVirtualizationLevelFromMasterNodesLabels(masterNodeInfoSnaps []sdkpkg.Snapshot) int {
	minimalVirtualizationLevel := math.MaxInt
	for masterNodeInfo, err := range sdkobjectpatch.SnapshotIter[masterNodeInfo](masterNodeInfoSnaps) {
		if err != nil {
			continue
		}
		if masterNodeInfo.VirtualizationLevel >= 0 && masterNodeInfo.VirtualizationLevel < minimalVirtualizationLevel {
			minimalVirtualizationLevel = masterNodeInfo.VirtualizationLevel
		}
	}

	if minimalVirtualizationLevel == math.MaxInt {
		minimalVirtualizationLevel = 0
	}

	return minimalVirtualizationLevel
}
