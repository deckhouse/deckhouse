/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/instanceclass/v1"
	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/pcc/v1"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/hooks/internal/api/settings/v2"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

const (
	masterNodeGroupName = "master"

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
	resources = append(resources, BuildModuleConfig(BuildModuleConfigSettingsV2(pcc)))

	if isHybrid {
		return resources, nil
	}

	if pcc.MasterNodeGroup.Replicas > 0 {
		master, err := BuildNodeGroupAndInstanceClassResources(
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
		resources = append(resources, master...)
	}

	for _, nodeGroup := range pcc.NodeGroups {
		if nodeGroup.Name == "" {
			return nil, fmt.Errorf("nodeGroups[].name cannot be empty")
		}

		pair, err := BuildNodeGroupAndInstanceClassResources(
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
		resources = append(resources, pair...)
	}

	return resources, nil
}

// BuildCredentialSecret returns the managed d8-credentials Secret manifest. zVirt authenticates
// with a login and a password, so the payload uses the userPassword scheme.
func BuildCredentialSecret(pcc zpccv1.ZvirtProviderClusterConfiguration) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      cpapi.CredentialSecretName,
			"namespace": Namespace,
		},
		"type": cpapi.CredentialsSecretType,
		"stringData": map[string]any{
			cpapi.CredentialSecretAuthSchemeKey: string(cpapi.AuthSchemeUserPassword),
			cpapi.CredentialSecretIdentityKey:   pcc.Provider.Username,
			cpapi.CredentialSecretSecretKey:     pcc.Provider.Password,
		},
	}
}

// BuildModuleConfig wraps settings into the ModuleConfig v2 manifest.
func BuildModuleConfig(settings zsettingsv2.ModuleConfigSettings) map[string]any {
	return map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata": map[string]any{
			"name": ModuleName,
		},
		"spec": map[string]any{
			"enabled": true,
			"version": 2,
			"settings": map[string]any{
				"provider": map[string]any{
					"parameters": settings.Provider.Parameters,
				},
				"nodes": map[string]any{
					"parameters": settings.Nodes.Parameters,
				},
				"storage": map[string]any{
					"parameters": map[string]any{},
				},
			},
		},
	}
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
				SSHPublicKey: sshPublicKey,
				Layout:       layout,
			},
		},
	}
}

// BuildNodeGroupAndInstanceClassResources creates a ZvirtInstanceClass and NodeGroup pair for one
// node group of the legacy configuration.
func BuildNodeGroupAndInstanceClassResources(
	name string,
	replicas int,
	instanceClass zpccv1.ZvirtInstanceClass,
	etcdDiskSizeGb *int,
	nodeTemplate map[string]any,
	master bool,
) ([]any, error) {
	if instanceClass.Template == "" || instanceClass.VNICProfileID == "" {
		return nil, fmt.Errorf("%s.instanceClass must define template and vnicProfileID", name)
	}

	instanceClassName := cpapi.BuildInstanceClassName(name)

	spec := MapPCCInstanceClassToSpec(instanceClass, etcdDiskSizeGb, master)

	instanceClassResource := map[string]any{
		"apiVersion": zicv1.ZvirtInstanceClassAPIVersion,
		"kind":       zicv1.ZvirtInstanceClassKind,
		"metadata": map[string]any{
			"name": instanceClassName,
		},
		"spec": spec,
	}

	nodeGroupSpec := map[string]any{
		"nodeType": string(cpapi.NodeTypeCloudPermanent),
		"cloudInstances": map[string]any{
			"minPerZone": replicas,
			"maxPerZone": replicas,
			"classReference": map[string]any{
				"kind": zicv1.ZvirtInstanceClassKind,
				"name": instanceClassName,
			},
		},
	}

	switch {
	case master:
		nodeGroupSpec["nodeTemplate"] = map[string]any{
			"labels": map[string]any{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
			},
		}
	case len(nodeTemplate) > 0:
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

	return []any{instanceClassResource, nodeGroupResource}, nil
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
		RootDiskSizeGb:  &rootDiskSizeGb,
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

	if instanceClass.CustomNetworkConfig != nil {
		spec.CustomNetworkConfig = &zicv1.CustomNetworkConfig{
			NetworkInterfaceName:    instanceClass.CustomNetworkConfig.NetworkInterfaceName,
			NetworkInterfaceAddress: instanceClass.CustomNetworkConfig.NetworkInterfaceAddress,
			NetworkInterfaceNetmask: instanceClass.CustomNetworkConfig.NetworkInterfaceNetmask,
			NetworkInterfaceGateway: instanceClass.CustomNetworkConfig.NetworkInterfaceGateway,
			DNSServers:              strings.Fields(instanceClass.CustomNetworkConfig.DNSServers),
		}
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
