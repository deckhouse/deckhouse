/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/mcm/v1alpha1"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"

	"github.com/flant/shell-operator/pkg/kube/object_patch"
)

func patchNodeGroupStatus(patcher *object_patch.PatchCollector, nodeGroupName string, patch interface{}) {
	patcher.MergePatch(patch, "deckhouse.io/v1", "NodeGroup", "", nodeGroupName, object_patch.WithSubresource("/status"))
}

func setNodeGroupStandbyStatus(patcher *object_patch.PatchCollector, nodeGroupName string, standby *int) {
	statusStandbyPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"standby": standby,
		},
	}
	patchNodeGroupStatus(patcher, nodeGroupName, statusStandbyPatch)
}

func setNodeGroupErrorStatus(patcher *object_patch.PatchCollector, nodeGroupName, message string) {
	statusErrorPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"error": message,
		},
	}
	patchNodeGroupStatus(patcher, nodeGroupName, statusErrorPatch)
}

func setNodeGroupKubeVersionStatus(patcher *object_patch.PatchCollector, nodeGroupName string, version string) {
	kubeVersionPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"kubernetesVersion": version,
		},
	}
	patchNodeGroupStatus(patcher, nodeGroupName, kubeVersionPatch)
}

func buildUpdateStatusPatch(
	nodesNum, readyNodesNum, uptodateNodesCount, minPerZone, maxPerZone, desiredMax, instancesNum int32,
	nodeType ngv1.NodeType, statusMsg string,
	lastMachineFailures []*v1alpha1.MachineSummary,
) interface{} {
	ready := "True"
	if len(statusMsg) > 0 {
		ready = "False"
	}

	patch := map[string]interface{}{
		"nodes":    nodesNum,
		"ready":    readyNodesNum,
		"upToDate": uptodateNodesCount,
	}
	if nodeType == ngv1.NodeTypeCloudEphemeral {
		patch["min"] = minPerZone
		patch["max"] = maxPerZone
		patch["desired"] = desiredMax
		patch["instances"] = instancesNum
		patch["lastMachineFailures"] = lastMachineFailures

		if len(lastMachineFailures) == 0 {
			patch["lastMachineFailures"] = make([]interface{}, 0) // to make [] array in json result
		}
	}

	patch["conditionSummary"] = map[string]interface{}{
		"ready":         ready,
		"statusMessage": statusMsg,
	}

	statusPatch := map[string]interface{}{
		"status": patch,
	}

	return statusPatch
}
