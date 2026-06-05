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
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	v1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/hooks/internal/v1"
)

const (
	dvpMigrationConfigMapName = "d8-module-is-migrating"
)

// pccSecretFilterResult holds the parsed data from the PCC secret.
type pccSecretFilterResult struct {
	// ProviderClusterConfig is the raw JSON map from the cluster configuration.
	ProviderClusterConfig map[string]json.RawMessage `json:"providerClusterConfig,omitempty"`
	// ProviderDiscoveryData is the raw JSON of the cloud provider discovery data.
	ProviderDiscoveryDataJSON json.RawMessage `json:"providerDiscoveryDataJSON,omitempty"`
}

// moduleConfigFilterResult holds the relevant fields from a ModuleConfig object.
type moduleConfigFilterResult struct {
	Version  int64           `json:"version"`
	Enabled  bool            `json:"enabled"`
	Provider json.RawMessage `json:"provider,omitempty"`
}

// namedResourceResult holds just the name of a Kubernetes resource.
type namedResourceResult struct {
	Name string `json:"name"`
}

// credentialSecretResult holds the name of a credential secret.
type credentialSecretResult struct {
	Name string `json:"name"`
}

// ---- filter functions ----

func filterPCCSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("cannot convert PCC secret from unstructured: %v", err)
	}

	result := &pccSecretFilterResult{}

	if discoveryDataJSON, ok := secret.Data["cloud-provider-discovery-data.json"]; ok && len(discoveryDataJSON) > 0 {
		if _, err := config.ValidateDiscoveryData(&discoveryDataJSON, nil, nil); err != nil {
			return nil, fmt.Errorf("validate cloud-provider-discovery-data.json: %v", err)
		}
		result.ProviderDiscoveryDataJSON = json.RawMessage(discoveryDataJSON)
	}

	result := &pccSecretFilterResult{}

	if discoveryDataJSON, ok := secret.Data["cloud-provider-discovery-data.json"]; ok && len(discoveryDataJSON) > 0 {
		if _, err := config.ValidateDiscoveryData(&discoveryDataJSON, additionalOpenAPISchemasPaths); err != nil {
			return nil, fmt.Errorf("validate cloud-provider-discovery-data.json: %v", err)
		}
		result.ProviderDiscoveryDataJSON = json.RawMessage(discoveryDataJSON)
	}

	if clusterConfigYAML, ok := secret.Data["cloud-provider-cluster-configuration.yaml"]; ok && len(clusterConfigYAML) > 0 {
		m, err := config.ParseConfigFromData(
			context.Background(),
			string(clusterConfigYAML),
			infrastructureprovider.MetaConfigPreparatorProvider(infrastructureprovider.NewPreparatorProviderParamsWithoutLogger()),
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf("validate cloud-provider-cluster-configuration.yaml: %v", err)
		}
		result.ProviderClusterConfig = m.ProviderClusterConfig
	}

	return result, nil
}

func filterModuleConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &deckhousev1alpha1.ModuleConfig{}
	if err := sdk.FromUnstructured(obj, mc); err != nil {
		return nil, fmt.Errorf("convert ModuleConfig from unstructured: %w", err)
	}

	result := moduleConfigFilterResult{
		Version: int64(mc.Spec.Version),
		Enabled: mc.Spec.Enabled != nil && *mc.Spec.Enabled,
	}

	if mc.Spec.Settings != nil {
		if providerRaw, ok := mc.Spec.Settings.GetMap()["provider"]; ok {
			providerBytes, err := json.Marshal(providerRaw)
			if err == nil {
				result.Provider = json.RawMessage(providerBytes)
			}
		}
	}

	return result, nil
}

func filterNamedResource(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return namedResourceResult{Name: obj.GetName()}, nil
}

func filterCredentialSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, err
	}
	if secret.Type != dvpCredentialSecretType {
		return nil, nil
	}
	return credentialSecretResult{Name: secret.Name}, nil
}

