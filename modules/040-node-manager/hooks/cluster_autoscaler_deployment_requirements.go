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

package hooks

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

type autoscalerNodeGroup struct {
	Name       string
	Engine     ngv1.NodeGroupEngine
	UseMCM     bool
	NodeType   ngv1.NodeType
	MinPerZone int32
	MaxPerZone int32
	CloudZones []string
}

func autoscalerNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	var minPerZone int32
	if ng.Spec.CloudInstances.MinPerZone != nil {
		minPerZone = *ng.Spec.CloudInstances.MinPerZone
	}

	var maxPerZone int32
	if ng.Spec.CloudInstances.MaxPerZone != nil {
		maxPerZone = *ng.Spec.CloudInstances.MaxPerZone
	}

	return autoscalerNodeGroup{
		Name:       ng.Name,
		Engine:     ng.Status.Engine,
		UseMCM:     ng.GetAnnotations()[useMCMAnnotation] != "",
		NodeType:   ng.Spec.NodeType,
		MinPerZone: minPerZone,
		MaxPerZone: maxPerZone,
		CloudZones: ng.Spec.CloudInstances.Zones,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_group",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: autoscalerNodeGroupFilter,
		},
	},
}, handleClusterAutoscalerDeploymentRequirements)

func handleClusterAutoscalerDeploymentRequirements(_ context.Context, input *go_hook.HookInput) error {
	deployMCM := false
	deployCAPI := false
	mcmNodes := make([]string, 0)
	capiNodes := make([]string, 0)

	prefix := input.Values.Get("nodeManager.internal.instancePrefix").String()
	clusterUUID := input.Values.Get("global.discovery.clusterUUID").String()

	snaps := input.Snapshots.Get("node_group")
	for ng, err := range sdkobjectpatch.SnapshotIter[autoscalerNodeGroup](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'node_group' snapshots: %w", err)
		}

		if ng.NodeType != ngv1.NodeTypeCloudEphemeral {
			continue
		}
		if ng.MinPerZone == ng.MaxPerZone {
			continue
		}

		engine := ng.Engine
		if engine == "" {
			engine = defaultCloudEphemeralNodeGroupEngineForNewNodeGroups(input, ng.UseMCM)
		}

		for _, zoneName := range ng.CloudZones {
			mdSuffix := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%v%v", clusterUUID, zoneName))))[:8]
			mdName := fmt.Sprintf("%s-%s", ng.Name, mdSuffix)
			if prefix != "" {
				mdName = fmt.Sprintf("%s-%s", prefix, mdName)
			}
			arg := fmt.Sprintf("--nodes=%d:%d:d8-cloud-instance-manager.%s", ng.MinPerZone, ng.MaxPerZone, mdName)

			switch engine {
			case ngv1.NodeGroupEngineMCM:
				deployMCM = true
				mcmNodes = append(mcmNodes, arg)
			case ngv1.NodeGroupEngineCAPI:
				deployCAPI = true
				capiNodes = append(capiNodes, arg)
			}
		}
	}

	if deployMCM {
		input.Values.Set("nodeManager.internal.deployAutoscalerMCM", true)
		input.Values.Set("nodeManager.internal.autoscalerMCMNodes", mcmNodes)
	} else {
		input.Values.Remove("nodeManager.internal.deployAutoscalerMCM")
		input.Values.Remove("nodeManager.internal.autoscalerMCMNodes")
	}

	if deployCAPI {
		input.Values.Set("nodeManager.internal.deployAutoscaler", true)
		input.Values.Set("nodeManager.internal.autoscalerNodes", capiNodes)
	} else {
		input.Values.Remove("nodeManager.internal.deployAutoscaler")
		input.Values.Remove("nodeManager.internal.autoscalerNodes")
	}

	return nil
}
