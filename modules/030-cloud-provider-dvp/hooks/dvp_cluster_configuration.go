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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/cluster_configuration"
	v1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/hooks/internal/v1"
)

var _ = cluster_configuration.RegisterHook(func(input *go_hook.HookInput, metaCfg *config.MetaConfig, providerDiscoveryData *unstructured.Unstructured, _ bool) error {
	p := make(map[string]json.RawMessage)
	if metaCfg != nil {
		p = metaCfg.ProviderClusterConfig
	}

	var providerClusterConfiguration v1.DvpProviderClusterConfiguration
	err := convertJSONRawMessageToStruct(p, &providerClusterConfiguration)
	if err != nil {
		return err
	}

	var moduleConfiguration v1.DvpModuleConfiguration
	err = json.Unmarshal([]byte(input.Values.Get("cloudProviderDvp").String()), &moduleConfiguration)
	if err != nil {
		return err
	}

	err = overrideValues(&providerClusterConfiguration, &moduleConfiguration)
	if err != nil {
		return err
	}
	input.Values.Set("cloudProviderDvp.internal.providerClusterConfiguration", providerClusterConfiguration)

	err = createProviderClusterConfigurationResources(input, &providerClusterConfiguration)
	if err != nil {
		return err
	}

	var discoveryData cloudDataV1.DVPCloudProviderDiscoveryData
	if providerDiscoveryData != nil {
		err := sdk.FromUnstructured(providerDiscoveryData, &discoveryData)
		if err != nil {
			return err
		}
	}

	providerDiscoveryDataValuesJSON, ok := input.Values.GetOk("cloudProviderDvp.internal.providerDiscoveryData")
	if ok && len(providerDiscoveryDataValuesJSON.String()) != 0 {
		var providerDiscoveryDataValues cloudDataV1.DVPCloudProviderDiscoveryData
		err = json.Unmarshal([]byte(providerDiscoveryDataValuesJSON.String()), &providerDiscoveryDataValues)
		if err != nil {
			return err
		}
		discoveryData = mergeDiscoveryData(discoveryData, providerDiscoveryDataValues)
	}

	if discoveryData.APIVersion == "" {
		discoveryData.APIVersion = "deckhouse.io/v1"
	}

	if discoveryData.Kind == "" {
		discoveryData.Kind = "DVPCloudDiscoveryData"
	}

	if len(discoveryData.Zones) == 0 {
		discoveryData.Zones = []string{"default"}
	}

	input.Values.Set("cloudProviderDvp.internal.providerDiscoveryData", discoveryData)

	return nil
}, cluster_configuration.NewConfig(infrastructureprovider.MetaConfigPreparatorProvider(infrastructureprovider.NewPreparatorProviderParamsWithoutLogger())))

const (
	dvpModuleConfigName           = "cloud-provider-dvp"
	dvpMigrationResourcesName     = "d8-migration-resources"
	dvpMigrationResourcesFilename = "resources.yaml"
	dvpCredentialSecretName       = "d8-cloud-provider-dvp-credentials"
	dvpInstanceClassKind          = "DVPInstanceClass"
	dvpInstanceClassAPI           = "deckhouse.io/v1alpha1"
	dvpDefaultInstanceSuffix      = "dvp"
)

func createProviderClusterConfigurationResources(input *go_hook.HookInput, cfg *v1.DvpProviderClusterConfiguration) error {
	if cfg == nil || cfg.Provider == nil || cfg.Provider.KubeconfigDataBase64 == nil || cfg.Provider.Namespace == nil {
		return nil
	}

	providerSettings := map[string]any{
		"parameters": map[string]any{
			"namespace": *cfg.Provider.Namespace,
		},
	}

	nodesSettings := map[string]any{
		"enabled": true,
		"parameters": map[string]any{
			"layout":       stringValue(cfg.Layout),
			"sshPublicKey": stringValue(cfg.SSHPublicKey),
		},
	}
	nodesParameters := nodesSettings["parameters"].(map[string]any)
	if cfg.Region != nil {
		nodesParameters["region"] = *cfg.Region
	}
	if cfg.Zones != nil {
		nodesParameters["zones"] = stringsToAnySlice(*cfg.Zones)
	}

	resources := make([]any, 0, 4+len(cfg.NodeGroups))

	moduleConfig := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleConfig",
		"metadata": map[string]any{
			"name": dvpModuleConfigName,
		},
		"spec": map[string]any{
			"enabled": true,
			"version": int64(1),
			"settings": map[string]any{
				"provider": providerSettings,
				"storage": map[string]any{
					"enabled":    true,
					"parameters": map[string]any{},
				},
				"nodes": nodesSettings,
			},
		},
	}
	resources = append(resources, moduleConfig)

	credentialSecret := map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      dvpCredentialSecretName,
			"namespace": "d8-cloud-provider-dvp",
			"labels": map[string]any{
				"heritage": "deckhouse",
				"module":   "cloud-provider-dvp",
			},
		},
		"type": "cloud-provider.deckhouse.io/credentials",
		"data": map[string]any{
			"authScheme": "S3ViZWNvbmZpZw==",
			"secret":     *cfg.Provider.KubeconfigDataBase64,
		},
	}
	resources = append(resources, credentialSecret)

	masterNodeGroup, err := mapFromAny(cfg.MasterNodeGroup)
	if err != nil {
		return fmt.Errorf("convert masterNodeGroup: %w", err)
	}
	if len(masterNodeGroup) != 0 {
		masterResources, err := createNodeGroupResources("master", masterNodeGroup, true, cfg.Zones)
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

		nodeGroupResources, err := createNodeGroupResources(name, nodeGroup, false, cfg.Zones)
		if err != nil {
			return err
		}
		resources = append(resources, nodeGroupResources...)
	}

	return createMigrationResourcesSecret(input, resources)
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
			Namespace: "d8-cloud-provider-dvp",
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "cloud-provider-dvp",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			dvpMigrationResourcesFilename: manifest,
		},
	}
	input.PatchCollector.CreateOrUpdate(secret)

	return nil
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

