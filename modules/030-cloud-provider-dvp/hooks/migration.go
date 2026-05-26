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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	v1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/hooks/internal/v1"
)

const (
	dvpModuleConfigName           = "cloud-provider-dvp"
	dvpNamespace                  = "d8-cloud-provider-dvp"
	dvpMigrationResourcesName     = "d8-migration-resources"
	dvpMigrationResourcesFilename = "resources.yaml"
	dvpCredentialSecretName       = "d8-cloud-provider-dvp-credentials"
	dvpInstanceClassKind          = "DVPInstanceClass"
	dvpInstanceClassAPI           = "deckhouse.io/v1alpha1"
	// dvpAuthSchemeKubeconfig is base64("kubeconfig") — the credential type
	// understood by the DVP cloud-provider credential secret.
	dvpAuthSchemeKubeconfig = "S3ViZWNvbmZpZw=="
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

	layout := ""
	if cfg.Layout != nil {
		layout = *cfg.Layout
	}
	sshPublicKey := ""
	if cfg.SSHPublicKey != nil {
		sshPublicKey = *cfg.SSHPublicKey
	}
	nodesSettings := map[string]any{
		"enabled": true,
		"parameters": map[string]any{
			"layout":       layout,
			"sshPublicKey": sshPublicKey,
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
			"version": int(2),
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
			"namespace": dvpNamespace,
		},
		"type": "cloud-provider.deckhouse.io/credentials",
		"data": map[string]any{
			"authScheme": dvpAuthSchemeKubeconfig,
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
			Namespace: dvpNamespace,
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

	instanceClassName := fmt.Sprintf("%s-dvp", name)
	instanceClass := map[string]any{
		"apiVersion": dvpInstanceClassAPI,
		"kind":       dvpInstanceClassKind,
		"metadata": map[string]any{
			"name":   instanceClassName,
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
		"minPerZone": int64(replicas),
		"maxPerZone": int64(replicas),
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
			"name":   name,
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
	// mapFromAny routes everything through json.Unmarshal into map[string]any,
	// which always yields float64 for JSON numbers.
	v, ok := replicas.(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected type %T", replicas)
	}
	return int(v), nil
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
