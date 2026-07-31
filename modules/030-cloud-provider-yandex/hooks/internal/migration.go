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

package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	yciccv1 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycpccv1 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/pcc/v1"
	ycsettingsv1 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/settings/v1"
	ycsettingsv2 "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/api/settings/v2"
	ycmeta "github.com/deckhouse/deckhouse/cloud-provider-yandex/pkg/meta"
	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

func IsMigrationResourcesApplied(input *go_hook.HookInput, pcc ycpccv1.YandexProviderClusterConfiguration) bool {
	// Check ModuleConfig
	mcSnaps := input.Snapshots.Get("module_config")
	if len(mcSnaps) == 0 {
		return false
	}
	var mc ModuleConfigFilterResult
	if err := mcSnaps[0].UnmarshalTo(&mc); err != nil {
		return false
	}
	if mc.Version < 2 || !mc.Enabled || len(mc.SettingsV2) == 0 {
		return false
	}

	// Check Credential Secrets
	existingCreds, err := sdkobjectpatch.UnmarshalToStruct[NamedResourceFilterResult](input.Snapshots, "credential_secrets")
	if err != nil {
		return false
	}
	for _, cred := range existingCreds {
		if cred.Name == "" {
			return false
		}
	}

	// Check NodeGroups and InstanceClasses
	existingNodeGroups, err := sdkobjectpatch.UnmarshalToStruct[NamedResourceFilterResult](input.Snapshots, "node_groups")
	if err != nil {
		return false
	}
	nodeGroupSet := make(map[string]bool, len(existingNodeGroups))
	for _, ng := range existingNodeGroups {
		nodeGroupSet[ng.Name] = true
	}

	existingICs, err := sdkobjectpatch.UnmarshalToStruct[NamedResourceFilterResult](input.Snapshots, "yandex_instance_classes")
	if err != nil {
		return false
	}
	icSet := make(map[string]bool, len(existingICs))
	for _, ic := range existingICs {
		icSet[ic.Name] = true
	}

	// hybrid clusters have no masterNodeGroup
	if !nodeGroupSet["master"] || !icSet[cpapi.BuildInstanceClassName("master")] {
		return false
	}

	for _, nodeGroup := range pcc.NodeGroups {
		if nodeGroup.Name == "" {
			return false
		}

		if !nodeGroupSet[nodeGroup.Name] || !icSet[cpapi.BuildInstanceClassName(nodeGroup.Name)] {
			return false
		}
	}

	return true
}

func CreateMigrationResourcesSecret(
	input *go_hook.HookInput,
	pcc ycpccv1.YandexProviderClusterConfiguration,
	mc ycsettingsv1.ModuleConfigSettings,
) error {
	resources, err := buildMigrationResources(pcc, mc)
	if err != nil {
		return fmt.Errorf("build migration resources: %w", err)
	}

	var buf bytes.Buffer
	for i, resource := range resources {
		if i > 0 {
			buf.WriteString("---\n")
		}
		data, err := yaml.Marshal(resource)
		if err != nil {
			return fmt.Errorf("marshal resource: %w", err)
		}
		buf.Write(data)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      YandexMigrationResourcesName,
			Namespace: ycmeta.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			YandexMigrationResourcesFilename: buf.Bytes(),
		},
	}
	input.PatchCollector.CreateOrUpdate(secret)

	return nil
}

func CreateMigrationConfigMap(input *go_hook.HookInput) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      YandexMigrationConfigMapName,
			Namespace: ycmeta.Namespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   ycmeta.ModuleName,
			},
		},
	}
	input.PatchCollector.CreateOrUpdate(cm)
}

func DeleteMigrationArtifacts(input *go_hook.HookInput) {
	input.PatchCollector.Delete("v1", "Secret", ycmeta.Namespace, YandexMigrationResourcesName)
	input.PatchCollector.Delete("v1", "ConfigMap", ycmeta.Namespace, YandexMigrationConfigMapName)
}

