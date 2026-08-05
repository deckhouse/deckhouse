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

package preflight

import (
	"fmt"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"

	ycpccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/pcc/v1"
	ycval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/validation"
)

// Validation violation codes for legacy ProviderClusterConfiguration checks.
const (
	CodePCCMasterReplicasGreaterExternalIPAddresses    = "pcc_master_node_group_replicas_greater_length_of_extrenal_ip_addresses"
	CodePCCNodeGroupReplicasGreaterExternalIPAddresses = "pcc_node_group_replicas_greater_length_of_extrenal_ip_addresses"
	CodePCCNATInstanceSubnetRequired                   = "pcc_internal_subnet_cidr_or_internal_subnet_id_empty"
)

// ValidatePreflight checks resources required before cluster bootstrap or converge.
func ValidatePreflight(state *ycval.State, operation string) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	// Validate legacy ProviderClusterConfiguration.
	if state.HasProviderClusterConfig() {
		result.Merge(
			validateMasterNodeGroupReplicasAndIPAddresses(state.ProviderClusterConfig),
			validateNodeGroupsReplicasAndIPAddresses(state.ProviderClusterConfig),
			validateWithNATInstanceLayout(state.ProviderClusterConfig, operation),
		)
	}

	if cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return result
	}

	// Validate new resources: ModuleConfig, NodeGroup, InstanceClasses, CredentialSecrets.
	result.Merge(
		cpval.ValidateModuleConfig(state),
		cpval.ValidateCredentialSecretPresence(state),
		cpval.ValidateCredentialSecretContent(state, ycval.CredentialsValidator),
		cpval.ValidateMasterNodeGroupPresence(state),
		cpval.ValidateNodeGroupsClassReference(state, true),
		cpval.ValidateInstanceClassesEtcdDisk(state),
		ycval.ValidateNodeGroupExternalIPAddresses(state),
		ycval.ValidateWithNATInstanceLayout(state),
	)

	return result
}

func validateMasterNodeGroupReplicasAndIPAddresses(pcc *ycpccv1.YandexProviderClusterConfiguration) cpvalapi.Result {
	result := cpvalapi.Result{}

	masterNodeGroup := pcc.MasterNodeGroup
	addresses := masterNodeGroup.InstanceClass.ExternalIPAddresses

	if masterNodeGroup.Replicas > 0 && len(addresses) > 0 && masterNodeGroup.Replicas > len(addresses) {
		result.AddError(
			"ProviderClusterConfiguration.masterNodeGroup.instanceClass.externalIPAddresses",
			CodePCCMasterReplicasGreaterExternalIPAddresses,
			addresses,
			fmt.Sprintf(
				"number of masterNodeGroup.replicas (%d) should be less or equal to the length of masterNodeGroup.instanceClass.externalIPAddresses (%d)",
				masterNodeGroup.Replicas, len(addresses),
			),
		)
	}

	return result
}

func validateNodeGroupsReplicasAndIPAddresses(pcc *ycpccv1.YandexProviderClusterConfiguration) cpvalapi.Result {
	result := cpvalapi.Result{}

	for i, nodeGroup := range pcc.NodeGroups {
		addresses := nodeGroup.InstanceClass.ExternalIPAddresses
		if nodeGroup.Replicas <= 0 || len(addresses) == 0 || nodeGroup.Replicas <= len(addresses) {
			continue
		}

		result.AddError(
			fmt.Sprintf("ProviderClusterConfiguration.nodeGroups[%d].instanceClass.externalIPAddresses", i),
			CodePCCNodeGroupReplicasGreaterExternalIPAddresses,
			addresses,
			fmt.Sprintf(
				`number of nodeGroups["%s"].replicas (%d) should be less or equal to the length of nodeGroups["%s"].instanceClass.externalIPAddresses (%d)`,
				nodeGroup.Name, nodeGroup.Replicas, nodeGroup.Name, len(addresses),
			),
		)
	}

	return result
}

func validateWithNATInstanceLayout(pcc *ycpccv1.YandexProviderClusterConfiguration, operation string) cpvalapi.Result {
	result := cpvalapi.Result{}

	if pcc.Layout != ycval.LayoutWithNATInstance {
		return result
	}

	if operation != proto.OperationBootstrap {
		return result
	}

	natInstance := pcc.WithNATInstance
	hasInternalSubnetCIDR := natInstance != nil && natInstance.InternalSubnetCIDR != nil && *natInstance.InternalSubnetCIDR != ""
	hasInternalSubnetID := natInstance != nil && natInstance.InternalSubnetID != nil && *natInstance.InternalSubnetID != ""

	if hasInternalSubnetCIDR || hasInternalSubnetID {
		return result
	}

	result.AddError(
		"ProviderClusterConfiguration.withNATInstance",
		CodePCCNATInstanceSubnetRequired,
		natInstance,
		"must provide internalSubnetCIDR or internalSubnetID for withNATInstance",
	)

	return result
}
