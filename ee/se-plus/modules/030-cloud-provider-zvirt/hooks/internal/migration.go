/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/instanceclass/v1"
	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/pcc/v1"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/settings/v2"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

const (
	masterNodeGroupName = "master"

	nodeGroupAPIVersion = "deckhouse.io/v1"
	nodeGroupKind       = "NodeGroup"

	defaultRootDiskSizeGb = 50
	defaultEtcdDiskSizeGb = 10
)

// IsMigrationResourcesApplied reports whether the admin has already applied the migration bundle,
// i.e. the new model fully describes the cluster and the legacy PCC no longer drives it.
func IsMigrationResourcesApplied(input *go_hook.HookInput, pcc zpccv1.ZvirtProviderClusterConfiguration) bool {
	return HasCredentialSecret(input) &&
		HasMigratedModuleConfig(input) &&
		HasMigratedNodeGroupsAndInstances(input, pcc)
}

// HasCredentialSecret reports whether the managed credential Secret exists in the cluster.
// The match is by name so that an unrelated Secret of the same type cannot pass for it.
func HasCredentialSecret(input *go_hook.HookInput) bool {
	secrets, err := sdkobjectpatch.UnmarshalToStruct[CredentialSecretFilterResult](input.Snapshots, "credential_secrets")
	if err != nil {
		return false
	}

	for _, secret := range secrets {
		if secret.Name == cpapi.CredentialSecretName {
			return true
		}
	}

	return false
}

// HasMigratedModuleConfig reports whether the cluster carries an enabled ModuleConfig v2 with
// settings, i.e. the new configuration model is in place.
func HasMigratedModuleConfig(input *go_hook.HookInput) bool {
	results, err := sdkobjectpatch.UnmarshalToStruct[ModuleConfigFilterResult](input.Snapshots, "module_config")
	if err != nil || len(results) == 0 {
		return false
	}

	mc := results[0]

	return mc.Version >= 2 && mc.Enabled && mc.SettingsV2 != nil
}

// HasMigratedNodeGroupsAndInstances reports whether every node group the legacy PCC describes
// already exists as a NodeGroup plus ZvirtInstanceClass pair.
func HasMigratedNodeGroupsAndInstances(input *go_hook.HookInput, pcc zpccv1.ZvirtProviderClusterConfiguration) bool {
	existingNodeGroups, err := sdkobjectpatch.UnmarshalToStruct[NodeGroupFilterResult](input.Snapshots, "node_groups")
	if err != nil {
		return false
	}
	nodeGroupSet := make(map[string]bool, len(existingNodeGroups))
	for _, ng := range existingNodeGroups {
		nodeGroupSet[ng.Name] = true
	}

	existingClasses, err := sdkobjectpatch.UnmarshalToStruct[NamedResourceFilterResult](input.Snapshots, "zvirt_instance_classes")
	if err != nil {
		return false
	}
	classSet := make(map[string]bool, len(existingClasses))
	for _, ic := range existingClasses {
		classSet[ic.Name] = true
	}

	// A hybrid cluster has no cloud master NodeGroup to migrate.
	isHybrid := IsHybridCluster(existingNodeGroups)
	if pcc.MasterNodeGroup.Replicas > 0 && !isHybrid {
		if !nodeGroupSet[masterNodeGroupName] || !classSet[cpapi.BuildInstanceClassName(masterNodeGroupName)] {
			return false
		}
	}

	for _, nodeGroup := range pcc.NodeGroups {
		if nodeGroup.Name == "" {
			continue
		}

		if !nodeGroupSet[nodeGroup.Name] || !classSet[cpapi.BuildInstanceClassName(nodeGroup.Name)] {
			return false
		}
	}

	return true
}

// IsHybridCluster reports whether the cluster is hybrid: its master NodeGroup is Static, i.e.
// provisioned outside of DKP. Such a cluster never runs the DKP infrastructure, so the migration
// must not project a CloudPermanent master NodeGroup for it.
func IsHybridCluster(nodeGroups []NodeGroupFilterResult) bool {
	for _, ng := range nodeGroups {
		if ng.Name == masterNodeGroupName {
			return ng.NodeType == string(cpapi.NodeTypeStatic)
		}
	}

	return false
}

