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
	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"

	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycpccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/pcc/v1"
)

// Validation violation codes for Yandex-specific checks.
const (
	CodeNodeGroupNodesGreaterExternalIPAddresses = "node_group_nodes_greater_length_of_external_ip_addresses"
	CodeNATInstanceSubnetRequired                = "internal_subnet_cidr_or_internal_subnet_id_empty"
	CodeProvisionedStorageClassNamesUnique       = "provisioned_storage_class_names_not_unique"
	CodeDiskHasNonMultipleSize                   = "disk_has_non_multiple_size"

	CodePCCInvalidServiceAccountSecret                 = "pcc_invalid_service_account_secret"
	CodePCCMasterReplicasGreaterExternalIPAddresses    = "pcc_master_node_group_replicas_greater_length_of_extrenal_ip_addresses"
	CodePCCNodeGroupReplicasGreaterExternalIPAddresses = "pcc_node_group_replicas_greater_length_of_extrenal_ip_addresses"
	CodePCCNATInstanceSubnetRequired                   = "pcc_internal_subnet_cidr_or_internal_subnet_id_empty"
	CodePCCDiskHasNonMultipleSize                      = "pcc_disk_has_non_multiple_size"

	LayoutWithNATInstance = "WithNATInstance"
)

// Disk sizes the IaC and the machine templates fall back to when an instance class leaves the
// size unset: terraform-modules/{master-node,static-node}/variables.tf, the migration projection
// of the legacy PCC, cloud-instance-manager/machine-class.yaml and capi/template.yaml. A rule that
// skipped an unset size would let a disk type with a size restriction through to the cloud API.
const (
	defaultDiskSizeGB     = 50
	defaultEtcdDiskSizeGB = 10
)

// diskSizeMultipleGB maps the disk types Yandex Cloud only allocates in fixed-size units to that
// unit, in GiB - the unit diskSizeGB and etcdDiskSizeGB are specified in.
var diskSizeMultipleGB = map[string]int{
	"network-ssd-nonreplicated": 93,
	"network-ssd-io-m3":         93,
}

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

// ValidateProvisionedStorageClasses checks that settings.storage.parameters.provisionedStorageClasses
// declares every StorageClass name at most once.
//
// hooks/storage_classes.go matches provisioned names against the default StorageClasses exactly and
// appends the provisioned list as it is, so a repeated name reaches
// internal.storageClasses twice and templates/csi/storage-classes.yaml renders two StorageClass
// objects with the same name — which the Helm release cannot apply. Overriding a default name
// (network-hdd, network-ssd, network-ssd-nonreplicated, network-ssd-io-m3) is not a duplicate:
// the default is dropped in favour of the provisioned entry.
//
// Only the first repeated name is reported: Result keys violations by code and path, so every
// further duplicate would collapse into the same violation anyway.
func ValidateProvisionedStorageClasses(state *State) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	if state.ModuleConfig == nil {
		return result
	}

	storageParams := state.ModuleConfig.Spec.Settings.Storage.Parameters
	if len(storageParams.ProvisionedStorageClasses) == 0 {
		return result
	}

	uniqueSCNames := make(map[string]any, len(storageParams.ProvisionedStorageClasses))
	for _, sc := range storageParams.ProvisionedStorageClasses {
		if _, ok := uniqueSCNames[sc.Name]; ok {
			result.AddError(
				"ModuleConfig.spec.settings.storage.parameters.provisionedStorageClasses",
				CodeProvisionedStorageClassNamesUnique,
				sc.Name,
				"names of provisioned storage classes must not be repeated",
			)

			return result
		}

		uniqueSCNames[sc.Name] = struct{}{}
	}

	return result
}

// ValidateDiskSizeByType checks that every YandexInstanceClass with a network-ssd-nonreplicated or
// network-ssd-io-m3 disk sizes its boot and etcd disks in multiples of 93 GiB, the only sizes
// Yandex Cloud creates for these types. The etcd disk is created with the boot disk type.
//
// An unset diskSizeGB is checked as the default size. An unset etcdDiskSizeGB is skipped: only a
// master instance class gets an etcd disk, and ValidateInstanceClassesEtcdDisk already requires it
// to be set there.
func ValidateDiskSizeByType(state *State) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	for _, ic := range state.InstanceClasses {
		diskType := ic.Spec.DiskType

		var diskSizeGB *int
		if ic.Spec.DiskSizeGB != 0 {
			diskSizeGB = &ic.Spec.DiskSizeGB
		}

		validateDiskSizeMultiple(
			&result,
			fmt.Sprintf("%s/%s.spec.diskSizeGB", ycicv1.YandexInstanceClassKind, ic.GetName()),
			CodeDiskHasNonMultipleSize,
			diskType, diskSizeGB, defaultDiskSizeGB,
		)

		if ic.Spec.EtcdDiskSizeGB != nil {
			validateDiskSizeMultiple(
				&result,
				fmt.Sprintf("%s/%s.spec.etcdDiskSizeGB", ycicv1.YandexInstanceClassKind, ic.GetName()),
				CodeDiskHasNonMultipleSize,
				diskType, ic.Spec.EtcdDiskSizeGB, defaultEtcdDiskSizeGB,
			)
		}
	}

	return result
}

