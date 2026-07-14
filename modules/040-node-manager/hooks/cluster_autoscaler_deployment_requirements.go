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
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

type autoscalerNodeGroupValue struct {
	Name           string                `json:"name"`
	Engine         ngv1.NodeGroupEngine  `json:"engine"`
	NodeType       ngv1.NodeType         `json:"nodeType"`
	CloudInstances autoscalerCloudValues `json:"cloudInstances"`
}

type autoscalerCloudValues struct {
	MinPerZone *int32   `json:"minPerZone"`
	MaxPerZone *int32   `json:"maxPerZone"`
	Zones      []string `json:"zones"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 11},
}, handleClusterAutoscalerDeploymentRequirements)

func handleClusterAutoscalerDeploymentRequirements(_ context.Context, input *go_hook.HookInput) error {
	deployMCM := false
	deployCAPI := false
	mcmNodes := make([]string, 0)
	capiNodes := make([]string, 0)

	prefix := input.Values.Get("nodeManager.internal.instancePrefix").String()
	clusterUUID := input.Values.Get("global.discovery.clusterUUID").String()

	var nodeGroups []autoscalerNodeGroupValue
	rawNodeGroups := input.Values.Get("nodeManager.internal.nodeGroups")
	if !rawNodeGroups.Exists() || rawNodeGroups.String() == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(rawNodeGroups.String()), &nodeGroups); err != nil {
		return fmt.Errorf("failed to unmarshal 'nodeManager.internal.nodeGroups': %w", err)
	}

	for _, ng := range nodeGroups {
		if ng.NodeType != ngv1.NodeTypeCloudEphemeral {
			continue
		}
		if ng.CloudInstances.MinPerZone == nil || ng.CloudInstances.MaxPerZone == nil {
			continue
		}
		if *ng.CloudInstances.MinPerZone == *ng.CloudInstances.MaxPerZone {
			continue
		}

		for _, zoneName := range ng.CloudInstances.Zones {
			mdSuffix := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%v%v", clusterUUID, zoneName))))[:8]
			mdName := fmt.Sprintf("%s-%s", ng.Name, mdSuffix)
			if prefix != "" {
				mdName = fmt.Sprintf("%s-%s", prefix, mdName)
			}
			arg := fmt.Sprintf("--nodes=%d:%d:d8-cloud-instance-manager.%s", *ng.CloudInstances.MinPerZone, *ng.CloudInstances.MaxPerZone, mdName)

			switch ng.Engine {
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