func buildMigrationResources(pcc ycpccv1.YandexProviderClusterConfiguration, mc ycsettingsv1.ModuleConfigSettings) ([]any, error) {
	resources := make([]any, 0)

	// build credential secrets
	for _, s := range BuildCredentialsSecrets(pcc) {
		resources = append(resources, s)
	}

	// build ModuleConfig v2
	mcSettingsV2 := BuildModuleConfigSettingsV2(pcc, mc)
	mcSettingsV2JSON, err := json.Marshal(mcSettingsV2)
	if err != nil {
		return []any{}, fmt.Errorf("marshal settings: %w", err)
	}

	var mcSettingsV2MappedFields deckhousev1alpha1.MappedFields
	if err := json.Unmarshal(mcSettingsV2JSON, &mcSettingsV2MappedFields); err != nil {
		return []any{}, fmt.Errorf("unmarshal settings: %w", err)
	}

	resources = append(resources, deckhousev1alpha1.ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deckhousev1alpha1.SchemeGroupVersion.String(),
			Kind:       "ModuleConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ycmeta.ModuleName,
		},
		Spec: deckhousev1alpha1.ModuleConfigSpec{
			Enabled:  ptr.To(true),
			Version:  2,
			Settings: ptr.To(mcSettingsV2MappedFields),
		},
	})

	// build NodeGroups and YandexInstanceClasses
	if pcc.MasterNodeGroup.Replicas > 0 {
		nodeTemplate := map[string]any{
			"labels": map[string]any{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
			},
		}

		nodeGroup, instanceClass, err := BuildNodeGroupAndInstanceClassResources(
			"master",
			pcc.MasterNodeGroup.Replicas,
			pcc.MasterNodeGroup.Zones,
			pcc.MasterNodeGroup.InstanceClass.YandexInstanceClass,
			nodeTemplate,
			pcc.Zones,
		)
		if err != nil {
			return nil, err
		}

		instanceClass.Spec.EtcdDiskSizeGB = pcc.MasterNodeGroup.InstanceClass.EtcdDiskSizeGB

		resources = append(resources, nodeGroup, instanceClass)
	}

	for _, ng := range pcc.NodeGroups {
		if ng.Name == "" {
			return nil, errors.New("nodeGroups[].name cannot be empty")
		}

		nodeGroup, instanceClass, err := BuildNodeGroupAndInstanceClassResources(
			ng.Name,
			ng.Replicas,
			ng.Zones,
			ng.InstanceClass.YandexInstanceClass,
			ng.NodeTemplate,
			pcc.Zones,
		)
		if err != nil {
			return nil, err
		}

		resources = append(resources, nodeGroup, instanceClass)
	}

	return resources, nil
}

// BuildCredentialsSecrets returns the managed d8-credentials Secret manifest,
// shared by the PCC and hybrid v1 migration paths.
func BuildCredentialsSecrets(pcc ycpccv1.YandexProviderClusterConfiguration) []corev1.Secret {
	secrets := make([]corev1.Secret, 0, 2)

	if len(pcc.Provider.ServiceAccountJSON) > 0 {
		secrets = append(secrets, corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      cpapi.CredentialSecretName,
				Namespace: ycmeta.Namespace,
			},
			Type: cpapi.CredentialsSecretType,
			StringData: map[string]string{
				cpapi.CredentialSecretAuthSchemeKey: string(cpapi.AuthSchemeServiceAccount),
				cpapi.CredentialSecretSecretKey:     pcc.Provider.ServiceAccountJSON,
			},
		})
	}

	if pcc.WithNATInstance != nil && pcc.WithNATInstance.ExporterAPIKey != nil && len(*pcc.WithNATInstance.ExporterAPIKey) > 0 {
		secrets = append(secrets, corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      ycmeta.ExporterCredentialSecretName,
				Namespace: ycmeta.Namespace,
			},
			Type: cpapi.CredentialsSecretType,
			StringData: map[string]string{
				cpapi.CredentialSecretAuthSchemeKey: string(cpapi.AuthSchemeAPIToken),
				cpapi.CredentialSecretSecretKey:     *pcc.WithNATInstance.ExporterAPIKey,
			},
		})
	}

	return secrets
}