// CreateMigrationResourcesSecret stores the bundle an admin applies with `kubectl apply -f -`.
func CreateMigrationResourcesSecret(
	input *go_hook.HookInput,
	pcc zpccv1.ZvirtProviderClusterConfiguration,
	isHybrid bool,
) error {
	resources, err := BuildMigrationResources(pcc, isHybrid)
	if err != nil {
		return fmt.Errorf("build migration resources: %w", err)
	}

	manifest, err := marshalResourcesManifest(resources)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpapi.MigrationSecretName,
			Namespace: Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			MigrationResourcesFilename: manifest,
		},
	}
	input.PatchCollector.CreateOrUpdate(secret)

	return nil
}

// CreateMigrationConfigMap marks the migration as in progress. While the ConfigMap exists,
// cpapi.ShouldSkipNewModelValidation keeps new-model validation switched off, so a half-migrated
// cluster does not fail admission.
func CreateMigrationConfigMap(input *go_hook.HookInput) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpapi.MigrationConfigMapName,
			Namespace: Namespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   ModuleName,
			},
		},
	}
	input.PatchCollector.CreateOrUpdate(cm)
}

func DeleteMigrationArtifacts(input *go_hook.HookInput) {
	input.PatchCollector.Delete("v1", "Secret", Namespace, cpapi.MigrationSecretName)
	input.PatchCollector.Delete("v1", "ConfigMap", Namespace, cpapi.MigrationConfigMapName)
}

// BuildMigrationResources assembles the manifests the admin has to apply: the credential Secret,
// the ModuleConfig v2, and a NodeGroup plus ZvirtInstanceClass pair per node group.
//
// A hybrid cluster gets only the Secret and the ModuleConfig: its nodes are either static or
// already user-managed CloudEphemeral resources, so generating NodeGroups for them would fight
// with what the admin already declared.
func BuildMigrationResources(pcc zpccv1.ZvirtProviderClusterConfiguration, isHybrid bool) ([]any, error) {
	resources := make([]any, 0, 2+2*(1+len(pcc.NodeGroups)))

	resources = append(resources, BuildCredentialSecret(pcc))

	moduleConfig, err := BuildModuleConfig(pcc)
	if err != nil {
		return nil, err
	}
	resources = append(resources, moduleConfig)

	if isHybrid {
		return resources, nil
	}

	if pcc.MasterNodeGroup.Replicas > 0 {
		nodeGroup, instanceClass, err := BuildNodeGroupAndInstanceClassResources(
			masterNodeGroupName,
			pcc.MasterNodeGroup.Replicas,
			pcc.MasterNodeGroup.InstanceClass.ZvirtInstanceClass,
			pcc.MasterNodeGroup.InstanceClass.EtcdDiskSizeGb,
			nil,
			true,
		)
		if err != nil {
			return nil, err
		}
		resources = append(resources, instanceClass, nodeGroup)
	}

	for _, nodeGroup := range pcc.NodeGroups {
		if nodeGroup.Name == "" {
			return nil, fmt.Errorf("nodeGroups[].name cannot be empty")
		}

		nodeGroupResource, instanceClass, err := BuildNodeGroupAndInstanceClassResources(
			nodeGroup.Name,
			nodeGroup.Replicas,
			nodeGroup.InstanceClass.ZvirtInstanceClass,
			nil,
			nodeGroup.NodeTemplate,
			false,
		)
		if err != nil {
			return nil, err
		}
		resources = append(resources, instanceClass, nodeGroupResource)
	}

	return resources, nil
}

// BuildCredentialSecret returns the managed d8-credentials Secret manifest. zVirt authenticates
// with a login and a password, so the payload uses the userPassword scheme.
func BuildCredentialSecret(pcc zpccv1.ZvirtProviderClusterConfiguration) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpapi.CredentialSecretName,
			Namespace: Namespace,
		},
		Type: cpapi.CredentialsSecretType,
		StringData: map[string]string{
			cpapi.CredentialSecretAuthSchemeKey: string(cpapi.AuthSchemeUserPassword),
			cpapi.CredentialSecretIdentityKey:   pcc.Provider.Username,
			cpapi.CredentialSecretSecretKey:     pcc.Provider.Password,
		},
	}
}

