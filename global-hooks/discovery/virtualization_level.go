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
	"math"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type masterNodeInfo struct {
	Name                string
	VirtualizationLevel int
}

const (
	nodeRole               = "master"
	cmFieldName            = "level"
	masterNodeGroup        = "node.deckhouse.io/group"
	virtualizationLevelKey = "node.deckhouse.io/dvp-nesting-level"
	cmSnapshotName         = "virtualization_level_configmap"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       cmSnapshotName,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-virtualization-level"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyConfigMapFilter,
		},
		{
			Name:          "master_nodes",
			ApiVersion:    "v1",
			Kind:          "Node",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{masterNodeGroup: nodeRole}},
			FilterFunc:    applyMasterNodesFilter,
		},
	},
}, setGlobalVirtualizationLevel)

func applyConfigMapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := new(corev1.ConfigMap)
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, err
	}

	level, ok := cm.Data[cmFieldName]
	if !ok {
		return nil, nil
	}

	virtualizationLevel, err := strconv.Atoi(level)
	if err != nil {
		return nil, nil
	}

	return virtualizationLevel, nil
}

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

func setGlobalVirtualizationLevel(input *go_hook.HookInput) error {
	virtualizationLevel := 0

	virtualizationLevelFromLabels := getVirtualizationLevelFromMasterNodesLabels(input.Snapshots["master_nodes"])
	if len(input.Snapshots[cmSnapshotName]) != 0 && input.Snapshots[cmSnapshotName][0] != nil {
		virtualizationLevel = input.Snapshots[cmSnapshotName][0].(int)
		// if master nodes' labels report a deeper level of virtualization than the configmap, override the configmap value with the label-based on
		if virtualizationLevelFromLabels > virtualizationLevel {
			virtualizationLevel = virtualizationLevelFromLabels
			createOrUpdateVirtualizationLevelCM(input, virtualizationLevelFromLabels)
		}
	} else { // set virtualization level based on labels as configmap either doesn't exist or contains a faulty value
		virtualizationLevel = virtualizationLevelFromLabels
		createOrUpdateVirtualizationLevelCM(input, virtualizationLevel)
	}

	input.Values.Set("global.discovery.dvpNestingLevel", virtualizationLevel)
	input.Logger.Infof("set DVP nesting level to: %d", virtualizationLevel)

	return nil
}

func createOrUpdateVirtualizationLevelCM(input *go_hook.HookInput, virtualizationLevel int) {
	configmap := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-virtualization-level",
			Namespace: "d8-system",
			Labels: map[string]string{
				"app":      "deckhouse",
				"module":   "deckhouse",
				"heritage": "deckhouse",
			},
		},
		Data: map[string]string{"level": (strconv.Itoa(virtualizationLevel))},
	}
	input.PatchCollector.CreateOrUpdate(configmap)
}

func getVirtualizationLevelFromMasterNodesLabels(masterNodeInfoSnaps []go_hook.FilterResult) int {
	minimalVirtualizationLevel := math.MaxInt
	for _, masterNodeInfoSnap := range masterNodeInfoSnaps {
		masterNodeInfo, ok := masterNodeInfoSnap.(masterNodeInfo)
		if ok {
			if masterNodeInfo.VirtualizationLevel >= 0 && masterNodeInfo.VirtualizationLevel < minimalVirtualizationLevel {
				minimalVirtualizationLevel = masterNodeInfo.VirtualizationLevel
			}
		}
	}

	if minimalVirtualizationLevel == math.MaxInt {
		minimalVirtualizationLevel = 0
	}

	return minimalVirtualizationLevel
}