func BuildModuleConfigSettingsV2(cfg ycpccv1.YandexProviderClusterConfiguration, mc ycsettingsv1.ModuleConfigSettings) ycsettingsv2.ModuleConfigSettings {
	withNATSettings := ycsettingsv2.NATInstanceParameters{}
	if cfg.WithNATInstance != nil {
		withNATResources := ycsettingsv2.NATInstanceResources{}
		if cfg.WithNATInstance.NATInstanceResources != nil {
			withNATResources = ycsettingsv2.NATInstanceResources{
				Cores:    ptr.Deref(cfg.WithNATInstance.NATInstanceResources.Cores, 0),
				Memory:   ptr.Deref(cfg.WithNATInstance.NATInstanceResources.Memory, 0),
				Platform: ptr.Deref(cfg.WithNATInstance.NATInstanceResources.Platform, ""),
			}
		}

		withNATSettings = ycsettingsv2.NATInstanceParameters{
			ExternalSubnetID:           ptr.Deref(cfg.WithNATInstance.ExternalSubnetID, ""),
			InternalSubnetID:           ptr.Deref(cfg.WithNATInstance.InternalSubnetID, ""),
			InternalSubnetCIDR:         ptr.Deref(cfg.WithNATInstance.InternalSubnetCIDR, ""),
			NATInstanceExternalAddress: ptr.Deref(cfg.WithNATInstance.NATInstanceExternalAddress, ""),
			NATInstanceInternalAddress: ptr.Deref(cfg.WithNATInstance.NATInstanceInternalAddress, ""),
			NATInstanceResources:       withNATResources,
		}
	}

	dhcpOptions := ycsettingsv2.DHCPOptions{}
	if cfg.DHCPOptions != nil {
		dhcpOptions = ycsettingsv2.DHCPOptions{
			DomainName:        ptr.Deref(cfg.DHCPOptions.DomainName, ""),
			DomainNameServers: cfg.DHCPOptions.DomainNameServers,
		}
	}

	settings := ycsettingsv2.ModuleConfigSettings{
		Provider: ycsettingsv2.Provider{
			Parameters: ycsettingsv2.ProviderParameters{
				CloudID:  cfg.Provider.CloudID,
				FolderID: cfg.Provider.FolderID,
			},
		},
		Nodes: ycsettingsv2.Nodes{
			Disabled: false,
			Parameters: ycsettingsv2.NodesParameters{
				SSHPublicKey:              cfg.SSHPublicKey,
				Layout:                    cfg.Layout,
				NodeNetworkCIDR:           cfg.NodeNetworkCIDR,
				WithNATInstance:           withNATSettings,
				ExistingNetworkID:         ptr.Deref(cfg.ExistingNetworkID, ""),
				ExistingZoneToSubnetIDMap: cfg.ExistingZoneToSubnetIDMap,
				DHCPOptions:               dhcpOptions,
				Labels:                    cfg.Labels,
				Zones:                     cfg.Zones,
			},
		},
		Storage: ycsettingsv2.Storage{
			Disabled: false,
			Parameters: ycsettingsv2.StorageParameters{
				ExcludedStorageClasses: mc.StorageClass.Exclude,
			},
		},
		CCM: ycsettingsv2.CCM{
			Disabled: false,
			Parameters: ycsettingsv2.CCMParameters{
				AdditionalExternalNetworkIDs: mc.AdditionalExternalNetworkIDs,
			},
		},
	}

	return settings
}