// BuildModuleConfig wraps settings into the ModuleConfig v2 manifest.
func BuildModuleConfig(pcc zpccv1.ZvirtProviderClusterConfiguration) (deckhousev1alpha1.ModuleConfig, error) {
	mcSettingsV2 := BuildModuleConfigSettingsV2(pcc)
	mcSettingsV2JSON, err := json.Marshal(mcSettingsV2)
	if err != nil {
		return deckhousev1alpha1.ModuleConfig{}, fmt.Errorf("marshal settings: %w", err)
	}

	var mcSettingsV2MappedFields deckhousev1alpha1.MappedFields
	if err := json.Unmarshal(mcSettingsV2JSON, &mcSettingsV2MappedFields); err != nil {
		return deckhousev1alpha1.ModuleConfig{}, fmt.Errorf("unmarshal settings: %w", err)
	}

	return deckhousev1alpha1.ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deckhousev1alpha1.SchemeGroupVersion.String(),
			Kind:       deckhousev1alpha1.ModuleConfigKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ModuleName,
		},
		Spec: deckhousev1alpha1.ModuleConfigSpec{
			Enabled:  ptr.To(true),
			Version:  2,
			Settings: ptr.To(mcSettingsV2MappedFields),
		},
	}, nil
}

// BuildModuleConfigSettingsV2 projects the legacy ZvirtClusterConfiguration onto the v2 settings.
//
// The login and the password are deliberately not part of the result: they belong to the
// credential Secret. Placeholders stand in for fields a PCC-less (hybrid) cluster cannot supply,
// and they match openapi/conversions/v2.yaml so that an admin sees the same marker either way.
func BuildModuleConfigSettingsV2(pcc zpccv1.ZvirtProviderClusterConfiguration) zsettingsv2.ModuleConfigSettings {
	server := pcc.Provider.Server
	if server == "" {
		server = PlaceholderServer
	}

	clusterID := pcc.ClusterID
	if clusterID == "" {
		clusterID = PlaceholderClusterID
	}

	sshPublicKey := pcc.SSHPublicKey
	if sshPublicKey == "" {
		sshPublicKey = PlaceholderSSHPublicKey
	}

	layout := pcc.Layout
	if layout == "" {
		layout = DefaultLayout
	}

	return zsettingsv2.ModuleConfigSettings{
		Provider: zsettingsv2.Provider{
			Parameters: zsettingsv2.ProviderParameters{
				Server:    server,
				ClusterID: clusterID,
				CABundle:  pcc.Provider.CABundle,
				Insecure:  pcc.Provider.Insecure,
			},
		},
		Nodes: zsettingsv2.Nodes{
			Parameters: zsettingsv2.NodesParameters{
				SSHPublicKey:         sshPublicKey,
				Layout:               layout,
				CustomNetworkConfigs: buildCustomNetworkConfigs(pcc),
			},
		},
	}
}

// buildCustomNetworkConfigs lifts the per-InstanceClass static network configuration of the legacy
// configuration into the settings map keyed by NodeGroup name.
//
// The legacy configuration attaches it to the InstanceClass of a node group, which made it
// reachable from CloudEphemeral groups it was never applied to. Keying by NodeGroup name says what
// was always true: the configuration belongs to a CloudPermanent group, whose node indices the
// addresses are handed out by.
func buildCustomNetworkConfigs(pcc zpccv1.ZvirtProviderClusterConfiguration) map[string]zsettingsv2.CustomNetworkConfig {
	configs := make(map[string]zsettingsv2.CustomNetworkConfig)

	if pcc.MasterNodeGroup.Replicas > 0 {
		if config, ok := mapPCCCustomNetworkConfig(pcc.MasterNodeGroup.InstanceClass.CustomNetworkConfig); ok {
			configs[masterNodeGroupName] = config
		}
	}

	for _, nodeGroup := range pcc.NodeGroups {
		if nodeGroup.Name == "" {
			continue
		}

		if config, ok := mapPCCCustomNetworkConfig(nodeGroup.InstanceClass.CustomNetworkConfig); ok {
			configs[nodeGroup.Name] = config
		}
	}

	if len(configs) == 0 {
		return nil
	}

	return configs
}

// mapPCCCustomNetworkConfig converts one legacy static network configuration. The legacy DNS
// servers are one space-separated string; the settings take a list.
func mapPCCCustomNetworkConfig(config *zpccv1.ZvirtNetworkConfig) (zsettingsv2.CustomNetworkConfig, bool) {
	if config == nil {
		return zsettingsv2.CustomNetworkConfig{}, false
	}

	return zsettingsv2.CustomNetworkConfig{
		NetworkInterfaceName:      config.NetworkInterfaceName,
		NetworkInterfaceAddresses: config.NetworkInterfaceAddress,
		NetworkInterfaceNetmask:   config.NetworkInterfaceNetmask,
		NetworkInterfaceGateway:   config.NetworkInterfaceGateway,
		DNSServers:                strings.Fields(config.DNSServers),
	}, true
}