// ValidatePCCServiceAccount checks that the legacy PCC carries a parseable service account key.
// The key is never reported back: the violation value is masked.
func ValidatePCCServiceAccount(pcc *ycpccv1.YandexProviderClusterConfiguration) cpvalapi.Result {
	result := cpvalapi.Result{}

	if err := ValidateServiceAccountFunc(pcc.Provider.ServiceAccountJSON); err != nil {
		result.AddError(
			"ProviderClusterConfiguration.provider.serviceAccountJSON",
			CodePCCInvalidServiceAccountSecret,
			"masked",
			fmt.Sprintf("invalid service account: %v", err),
		)
	}

	return result
}

// ValidatePCCNodeGroupsReplicasAndIPAddresses checks that the legacy PCC master node group and every
// node group with configured externalIPAddresses have at least as many addresses as replicas.
func ValidatePCCNodeGroupsReplicasAndIPAddresses(pcc *ycpccv1.YandexProviderClusterConfiguration) cpvalapi.Result {
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

// ValidatePCCWithNATInstanceLayout checks that a legacy PCC with the WithNATInstance layout names
// the internal subnet on bootstrap.
//
// The reported value is the withNATInstance section without exporterAPIKey: the violation is
// returned to dhctl and must not carry the key.
func ValidatePCCWithNATInstanceLayout(pcc *ycpccv1.YandexProviderClusterConfiguration, operation validatev1.Operation) cpvalapi.Result {
	result := cpvalapi.Result{}

	if pcc.Layout != LayoutWithNATInstance {
		return result
	}

	if operation != validatev1.OperationBootstrap {
		return result
	}

	natInstance := pcc.WithNATInstance
	hasInternalSubnetCIDR := natInstance != nil && natInstance.InternalSubnetCIDR != nil && *natInstance.InternalSubnetCIDR != ""
	hasInternalSubnetID := natInstance != nil && natInstance.InternalSubnetID != nil && *natInstance.InternalSubnetID != ""

	if hasInternalSubnetCIDR || hasInternalSubnetID {
		return result
	}

	var value *ycpccv1.YandexWithNATInstance
	if natInstance != nil {
		masked := *natInstance
		masked.ExporterAPIKey = nil
		value = &masked
	}

	result.AddError(
		"ProviderClusterConfiguration.withNATInstance",
		CodePCCNATInstanceSubnetRequired,
		value,
		"must provide internalSubnetCIDR or internalSubnetID for withNATInstance",
	)

	return result
}

// ValidatePCCDiskSizeByType is ValidateDiskSizeByType for the legacy PCC: the master boot and etcd
// disks and every node group boot disk.
//
// Unset sizes are checked as the defaults the PCC is projected with. An unset diskType is skipped:
// the projection fills it with network-ssd, which has no size restriction.
func ValidatePCCDiskSizeByType(pcc *ycpccv1.YandexProviderClusterConfiguration) cpvalapi.Result {
	result := cpvalapi.Result{}

	masterInstanceClass := pcc.MasterNodeGroup.InstanceClass
	if masterInstanceClass.DiskType != nil {
		validateDiskSizeMultiple(
			&result,
			"ProviderClusterConfiguration.masterNodeGroup.instanceClass.diskSizeGB",
			CodePCCDiskHasNonMultipleSize,
			*masterInstanceClass.DiskType, masterInstanceClass.DiskSizeGB, defaultDiskSizeGB,
		)
		validateDiskSizeMultiple(
			&result,
			"ProviderClusterConfiguration.masterNodeGroup.instanceClass.etcdDiskSizeGb",
			CodePCCDiskHasNonMultipleSize,
			*masterInstanceClass.DiskType, masterInstanceClass.EtcdDiskSizeGB, defaultEtcdDiskSizeGB,
		)
	}

	for i, nodeGroup := range pcc.NodeGroups {
		if nodeGroup.InstanceClass.DiskType == nil {
			continue
		}

		validateDiskSizeMultiple(
			&result,
			fmt.Sprintf("ProviderClusterConfiguration.nodeGroups[%d].instanceClass.diskSizeGB", i),
			CodePCCDiskHasNonMultipleSize,
			*nodeGroup.InstanceClass.DiskType, nodeGroup.InstanceClass.DiskSizeGB, defaultDiskSizeGB,
		)
	}

	return result
}

// validateDiskSizeMultiple adds a violation when a disk of a type with a size restriction is not
// sized in multiples of its allocation unit. A nil sizeGB is checked as defaultSizeGB.
func validateDiskSizeMultiple(result *cpvalapi.Result, path, code, diskType string, sizeGB *int, defaultSizeGB int) {
	multipleGB, ok := diskSizeMultipleGB[diskType]
	if !ok {
		return
	}

	size := defaultSizeGB
	if sizeGB != nil {
		size = *sizeGB
	}

	if size%multipleGB == 0 {
		return
	}

	message := fmt.Sprintf(
		"disk of type %s must have a size that is a multiple of %d GiB, got %d GiB",
		diskType, multipleGB, size,
	)
	if sizeGB == nil {
		message += " (the default, the size is not set)"
	}

	result.AddError(path, code, size, message)
}
