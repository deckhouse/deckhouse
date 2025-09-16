/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cni-cilium/egress-label-cleanup",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodeFilter,
		},
		{
			Name:       "egressgateways",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "EgressGateway",
			FilterFunc: applyEgressGatewayFilter,
		},
	},
}, handleEgressNodeCleanup)

func handleEgressNodeCleanup(_ context.Context, input *go_hook.HookInput) error {
	egressGateways, err := sdkobjectpatch.UnmarshalToStruct[EgressGatewayInfo](input.Snapshots, "egressgateways")
	if err != nil {
		return fmt.Errorf("failed to unmarshal egressgateways snapshot: %w", err)
	}
	// Make maps all NodeSelector's EgressGateway's
	nodeSelectors := make(map[string]struct{})

	for _, eg := range egressGateways {
		for key := range eg.NodeSelector {
			nodeSelectors[key] = struct{}{}
		}
	}

	// Map for remove labels
	nodesToUnlabel := make(map[string][]string)

	nodes, err := sdkobjectpatch.UnmarshalToStruct[NodeInfo](input.Snapshots, "nodes")
	if err != nil {
		return fmt.Errorf("failed to unmarshal nodes snapshot: %w", err)
	}
	for _, node := range nodes {
		// Check if a node matches at least one NodeSelector
		var hasNodeSelectorLabel bool
		for key := range nodeSelectors {
			if _, ok := node.Labels[key]; ok {
				hasNodeSelectorLabel = true
				break
			}
		}

		// If a node does not participate in any NodeSelector, but it has our labels
		if !hasNodeSelectorLabel && (node.IsMemberLabeled || len(node.ActiveForEGs) > 0) {
			input.Logger.Info("Node %s has stale egress labels, cleaning up...", node.Name)

			labelsToRemove := make([]string, 0)

			// Drop label "member"
			if node.IsMemberLabeled {
				labelsToRemove = append(labelsToRemove, memberNodeLabelKey)
			}

			// Drop active-for-* label's
			for _, activeEG := range node.ActiveForEGs {
				labelsToRemove = append(labelsToRemove, activeNodeLabelPrefix+activeEG)
			}

			nodesToUnlabel[node.Name] = labelsToRemove
		}
	}

	// Patch label's
	processRemovingLabels(input, nodesToUnlabel)

	return nil
}
