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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	cloudDataV1 "github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1"
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	v1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/hooks/internal/v1"
)

type pccSecretFilterResult struct {
	ProviderClusterConfig     map[string]json.RawMessage `json:"providerClusterConfig,omitempty"`
	ProviderDiscoveryDataJSON json.RawMessage            `json:"providerDiscoveryDataJSON,omitempty"`
}

type candiDiscoveryDataFilterResult struct {
	DiscoveryDataJSON json.RawMessage `json:"discoveryDataJSON,omitempty"`
}

type moduleConfigFilterResult struct {
	Version  int64           `json:"version"`
	Enabled  bool            `json:"enabled"`
	Provider json.RawMessage `json:"provider,omitempty"`
}

type namedResourceResult struct {
	Name string `json:"name"`
}

type credentialSecretResult struct {
	Name string `json:"name"`
}

func filterPCCSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// The fake k8s dynamic client ignores field selectors, so we guard by name here.
	if obj.GetName() != pccSecretName {
		return nil, nil
	}
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

func filterCandiDiscoverySecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// The fake k8s dynamic client ignores field selectors, so we guard by name here.
	if obj.GetName() != dvpCandiDiscoverySecretName {
		return nil, nil
	}
	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("cannot convert candi discovery secret from unstructured: %v", err)
	}

	discoveryDataJSON, ok := secret.Data["cloud-provider-discovery-data.json"]
	if !ok || len(discoveryDataJSON) == 0 {
		return candiDiscoveryDataFilterResult{}, nil
	}

	if _, err := config.ValidateDiscoveryData(&discoveryDataJSON, nil, nil); err != nil {
		return nil, fmt.Errorf("validate candi cloud-provider-discovery-data.json: %v", err)
	}

	return candiDiscoveryDataFilterResult{
		DiscoveryDataJSON: json.RawMessage(discoveryDataJSON),
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
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
		{
			Name:       "module_config",
			ApiVersion: moduleConfigAPIVersion,
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpModuleName},
			},
			FilterFunc: filterModuleConfig,
		},
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
			FilterFunc: filterCredentialSecret,
		},
		{
			Name:       "node_groups",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: filterNamedResource,
		},
		{
			Name:       "dvp_instance_classes",
			ApiVersion: moduleConfigAPIVersion,
			Kind:       dvpInstanceClassKind,
			FilterFunc: filterNamedResource,
		},
		{
			Name:       "candi_discovery_data",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{dvpNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpCandiDiscoverySecretName},
			},
			FilterFunc: filterCandiDiscoverySecret,
		},
	},
}, handleDVPClusterConfiguration)

func handleDVPClusterConfiguration(_ context.Context, input *go_hook.HookInput) error {
	// candi takes priority over PCC
	discoveryData, candiPresent := resolveDiscoveryData(input)

	pccSnaps := input.Snapshots.Get("provider_cluster_configuration")
	pccPresent := len(pccSnaps) > 0

	if !pccPresent {
		// no PCC: new cluster, standard flow
		deleteMigrationArtifacts(input)
		return mergeAndSetDiscoveryData(input, discoveryData)
	}

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

	// fall back to PCC discovery when no candi secret
	if !candiPresent && len(pccResult.ProviderDiscoveryDataJSON) > 0 {
		if err := json.Unmarshal(pccResult.ProviderDiscoveryDataJSON, &discoveryData); err != nil {
			return fmt.Errorf("unmarshal discovery data from PCC: %w", err)
		}
	}

	newResourcesComplete := isNewResourcesComplete(input, &pcc)

	if newResourcesComplete {
		// migration done: use MC v2 values
		deleteMigrationArtifacts(input)
		return mergeAndSetDiscoveryData(input, discoveryData)
	}

	// migration in progress: populate values from PCC so templates render
	if err := mapPCCtoRootValues(input, &pcc); err != nil {
		return fmt.Errorf("map PCC to root values: %w", err)
	}

	// validates PCC credentials before namespace exists; actual resources created in create_migration_resources.go
	var moduleConfiguration v1.DvpModuleConfiguration
	if err := json.Unmarshal([]byte(input.Values.Get("cloudProviderDvp").String()), &moduleConfiguration); err != nil {
		return fmt.Errorf("parse module configuration: %w", err)
	}

	overrideProviderClusterConfigValues(&pcc, &moduleConfiguration)

	// setDefaultZones preserves the "default" zone fallback for the live
	// cluster-configuration hook. node-manager's get_crds.go
	// (modules/040-node-manager/hooks/get_crds.go) substitutes defaultZones for
	// NodeGroups whose zones are nil; keeping a non-empty zone here preserves the
	// historical behavior for clusters that relied on the synthetic "default"
	// zone. The migration path (create_migration_resources.go) intentionally does
	// NOT call this, so migrated NodeGroups faithfully reflect the source PCC.
	setDefaultZones(&pcc)

	if err := validateProviderClusterConfig(pcc); err != nil {
		return fmt.Errorf("validate provider cluster config: %w", err)
	}

	return mergeAndSetDiscoveryData(input, discoveryData)
}