func createNodeGroupResources(name string, nodeGroup map[string]any, master bool, clusterZones *[]string) ([]any, error) {
	instanceClassSpec, ok := nodeGroup["instanceClass"].(map[string]any)
	if !ok || len(instanceClassSpec) == 0 {
		return nil, fmt.Errorf("%s.instanceClass cannot be empty", name)
	}

	instanceClassName := fmt.Sprintf("%s-%s", name, dvpDefaultInstanceSuffix)
	instanceClass := map[string]any{
		"apiVersion": dvpInstanceClassAPI,
		"kind":       dvpInstanceClassKind,
		"metadata": map[string]any{
			"name": instanceClassName,
			"labels": map[string]any{
				"heritage": "deckhouse",
				"module":   "cloud-provider-dvp",
			},
		},
		"spec": instanceClassSpec,
	}

	replicas, err := replicasFromNodeGroup(nodeGroup)
	if err != nil {
		return nil, fmt.Errorf("%s.replicas: %w", name, err)
	}

	zones := zonesFromNodeGroup(nodeGroup, clusterZones)
	cloudInstances := map[string]any{
		"zones":      zones,
		"minPerZone": replicasForUnstructured(replicas),
		"maxPerZone": replicasForUnstructured(replicas),
		"classReference": map[string]any{
			"kind": dvpInstanceClassKind,
			"name": instanceClassName,
		},
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
			"labels": map[string]any{
				"heritage": "deckhouse",
				"module":   "cloud-provider-dvp",
			},
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

	switch v := replicas.(type) {
	case float64:
		return int(v), nil
	case int64:
		return int(v), nil
	case int:
		return v, nil
	case int32:
		return int(v), nil
	default:
		return 0, fmt.Errorf("unexpected type %T", replicas)
	}
}

func replicasForUnstructured(replicas int) int64 {
	return int64(replicas)
}

func zonesFromNodeGroup(nodeGroup map[string]any, clusterZones *[]string) []any {
	if rawZones, ok := nodeGroup["zones"].([]interface{}); ok && len(rawZones) > 0 {
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

	return []any{"default"}
}

func stringsToAnySlice(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func convertJSONRawMessageToStruct(in map[string]json.RawMessage, out interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func overrideValues(p *v1.DvpProviderClusterConfiguration, m *v1.DvpModuleConfiguration) error {
	if m.Provider != nil {
		if p.Provider == nil {
			p.Provider = &v1.DvpProvider{}
		}
		if m.Provider.KubeconfigDataBase64 != nil {
			p.Provider.KubeconfigDataBase64 = m.Provider.KubeconfigDataBase64
		}
		if m.Provider.Namespace != nil {
			p.Provider.Namespace = m.Provider.Namespace
		}
	}

	if m.Zones != nil {
		p.Zones = m.Zones
	}

	if p.Provider == nil {
		return errors.New("provider section is required")
	}
	if p.Provider.KubeconfigDataBase64 == nil || len(*p.Provider.KubeconfigDataBase64) == 0 {
		return errors.New("provider.kubeconfigDataBase64 cannot be empty")
	}
	if p.Provider.Namespace == nil || len(*p.Provider.Namespace) == 0 {
		return errors.New("provider.namespace cannot be empty")
	}

	cloudManaged := p.APIVersion != nil || p.Kind != nil
	if cloudManaged {
		if p.APIVersion == nil || len(*p.APIVersion) == 0 {
			return errors.New("apiVersion cannot be empty")
		}
		if p.Kind == nil || len(*p.Kind) == 0 {
			return errors.New("kind cannot be empty")
		}
		if p.Zones == nil || len(*p.Zones) == 0 {
			def := []string{"default"}
			p.Zones = &def
		}
	}

	return nil
}

func mergeDiscoveryData(newValue cloudDataV1.DVPCloudProviderDiscoveryData, currentValue cloudDataV1.DVPCloudProviderDiscoveryData) cloudDataV1.DVPCloudProviderDiscoveryData {
	result := currentValue
	if newValue.APIVersion != "" && currentValue.APIVersion == "" {
		result.APIVersion = newValue.APIVersion
	}
	if newValue.Kind != "" && currentValue.Kind == "" {
		result.Kind = newValue.Kind
	}
	if newValue.Layout != "" && currentValue.Layout == "" {
		result.Layout = newValue.Layout
	}
	if len(newValue.Zones) > 0 && len(currentValue.Zones) == 0 {
		result.Zones = newValue.Zones
	}
	if len(newValue.StorageClassList) > 0 && len(currentValue.StorageClassList) == 0 {
		result.StorageClassList = newValue.StorageClassList
	}
	return result
}