// ---- hook registration ----

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		// Binding 0: PCC secret in kube-system (triggers the hook on change)
		{
			Name:       "provider_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-provider-cluster-configuration"},
			},
			FilterFunc: filterPCCSecret,
		},
		// Binding 1: ModuleConfig for the DVP module (does not trigger hook on change — read-only snapshot)
		{
			Name:       "module_config",
			ApiVersion: dvpModuleConfigAPIVersion,
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpModuleConfigName},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterModuleConfig,
		},
		// Binding 2: d8-credentials Secret (does not trigger hook on change — read-only snapshot)
		{
			Name:       "credential_secret_d8",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{dvpNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpCredentialSecretName},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterCredentialSecret,
		},
		// Binding 3: NodeGroup CRs (does not trigger hook on change — read-only snapshot)
		{
			Name:                "node_groups",
			ApiVersion:          "deckhouse.io/v1",
			Kind:                "NodeGroup",
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterNamedResource,
		},
		// Binding 4: DVPInstanceClass CRs (does not trigger hook on change — read-only snapshot)
		{
			Name:                "dvp_instance_classes",
			ApiVersion:          dvpModuleConfigAPIVersion,
			Kind:                dvpInstanceClassKind,
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterNamedResource,
		},
		// Binding 5: d8-migration-resources Secret in dvp namespace (does not trigger hook on change — read-only snapshot)
		{
			Name:       "migration_resources_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{dvpNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpMigrationResourcesName},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterNamedResource,
		},
		// Binding 6: d8-module-is-migrating ConfigMap in dvp namespace (does not trigger hook on change — read-only snapshot)
		{
			Name:       "migration_configmap",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{dvpNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpMigrationConfigMapName},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterNamedResource,
		},
	},
}, handleDVPClusterConfiguration)

func handleDVPClusterConfiguration(_ context.Context, input *go_hook.HookInput) error {
	// ---- Determine PCC presence ----
	pccSnaps := input.Snapshots.Get("provider_cluster_configuration")
	pccPresent := len(pccSnaps) > 0

	// ---- State machine ----
	if !pccPresent {
		// State A: no PCC — new cluster on v2, standard flow.
		// Values come from ModuleConfig v2 via addon-operator (already in input.Values).
		// Clean up migration artifacts if they exist.
		deleteMigrationArtifacts(input)
		return mergeAndSetDiscoveryData(input, cloudDataV1.DVPCloudProviderDiscoveryData{})
	}

	// PCC is present — parse it.
	var pccResult pccSecretFilterResult
	if err := pccSnaps[0].UnmarshalTo(&pccResult); err != nil {
		return fmt.Errorf("unmarshal PCC snapshot: %w", err)
	}

	var pcc v1.DvpProviderClusterConfiguration
	if len(pccResult.ProviderClusterConfig) > 0 {
		if err := convertJSONRawMessageToStruct(pccResult.ProviderClusterConfig, &pcc); err != nil {
			return fmt.Errorf("parse PCC: %w", err)
		}
	}

	// Parse discovery data.
	var discoveryData cloudDataV1.DVPCloudProviderDiscoveryData
	if len(pccResult.ProviderDiscoveryDataJSON) > 0 {
		if err := json.Unmarshal(pccResult.ProviderDiscoveryDataJSON, &discoveryData); err != nil {
			return fmt.Errorf("unmarshal discovery data: %w", err)
		}
	}

	// ---- Determine completeness of new resources ----
	newResourcesComplete := isNewResourcesComplete(input, &pcc)

	if newResourcesComplete {
		// State C: PCC present but migration is done.
		// Values come from MC v2 (root path) — do NOT override from PCC.
		// Clean up migration artifacts.
		deleteMigrationArtifacts(input)
		return mergeAndSetDiscoveryData(input, discoveryData)
	}

	// State B: PCC present, new resources incomplete — migration in progress.
	// Write root values from PCC so templates can render.
	if err := mapPCCtoRootValues(input, &pcc); err != nil {
		return fmt.Errorf("map PCC to root values: %w", err)
	}

	// Validate and enrich the PCC (e.g. merge any MC-level overrides).
	var moduleConfiguration v1.DvpModuleConfiguration
	if err := json.Unmarshal([]byte(input.Values.Get("cloudProviderDvp").String()), &moduleConfiguration); err != nil {
		return fmt.Errorf("parse module configuration: %w", err)
	}
	if err := overrideValues(&pcc, &moduleConfiguration); err != nil {
		return fmt.Errorf("override values: %w", err)
	}

	// Create d8-migration-resources Secret.
	if err := createProviderClusterConfigurationResources(input, &pcc); err != nil {
		return fmt.Errorf("create migration resources: %w", err)
	}

	// Create d8-module-is-migrating ConfigMap.
	createMigrationConfigMap(input)

	return mergeAndSetDiscoveryData(input, discoveryData)
}