// BuildNodeGroupAndInstanceClassResources creates a ZvirtInstanceClass and NodeGroup pair for one
// node group of the legacy configuration.
func BuildNodeGroupAndInstanceClassResources(
	name string,
	replicas int,
	instanceClass zpccv1.ZvirtInstanceClass,
	etcdDiskSizeGb *int,
	nodeTemplate *cpapi.NodeTemplate,
	master bool,
) (cpapi.NodeGroup, zicv1.ZvirtInstanceClass, error) {
	if instanceClass.Template == "" || instanceClass.VNICProfileID == "" {
		return cpapi.NodeGroup{}, zicv1.ZvirtInstanceClass{}, fmt.Errorf("%s.instanceClass must define template and vnicProfileID", name)
	}

	instanceClassName := cpapi.BuildInstanceClassName(name)

	instanceClassResource := zicv1.ZvirtInstanceClass{
		TypeMeta: metav1.TypeMeta{
			APIVersion: zicv1.GroupVersionKind.GroupVersion().String(),
			Kind:       zicv1.ZvirtInstanceClassKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: instanceClassName,
		},
		Spec: MapPCCInstanceClassToSpec(instanceClass, etcdDiskSizeGb, master),
	}

	// The master node group carries the control-plane labels; every other group keeps whatever
	// the legacy configuration declared.
	if master {
		nodeTemplate = &cpapi.NodeTemplate{
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
			},
		}
	}

	nodeGroupResource := cpapi.NodeGroup{
		TypeMeta: cpapi.TypeMeta{
			APIVersion: nodeGroupAPIVersion,
			Kind:       nodeGroupKind,
		},
		ObjectMeta: cpapi.ObjectMeta{
			Name: name,
		},
		Spec: cpapi.NodeGroupSpec{
			NodeType: cpapi.NodeTypeCloudPermanent,
			CloudInstances: &cpapi.CloudInstances{
				ClassReference: &cpapi.ClassReference{
					Kind: zicv1.ZvirtInstanceClassKind,
					Name: instanceClassName,
				},
				MinPerZone: replicas,
				MaxPerZone: replicas,
			},
			NodeTemplate: nodeTemplate,
		},
	}

	return nodeGroupResource, instanceClassResource, nil
}

// MapPCCInstanceClassToSpec converts the instanceClass of a legacy node group into a
// ZvirtInstanceClass spec.
//
// A dedicated etcd disk exists on the master node group only, so etcdDiskSizeGb is filled in for
// the master and left absent everywhere else. Both disk sizes always end up explicit: see the
// note on defaultRootDiskSizeGb.
func MapPCCInstanceClassToSpec(instanceClass zpccv1.ZvirtInstanceClass, etcdDiskSizeGb *int, master bool) zicv1.InstanceClassSpec {
	rootDiskSizeGb := defaultRootDiskSizeGb
	if instanceClass.RootDiskSizeGb != nil {
		rootDiskSizeGb = *instanceClass.RootDiskSizeGb
	}

	spec := zicv1.InstanceClassSpec{
		NumCPUs:         instanceClass.NumCPUs,
		Memory:          instanceClass.Memory,
		RootDiskSizeGb:  rootDiskSizeGb,
		Template:        instanceClass.Template,
		VNICProfileID:   instanceClass.VNICProfileID,
		StorageDomainID: instanceClass.StorageDomainID,
	}

	if master {
		etcdSize := defaultEtcdDiskSizeGb
		if etcdDiskSizeGb != nil {
			etcdSize = *etcdDiskSizeGb
		}
		spec.EtcdDiskSizeGb = &etcdSize
	}

	return spec
}

func marshalResourcesManifest(resources []any) ([]byte, error) {
	var buffer bytes.Buffer
	for index, resource := range resources {
		if index > 0 {
			buffer.WriteString("---\n")
		}

		data, err := yaml.Marshal(resource)
		if err != nil {
			return nil, err
		}
		buffer.Write(data)
	}

	return buffer.Bytes(), nil
}
