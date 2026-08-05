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

package validation

import (
	"fmt"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

// Validation violation codes for Yandex-specific checks.
const (
	CodeNodeGroupNodesGreaterExternalIPAddresses = "node_group_nodes_greater_length_of_external_ip_addresses"
	CodeNATInstanceSubnetRequired                = "internal_subnet_cidr_or_internal_subnet_id_empty"

	LayoutWithNATInstance = "WithNATInstance"
)

// ValidateNodeGroupExternalIPAddresses checks that every CloudPermanent NodeGroup with configured
// external addresses has at least as many addresses in
// settings.nodes.parameters.externalIPAddresses as the number of nodes it creates.
//
// The number of nodes is spec.cloudInstances.maxPerZone: the migration hook projects the legacy
// masterNodeGroup/nodeGroups replicas count into both minPerZone and maxPerZone.
func ValidateNodeGroupExternalIPAddresses(state *State) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	if state.ModuleConfig == nil {
		return result
	}

	externalIPAddresses := state.ModuleConfig.Spec.Settings.Nodes.Parameters.ExternalIPAddresses
	if len(externalIPAddresses) == 0 {
		return result
	}

	for _, nodeGroup := range state.NodeGroups {
		if nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent || nodeGroup.Spec.CloudInstances == nil {
			continue
		}

		addresses := externalIPAddresses[nodeGroup.Name]
		if len(addresses) == 0 {
			continue
		}

		nodes := nodeGroup.Spec.CloudInstances.MaxPerZone
		if nodes <= 0 || nodes <= len(addresses) {
			continue
		}

		result.AddError(
			fmt.Sprintf(
				"ModuleConfig/%s.spec.settings.nodes.parameters.externalIPAddresses.%s",
				state.ModuleName, nodeGroup.Name,
			),
			CodeNodeGroupNodesGreaterExternalIPAddresses,
			addresses,
			fmt.Sprintf(
				`number of nodes in NodeGroup %q (%d) should be less than or equal to the length of settings.nodes.parameters.externalIPAddresses[%q] (%d)`,
				nodeGroup.Name, nodes, nodeGroup.Name, len(addresses),
			),
		)
	}

	return result
}

func ValidateWithNATInstanceLayout(state *State) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	if state.ModuleConfig == nil {
		return result
	}

	nodesParams := state.ModuleConfig.Spec.Settings.Nodes.Parameters
	if nodesParams.Layout != LayoutWithNATInstance {
		return result
	}

	natInstance := nodesParams.WithNATInstance
	hasInternalSubnetCIDR := natInstance.InternalSubnetCIDR != ""
	hasInternalSubnetID := natInstance.InternalSubnetID != ""

	if hasInternalSubnetCIDR || hasInternalSubnetID {
		return result
	}

	result.AddError(
		"ModuleConfig.spec.settings.nodes.parameters.withNATInstance",
		CodeNATInstanceSubnetRequired,
		natInstance,
		"must provide internalSubnetCIDR or internalSubnetID for withNATInstance",
	)

	return result
}
