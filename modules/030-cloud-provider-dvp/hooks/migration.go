/*
Copyright 2025 Flant JSC

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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	v1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/hooks/internal/v1"
)

const (
	dvpModuleName                 = "cloud-provider-dvp"
	dvpNamespace                  = "d8-cloud-provider-dvp"
	dvpMigrationResourcesName     = "d8-migration-resources"
	dvpMigrationResourcesFilename = "resources.yaml"
	dvpMigrationConfigMapName     = "d8-module-is-migrating"
	dvpCredentialSecretName       = "d8-credentials"
	dvpInstanceClassKind          = "DVPInstanceClass"
	dvpInstanceClassAPI           = "deckhouse.io/v1alpha1"
	dvpAuthSchemeKubeconfig       = "kubeconfig"
	dvpCredentialSecretType       = "cloud-provider.deckhouse.io/credentials"
	moduleConfigAPIVersion        = "deckhouse.io/v1alpha1"
	pccSecretName                 = "d8-provider-cluster-configuration"
	dvpCandiDiscoverySecretName   = "d8-candi-cloud-provider-discovery-data"
)

func createProviderClusterConfigurationResources(input *go_hook.HookInput, cfg *v1.DvpProviderClusterConfiguration) error {
	if cfg == nil || cfg.Provider == nil || cfg.Provider.KubeconfigDataBase64 == nil || cfg.Provider.Namespace == nil {
		return nil
	}

	resources := make([]any, 0, 4+len(cfg.NodeGroups))
	resources = append(resources, buildD8CredentialsSecret(*cfg.Provider.KubeconfigDataBase64))

	moduleConfig, err := buildModuleConfigFromPCC(cfg)
	if err != nil {
		return err
	}
	resources = append(resources, moduleConfig)

	masterNodeGroup, err := mapFromAny(cfg.MasterNodeGroup)
	if err != nil {
		return fmt.Errorf("convert masterNodeGroup: %w", err)
	}

	if len(masterNodeGroup) != 0 {
		masterResources, err := buildNodeGroupAndInstanceClassResources("master", masterNodeGroup, true, cfg.Zones)
		if err != nil {
			return err
		}
		resources = append(resources, masterResources...)
	}

	for _, rawNodeGroup := range cfg.NodeGroups {
		nodeGroup, err := mapFromAny(rawNodeGroup)
		if err != nil {
			return fmt.Errorf("convert nodeGroup: %w", err)
		}

		name, ok := nodeGroup["name"].(string)
		if !ok || name == "" {
			return errors.New("nodeGroups[].name cannot be empty")
		}

		nodeGroupResources, err := buildNodeGroupAndInstanceClassResources(name, nodeGroup, false, cfg.Zones)
		if err != nil {
			return err
		}
		resources = append(resources, nodeGroupResources...)
	}

	return createMigrationResourcesSecret(input, resources)
}

// buildD8CredentialsSecret returns the managed d8-credentials Secret manifest,
// shared by the PCC and hybrid v1 migration paths.
func buildD8CredentialsSecret(kubeconfigDataBase64 string) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      dvpCredentialSecretName,
			"namespace": dvpNamespace,
		},
		"type": dvpCredentialSecretType,
		"stringData": map[string]any{
			"authScheme": dvpAuthSchemeKubeconfig,
			"secret":     kubeconfigDataBase64,
		},
	}
}

// buildModuleConfigFromPCC builds the ModuleConfig v2 manifest for the PCC (cloud
// DVP) migration path. Provider and nodes settings are derived from the
// ProviderClusterConfiguration: namespace, layout, sshPublicKey, region, zones
// and the per-NodeGroup ipAddresses aggregated from the master and worker
// NodeGroups. storage.parameters is emitted empty and disabled flags are omitted
// so schema defaults apply, matching buildModuleConfigForHybrid.
func buildModuleConfigFromPCC(cfg *v1.DvpProviderClusterConfiguration) (map[string]any, error) {
	providerSettings := map[string]any{
		"parameters": map[string]any{
			"namespace": *cfg.Provider.Namespace,
		},
	}

	layout := ""
	if cfg.Layout != nil {
		layout = *cfg.Layout
	}
	sshPublicKey := ""
	if cfg.SSHPublicKey != nil {
		sshPublicKey = *cfg.SSHPublicKey
	}
	nodesParameters := map[string]any{
		"layout":       layout,
		"sshPublicKey": sshPublicKey,
	}
	if cfg.Region != nil {
		nodesParameters["region"] = *cfg.Region
	}
	if cfg.Zones != nil {
		nodesParameters["zones"] = stringsToAnySlice(*cfg.Zones)
	}

	masterNodeGroup, err := mapFromAny(cfg.MasterNodeGroup)
	if err != nil {
		return nil, fmt.Errorf("convert masterNodeGroup: %w", err)
	}

	ipAddressesMap := make(map[string][]string)
	if addrs := extractIPAddresses("master", masterNodeGroup); len(addrs) > 0 {
		ipAddressesMap["master"] = addrs
	}

	for _, rawNodeGroup := range cfg.NodeGroups {
		nodeGroup, err := mapFromAny(rawNodeGroup)
		if err != nil {
			return nil, fmt.Errorf("convert nodeGroup: %w", err)
		}

		ngName, _ := nodeGroup["name"].(string)
		if ngName == "" {
			continue
		}

		if addrs := extractIPAddresses(ngName, nodeGroup); len(addrs) > 0 {
			ipAddressesMap[ngName] = addrs
		}
	}

	if len(ipAddressesMap) > 0 {
		nodesParameters["ipAddresses"] = ipAddressesMap
	}

	return map[string]any{
		"apiVersion": moduleConfigAPIVersion,
		"kind":       "ModuleConfig",
		"metadata": map[string]any{
			"name": dvpModuleName,
		},
		"spec": map[string]any{
			"enabled": true,
			"version": int(2),
			"settings": map[string]any{
				"provider": providerSettings,
				"storage": map[string]any{
					"parameters": map[string]any{},
				},
				"nodes": map[string]any{
					"parameters": nodesParameters,
				},
			},
		},
	}, nil
}

// buildModuleConfigForHybrid builds the ModuleConfig v2 manifest for the hybrid v1
// migration path. The shape mirrors the declarative v1->v2 conversion in
// openapi/conversions/v2.yaml: the legacy provider.namespace becomes
// provider.parameters.namespace (defaulting to "default"), the legacy top-level
// zones move into nodes.parameters.zones, kubeconfigDataBase64 is dropped (it now
// lives in the d8-credentials Secret), and nodes.parameters gets the required
// layout/sshPublicKey placeholders the admin must review.
//
// disabled flags (nodes/storage/ccm) are intentionally omitted: they carry schema
// defaults, matching createProviderClusterConfigurationResources.
//
// The legacy provider.namespace, provider.kubeconfigDataBase64 and top-level zones
// fields are emitted with a null value on purpose. A hybrid cluster already has a
// stored v1 ModuleConfig, and this bundle is applied by the admin via
// `kubectl apply -f -`. Without a prior last-applied-configuration annotation,
// kubectl computes a JSON-merge-patch that only ADDS the new v2 keys, leaving the
// old v1 keys in place; the merged object then fails v2 validation (provider is
// additionalProperties:false). JSON-merge-patch treats null as a delete marker, so
// these tombstones strip the stale v1 keys on apply.
func buildModuleConfigForHybrid(namespace string, zones []string) map[string]any {
	if namespace == "" {
		namespace = "default"
	}

	nodesParameters := map[string]any{
		"layout":       "Standard",
		"sshPublicKey": "ssh-rsa PLACEHOLDER_REPLACE_ME",
	}
	if len(zones) > 0 {
		nodesParameters["zones"] = stringsToAnySlice(zones)
	}

	return map[string]any{
		"apiVersion": moduleConfigAPIVersion,
		"kind":       "ModuleConfig",
		"metadata": map[string]any{
			"name": dvpModuleName,
		},
		"spec": map[string]any{
			"enabled": true,
			"version": int(2),
			"settings": map[string]any{
				"provider": map[string]any{
					"namespace":            nil,
					"kubeconfigDataBase64": nil,
					"parameters": map[string]any{
						"namespace": namespace,
					},
				},
				"zones": nil,
				"storage": map[string]any{
					"parameters": map[string]any{},
				},
				"nodes": map[string]any{
					"parameters": nodesParameters,
				},
			},
		},
	}
}

func createMigrationResourcesSecret(input *go_hook.HookInput, resources []any) error {
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
			Name:      dvpMigrationResourcesName,
			Namespace: dvpNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			dvpMigrationResourcesFilename: manifest,
		},
	}
	input.PatchCollector.CreateOrUpdate(secret)

	return nil
}

func createMigrationConfigMap(input *go_hook.HookInput) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dvpMigrationConfigMapName,
			Namespace: dvpNamespace,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   dvpModuleName,
			},
		},
	}
	input.PatchCollector.CreateOrUpdate(cm)
}

func deleteMigrationArtifacts(input *go_hook.HookInput) {
	input.PatchCollector.Delete("v1", "Secret", dvpNamespace, dvpMigrationResourcesName)
	input.PatchCollector.Delete("v1", "ConfigMap", dvpNamespace, dvpMigrationConfigMapName)
}

// func cleanupProviderClusterConfiguration(input *go_hook.HookInput) {
// 	patch := map[string]any{
// 		"data": map[string]any{
// 			pccClusterConfigKey: nil,
// 		},
// 	}
// 	input.PatchCollector.PatchWithMerge(patch, "v1", "Secret", pccSecretNamespace, pccSecretName,
// 		object_patch.WithIgnoreMissingObject())
// }

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

func buildNodeGroupAndInstanceClassResources(name string, nodeGroup map[string]any, master bool, clusterZones *[]string) ([]any, error) {
	instanceClassSpec, ok := nodeGroup["instanceClass"].(map[string]any)
	if !ok || len(instanceClassSpec) == 0 {
		return nil, fmt.Errorf("%s.instanceClass cannot be empty", name)
	}

	if vm, ok := instanceClassSpec["virtualMachine"].(map[string]any); ok {
		if _, hasIP := vm["ipAddresses"]; hasIP {
			vmCopy := make(map[string]any, len(vm))
			maps.Copy(vmCopy, vm)
			delete(vmCopy, "ipAddresses")
			specCopy := make(map[string]any, len(instanceClassSpec))
			maps.Copy(specCopy, instanceClassSpec)
			specCopy["virtualMachine"] = vmCopy
			instanceClassSpec = specCopy
		}
	}

	instanceClassName := cpapi.BuildInstanceClassName(name)
	instanceClass := map[string]any{
		"apiVersion": dvpInstanceClassAPI,
		"kind":       dvpInstanceClassKind,
		"metadata": map[string]any{
			"name": instanceClassName,
		},
		"spec": instanceClassSpec,
	}

	replicas, err := replicasFromNodeGroup(nodeGroup)
	if err != nil {
		return nil, fmt.Errorf("%s.replicas: %w", name, err)
	}

	cloudInstances := map[string]any{
		"minPerZone": int64(replicas),
		"maxPerZone": int64(replicas),
		"classReference": map[string]any{
			"kind": dvpInstanceClassKind,
			"name": instanceClassName,
		},
	}

	zones := zonesFromNodeGroup(nodeGroup, clusterZones)
	if zones != nil {
		cloudInstances["zones"] = zones
	}

	nodeGroupSpec := map[string]any{
		"nodeType":       "CloudPermanent",
		"cloudInstances": cloudInstances,
	}
	if nodeTemplate, ok := nodeGroup["nodeTemplate"]; ok {
		nodeGroupSpec["nodeTemplate"] = nodeTemplate
	}
	if master {
		nodeGroupSpec["nodeTemplate"] = map[string]any{
			"labels": map[string]any{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "",
			},
		}
	}

	nodeGroupResource := map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata": map[string]any{
			"name": name,
		},
		"spec": nodeGroupSpec,
	}

	return []any{instanceClass, nodeGroupResource}, nil
}

func mapFromAny(value any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func replicasFromNodeGroup(nodeGroup map[string]any) (int, error) {
	replicas, ok := nodeGroup["replicas"]
	if !ok {
		return 0, errors.New("cannot be empty")
	}
	// json.Unmarshal always yields float64 for JSON numbers
	v, ok := replicas.(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected type %T", replicas)
	}
	return int(v), nil
}

func zonesFromNodeGroup(nodeGroup map[string]any, clusterZones *[]string) []any {
	if rawZones, ok := nodeGroup["zones"].([]any); ok && len(rawZones) > 0 {
		zones := make([]any, 0, len(rawZones))
		for _, rawZone := range rawZones {
			if zone, ok := rawZone.(string); ok && zone != "" {
				zones = append(zones, zone)
			}
		}
		if len(zones) > 0 {
			return zones
		}
	}

	if clusterZones != nil && len(*clusterZones) > 0 {
		return stringsToAnySlice(*clusterZones)
	}

	// Return nil (not an empty slice) so the rendered NodeGroup has zones: null.
	// node-manager's get_crds.go distinguishes nil from []string{}: a nil Zones
	// field engages its defaultZones fallback, while a non-nil empty slice does
	// not. Returning nil keeps the migrated NodeGroup faithful to a source PCC
	// that had no zones, and lets node-manager apply its own fallback.
	return nil
}

func stringsToAnySlice(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func extractIPAddresses(_ string, nodeGroup map[string]any) []string {
	ic, ok := nodeGroup["instanceClass"].(map[string]any)
	if !ok {
		return nil
	}
	vm, ok := ic["virtualMachine"].(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := vm["ipAddresses"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	addrs := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			addrs = append(addrs, s)
		}
	}
	return addrs
}