// isNewResourcesComplete checks whether the migration target resources are fully in place.
func isNewResourcesComplete(input *go_hook.HookInput, pcc *v1.DvpProviderClusterConfiguration) bool {
	// Check ModuleConfig.
	mcSnaps := input.Snapshots.Get("module_config")
	if len(mcSnaps) == 0 {
		return false
	}
	var mc moduleConfigFilterResult
	if err := mcSnaps[0].UnmarshalTo(&mc); err != nil {
		return false
	}
	if mc.Version != 2 || !mc.Enabled || len(mc.Provider) == 0 {
		return false
	}

	// Check d8-credentials Secret.
	credSnaps := input.Snapshots.Get("credential_secret_d8")
	if len(credSnaps) == 0 {
		return false
	}
	var cred credentialSecretResult
	if err := credSnaps[0].UnmarshalTo(&cred); err != nil || cred.Name == "" {
		return false
	}

	// Build sets of existing NodeGroups and DVPInstanceClasses.
	existingNodeGroups, err := sdkobjectpatch.UnmarshalToStruct[namedResourceResult](input.Snapshots, "node_groups")
	if err != nil {
		return false
	}
	nodeGroupSet := make(map[string]bool, len(existingNodeGroups))
	for _, ng := range existingNodeGroups {
		nodeGroupSet[ng.Name] = true
	}

	existingICs, err := sdkobjectpatch.UnmarshalToStruct[namedResourceResult](input.Snapshots, "dvp_instance_classes")
	if err != nil {
		return false
	}
	icSet := make(map[string]bool, len(existingICs))
	for _, ic := range existingICs {
		icSet[ic.Name] = true
	}

	// Check master NodeGroup + InstanceClass only when the PCC defines a masterNodeGroup.
	// Hybrid clusters (static control plane, CSI-only) have no masterNodeGroup in PCC.
	if pcc != nil && pcc.MasterNodeGroup != nil {
		if !nodeGroupSet["master"] || !icSet["master-dvp"] {
			return false
		}
	}

	// Check each additional nodeGroup from PCC.
	if pcc != nil {
		for _, rawNG := range pcc.NodeGroups {
			ng, err := mapFromAny(rawNG)
			if err != nil {
				return false
			}
			name, ok := ng["name"].(string)
			if !ok || name == "" {
				return false
			}
			if !nodeGroupSet[name] || !icSet[fmt.Sprintf("%s-dvp", name)] {
				return false
			}
		}
	}

	return true
}