// BuildNodeGroupAndInstanceClassResources creates a YandexInstanceClass and NodeGroup
// resource pair from typed PCC fields. The instanceClass parameter is the embedded
// ycpccv1.YandexInstanceClass, shared by both YandexMasterInstanceClass and
// YandexNodeGroupInstanceClass.
func BuildNodeGroupAndInstanceClassResources(
	name string,
	replicas int,
	zones []string,
	instanceClass ycpccv1.YandexInstanceClass,
	nodeTemplate map[string]any,
	clusterZones []string,
) (map[string]any, yciccv1.YandexInstanceClass, error) {
	// Build InstanceClass
	instanceClassName := cpapi.BuildInstanceClassName(name)
	icSpec := mapPCCInstanceClassToYandexInstanceClassSpec(instanceClass)
	instanceClassResource := yciccv1.YandexInstanceClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: yciccv1.SchemeGroupVersion.String(),
			Kind:       yciccv1.YandexInstanceClassKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: instanceClassName,
		},
		Spec: icSpec,
	}

	// Build NodeGroup
	cloudInstances := map[string]any{
		"minPerZone": int64(replicas),
		"maxPerZone": int64(replicas),
		"classReference": map[string]any{
			"kind": yciccv1.YandexInstanceClassKind,
			"name": instanceClassName,
		},
	}

	resolvedZones := resolveZones(zones, clusterZones)
	if resolvedZones != nil {
		cloudInstances["zones"] = resolvedZones
	}

	nodeGroupSpec := map[string]any{
		"nodeType":       "CloudPermanent",
		"cloudInstances": cloudInstances,
	}

	if nodeTemplate != nil {
		nodeGroupSpec["nodeTemplate"] = nodeTemplate
	}

	nodeGroupResource := map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata": map[string]any{
			"name": name,
		},
		"spec": nodeGroupSpec,
	}

	return nodeGroupResource, instanceClassResource, nil
}

// mapPCCInstanceClassToYandexInstanceClassSpec converts the PCC-level
// YandexInstanceClass (used in both master and regular node groups) to the
// YandexInstanceClass CRD spec.
func mapPCCInstanceClassToYandexInstanceClassSpec(ic ycpccv1.YandexInstanceClass) yciccv1.YandexInstanceClassSpec {
	spec := yciccv1.YandexInstanceClassSpec{
		Cores:   ic.Cores,
		Memory:  ic.Memory,
		ImageID: ic.ImageID,
	}

	if ic.CoreFraction != nil {
		spec.CoreFraction = *ic.CoreFraction
	}
	if ic.Platform != nil {
		spec.PlatformID = *ic.Platform
	}
	if ic.DiskSizeGB != nil {
		spec.DiskSizeGB = *ic.DiskSizeGB
	}
	if ic.DiskType != nil {
		spec.DiskType = *ic.DiskType
	}
	if ic.NetworkType != nil {
		spec.NetworkType = *ic.NetworkType
	}
	if ic.AdditionalLabels != nil {
		spec.AdditionalLabels = ic.AdditionalLabels
	}
	if ic.ExternalSubnetID != nil {
		spec.MainSubnet = *ic.ExternalSubnetID
	}
	if len(ic.ExternalSubnetIDs) > 0 {
		spec.AdditionalSubnets = ic.ExternalSubnetIDs
	}

	return spec
}

// resolveZones returns the node-group zones, falling back to clusterZones.
// Returns nil (not empty slice) so that a nil Zones engages node-manager's
// defaultZones fallback.
func resolveZones(nodeGroupZones, clusterZones []string) []any {
	if len(nodeGroupZones) > 0 {
		zones := make([]any, 0, len(nodeGroupZones))
		for _, z := range nodeGroupZones {
			if z != "" {
				zones = append(zones, z)
			}
		}
		if len(zones) > 0 {
			return zones
		}
	}

	if len(clusterZones) > 0 {
		zones := make([]any, 0, len(clusterZones))
		for _, z := range clusterZones {
			if z != "" {
				zones = append(zones, z)
			}
		}
		if len(zones) > 0 {
			return zones
		}
	}

	return nil
}