// candiPresent=true on unmarshal error suppresses PCC fallback to avoid stale data
func resolveDiscoveryData(input *go_hook.HookInput) (cloudDataV1.DVPCloudProviderDiscoveryData, bool) {
	candiSnaps := input.Snapshots.Get("candi_discovery_data")
	if len(candiSnaps) == 0 {
		return cloudDataV1.DVPCloudProviderDiscoveryData{}, false
	}

	var candiResult candiDiscoveryDataFilterResult
	if err := candiSnaps[0].UnmarshalTo(&candiResult); err != nil {
		input.Logger.Warn("failed to unmarshal candi discovery snapshot; PCC fallback suppressed", "error", err)
		return cloudDataV1.DVPCloudProviderDiscoveryData{}, true
	}

	if len(candiResult.DiscoveryDataJSON) == 0 {
		return cloudDataV1.DVPCloudProviderDiscoveryData{}, true
	}

	var discoveryData cloudDataV1.DVPCloudProviderDiscoveryData
	if err := json.Unmarshal(candiResult.DiscoveryDataJSON, &discoveryData); err != nil {
		input.Logger.Warn("failed to parse candi discovery data JSON; PCC fallback suppressed", "error", err)
		return cloudDataV1.DVPCloudProviderDiscoveryData{}, true
	}

	return discoveryData, true
}

func isNewResourcesComplete(input *go_hook.HookInput, pcc *v1.DvpProviderClusterConfiguration) bool {
	mcSnaps := input.Snapshots.Get("module_config")
	if len(mcSnaps) == 0 {
		return false
	}
	var mc moduleConfigFilterResult
	if err := mcSnaps[0].UnmarshalTo(&mc); err != nil {
		return false
	}
	if mc.Version < 2 || !mc.Enabled || len(mc.Provider) == 0 {
		return false
	}

	credSnaps := input.Snapshots.Get("credential_secret_d8")
	if len(credSnaps) == 0 {
		return false
	}
	var cred credentialSecretResult
	if err := credSnaps[0].UnmarshalTo(&cred); err != nil || cred.Name == "" {
		return false
	}

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

	// hybrid clusters have no masterNodeGroup
	if pcc != nil && pcc.MasterNodeGroup != nil {
		if !nodeGroupSet["master"] || !icSet[cpapi.BuildInstanceClassName("master")] {
			return false
		}
	}

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
			if !nodeGroupSet[name] || !icSet[cpapi.BuildInstanceClassName(name)] {
				return false
			}
		}
	}

	return true
}

// leaf-only writes preserve addon-operator defaults (nodes.disabled, storage.disabled)
func mapPCCtoRootValues(input *go_hook.HookInput, pcc *v1.DvpProviderClusterConfiguration) error {
	if pcc == nil {
		return nil
	}

	// provider has no disabled flag, overwriting is safe
	if pcc.Provider != nil && pcc.Provider.Namespace != nil {
		input.Values.Set("cloudProviderDvp.provider", map[string]any{
			"parameters": map[string]any{
				"namespace": *pcc.Provider.Namespace,
			},
		})
	}

	// nodes.disabled intentionally not touched
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
		// add op cannot create intermediate path; read-merge preserves nodes.disabled
		nodes := make(map[string]any)
		if v, ok := input.Values.GetOk("cloudProviderDvp.nodes"); ok {
			if err := json.Unmarshal([]byte(v.Raw), &nodes); err != nil {
				return fmt.Errorf("unmarshal nodes: %w", err)
			}
		}
		nodes["parameters"] = nodesParams
		input.Values.Set("cloudProviderDvp.nodes", nodes)
	}

	// inject synthetic creds only if credentials.go (Order 19) hasn't populated yet
	if _, exists := input.Values.GetOk("cloudProviderDvp.internal.credentialSecrets.d8-credentials"); !exists {
		if pcc.Provider != nil && pcc.Provider.KubeconfigDataBase64 != nil && len(*pcc.Provider.KubeconfigDataBase64) > 0 {
			// set whole map at once; JSON-patch fails on missing intermediate path
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

func convertJSONRawMessageToStruct(in map[string]json.RawMessage, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func overrideProviderClusterConfigValues(p *v1.DvpProviderClusterConfiguration, m *v1.DvpModuleConfiguration) {
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
}

func validateProviderClusterConfig(p v1.DvpProviderClusterConfiguration) error {
	if p.Provider == nil {
		return errors.New("provider section is required")
	}
	if p.Provider.KubeconfigDataBase64 == nil || len(*p.Provider.KubeconfigDataBase64) == 0 {
		return errors.New("provider.kubeconfigDataBase64 cannot be empty")
	}
	if p.Provider.Namespace == nil || len(*p.Provider.Namespace) == 0 {
		return errors.New("provider.namespace cannot be empty")
	}

	hasObjectMeta := p.APIVersion != nil || p.Kind != nil
	if hasObjectMeta {
		if p.APIVersion == nil || len(*p.APIVersion) == 0 {
			return errors.New("apiVersion cannot be empty")
		}
		if p.Kind == nil || len(*p.Kind) == 0 {
			return errors.New("kind cannot be empty")
		}
	}

	return nil
}

func setDefaultZones(p *v1.DvpProviderClusterConfiguration) {
	hasObjectMeta := p.APIVersion != nil || p.Kind != nil
	if hasObjectMeta {
		if p.Zones == nil || len(*p.Zones) == 0 {
			def := []string{"default"}
			p.Zones = &def
		}
	}
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