// mapPCCtoRootValues writes PCC fields into the root module values path
// (cloudProviderDvp.provider/nodes) in v2 format so templates can render during migration.
// Only individual leaf values are written so that fields populated by addon-operator from
// config-values defaults (e.g. nodes.enabled, storage.enabled) are never overwritten.
// storage is left entirely untouched — PCC does not control subsystem enablement.
//
// It also injects a synthetic cloudProviderDvp.internal.credentialSecrets["d8-credentials"]
// entry built from PCC.provider.kubeconfigDataBase64, but ONLY when the key is absent.
// The credentials.go hook (Order 19) populates credentialSecrets from real Secrets; if it
// already placed d8-credentials, we must not overwrite it.
func mapPCCtoRootValues(input *go_hook.HookInput, pcc *v1.DvpProviderClusterConfiguration) error {
	if pcc == nil {
		return nil
	}

	// provider — set the whole object (provider has no enabled flag, so overwriting is safe).
	if pcc.Provider != nil && pcc.Provider.Namespace != nil {
		input.Values.Set("cloudProviderDvp.provider", map[string]any{
			"parameters": map[string]any{
				"namespace": *pcc.Provider.Namespace,
			},
		})
	}

	// nodes.parameters — only PCC-derived fields; nodes.enabled is intentionally not touched
	// so the default from config-values is preserved.
	nodesParams := map[string]any{}
	if pcc.Layout != nil {
		nodesParams["layout"] = *pcc.Layout
	}
	if pcc.SSHPublicKey != nil {
		nodesParams["sshPublicKey"] = *pcc.SSHPublicKey
	}
	if pcc.Region != nil {
		nodesParams["region"] = *pcc.Region
	}
	if pcc.Zones != nil && len(*pcc.Zones) > 0 {
		nodesParams["zones"] = *pcc.Zones
	}
	if len(nodesParams) > 0 {
		input.Values.Set("cloudProviderDvp.nodes.parameters", nodesParams)
	}

	// storage — not touched; PCC does not control subsystem enablement.

	// Inject synthetic d8-credentials from PCC kubeconfig only when the real Secret
	// is not yet present (i.e. credentials.go did not populate it at Order 19).
	// We must set the whole credentialSecrets map at once to avoid JSON-patch
	// "missing path" errors when the key does not exist yet.
	if _, exists := input.Values.GetOk("cloudProviderDvp.internal.credentialSecrets.d8-credentials"); !exists {
		if pcc.Provider != nil && pcc.Provider.KubeconfigDataBase64 != nil && len(*pcc.Provider.KubeconfigDataBase64) > 0 {
			// Read existing credentialSecrets map and add the synthetic entry.
			existing := make(map[string]any)
			if v, ok := input.Values.GetOk("cloudProviderDvp.internal.credentialSecrets"); ok {
				if err := json.Unmarshal([]byte(v.Raw), &existing); err != nil {
					return fmt.Errorf("unmarshal credentialSecrets: %w", err)
				}
			}
			existing[dvpCredentialSecretName] = map[string]any{
				"authScheme": dvpAuthSchemeKubeconfig,
				"secret":     *pcc.Provider.KubeconfigDataBase64,
			}
			input.Values.Set("cloudProviderDvp.internal.credentialSecrets", existing)
		}
	}

	return nil
}

// deleteMigrationArtifacts removes d8-migration-resources Secret and
// d8-module-is-migrating ConfigMap. Missing objects are ignored by the patch collector.
func deleteMigrationArtifacts(input *go_hook.HookInput) {
	input.PatchCollector.Delete("v1", "Secret", dvpNamespace, dvpMigrationResourcesName)
	input.PatchCollector.Delete("v1", "ConfigMap", dvpNamespace, dvpMigrationConfigMapName)
}

// createMigrationConfigMap creates (or updates) the d8-module-is-migrating ConfigMap.
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
				"module":   dvpModuleLabel,
			},
		},
	}
	input.PatchCollector.CreateOrUpdate(cm)
}

// mergeAndSetDiscoveryData merges the provided discovery data with any existing values
// and writes the result to internal.providerDiscoveryData.
func mergeAndSetDiscoveryData(input *go_hook.HookInput, discoveryData cloudDataV1.DVPCloudProviderDiscoveryData) error {
	providerDiscoveryDataValuesJSON, ok := input.Values.GetOk("cloudProviderDvp.internal.providerDiscoveryData")
	if ok && len(providerDiscoveryDataValuesJSON.String()) != 0 {
		var existing cloudDataV1.DVPCloudProviderDiscoveryData
		if err := json.Unmarshal([]byte(providerDiscoveryDataValuesJSON.String()), &existing); err != nil {
			return err
		}
		discoveryData = mergeDiscoveryData(discoveryData, existing)
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
}

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

func convertJSONRawMessageToStruct(in map[string]json.RawMessage, out any) error {
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
