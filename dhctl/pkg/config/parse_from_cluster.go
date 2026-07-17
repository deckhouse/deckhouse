// Copyright 2025 Flant JSC
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

package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	clusterConfigSecretName = "d8-cluster-configuration"

	// LegacyProviderClusterConfigSecret holds the pre-mc-flow provider config.
	LegacyProviderClusterConfigSecret = "d8-provider-cluster-configuration"
)

// clusterConfigFromCluster is the d8-cluster-configuration Secret split into
// what callers need.
type clusterConfigFromCluster struct {
	// Raw is the document itself, kept for schema validation.
	Raw    []byte
	Parsed map[string]json.RawMessage
	Type   string
	// Provider is the lowercased cloud provider name, "" for a non-cloud cluster.
	Provider string
}

func readClusterConfigFromCluster(ctx context.Context, kubeCl *client.KubernetesClient) (clusterConfigFromCluster, error) {
	secret, err := kubeCl.CoreV1().Secrets(global.ConfigsNS).Get(ctx, clusterConfigSecretName, metav1.GetOptions{})
	if err != nil {
		return clusterConfigFromCluster{}, err
	}

	cfg := clusterConfigFromCluster{Raw: secret.Data["cluster-configuration.yaml"]}
	if err := yaml.Unmarshal(cfg.Raw, &cfg.Parsed); err != nil {
		return clusterConfigFromCluster{}, fmt.Errorf("unmarshal cluster configuration: %w", err)
	}
	if err := json.Unmarshal(cfg.Parsed["clusterType"], &cfg.Type); err != nil {
		return clusterConfigFromCluster{}, fmt.Errorf("parse cluster type: %w", err)
	}
	if cfg.Type != CloudClusterType {
		return cfg, nil
	}

	var cloud ClusterConfigCloudSpec
	if err := json.Unmarshal(cfg.Parsed["cloud"], &cloud); err != nil {
		return clusterConfigFromCluster{}, fmt.Errorf("parse cloud provider from cluster config: %w", err)
	}
	cfg.Provider = strings.ToLower(cloud.Provider)
	return cfg, nil
}

// ClusterUsesProviderModuleConfig reports whether the running cluster is
// configured through the cloud-provider-<name> ModuleConfig (mc-flow) rather
// than the legacy d8-provider-cluster-configuration Secret. A non-cloud cluster
// reports false.
func ClusterUsesProviderModuleConfig(ctx context.Context, kubeCl *client.KubernetesClient) (bool, error) {
	cfg, err := readClusterConfigFromCluster(ctx, kubeCl)
	if err != nil || cfg.Provider == "" {
		return false, err
	}

	// Ask the API directly instead of going through loadCloudProviderModuleConfig:
	// only the ModuleConfig's presence matters here, while parsing it would need a
	// SchemaStore — and building one is what freezes the process-wide store, so a
	// store built for this check would then be handed to the edit itself.
	name := CloudProviderModuleName(cfg.Provider)
	if _, err := kubeCl.Dynamic().Resource(ModuleConfigGVR).Get(ctx, name, metav1.GetOptions{}); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get ModuleConfig %q: %w", name, err)
	}
	return true, nil
}

// nilType instantiates ByClusterType[T] for fillers that produce no value,
// keeping `return nil, err` valid in every branch.
type nilType *struct{}

type fromClusterMetaConfigFiller struct {
	kubeCl      *client.KubernetesClient
	schemaStore *SchemaStore
}

func newFromClusterMetaConfigFiller(kubeCl *client.KubernetesClient, schemaStore *SchemaStore) *fromClusterMetaConfigFiller {
	return &fromClusterMetaConfigFiller{
		kubeCl:      kubeCl,
		schemaStore: schemaStore,
	}
}

// Cloud loads cloud-provider configuration from the running cluster. Two
// markers exist — the cloud-provider-<name> ModuleConfig (mc-flow) and the
// legacy d8-provider-cluster-configuration Secret — and both are loaded when
// present: a cluster mid-migration carries both, with PCC staying the source
// of truth for the typed fields. Neither present is an error.
func (f *fromClusterMetaConfigFiller) Cloud(ctx context.Context, metaConfig *MetaConfig) (nilType, error) {
	if err := metaConfig.prepareProviderName(); err != nil {
		return nil, err
	}

	mc, err := loadCloudProviderModuleConfig(ctx, f.kubeCl, metaConfig.ProviderName, f.schemaStore)
	if err != nil {
		return nil, err
	}
	if mc != nil {
		metaConfig.ModuleConfigs = append(metaConfig.ModuleConfigs, mc)
	}

	pcc, err := loadLegacyProviderClusterConfig(ctx, f.kubeCl, f.schemaStore)
	if err != nil {
		return nil, err
	}
	if pcc != nil {
		metaConfig.ProviderClusterConfig = pcc
	}

	if mc == nil && pcc == nil {
		return nil, fmt.Errorf(
			"cluster has neither ModuleConfig %q nor Secret %q in namespace %q",
			CloudProviderModuleName(metaConfig.ProviderName),
			LegacyProviderClusterConfigSecret,
			global.ConfigsNS,
		)
	}

	return nil, nil
}

func loadCloudProviderModuleConfig(ctx context.Context, kubeCl *client.KubernetesClient, providerName string, schemaStore *SchemaStore) (*ModuleConfig, error) {
	name := CloudProviderModuleName(providerName)
	obj, err := kubeCl.Dynamic().Resource(ModuleConfigGVR).Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ModuleConfig %q: %w", name, err)
	}
	return moduleConfigFromUnstructured(obj, schemaStore)
}

// moduleConfigFromUnstructured deserialises a ModuleConfig fetched from the
// cluster and validates it against its registered schema, so a kubectl-patched
// invalid ModuleConfig fails fast here instead of as a confusing downstream
// validation error. A module without a registered schema is accepted.
func moduleConfigFromUnstructured(obj *unstructured.Unstructured, schemaStore *SchemaStore) (*ModuleConfig, error) {
	raw, err := json.Marshal(obj.Object)
	if err != nil {
		return nil, fmt.Errorf("marshal ModuleConfig: %w", err)
	}

	yamlDoc, err := yaml.JSONToYAML(raw)
	if err != nil {
		return nil, fmt.Errorf("convert ModuleConfig to YAML: %w", err)
	}
	if _, err := schemaStore.Validate(&yamlDoc); err != nil && !errors.Is(err, ErrSchemaNotFound) {
		return nil, fmt.Errorf("validate ModuleConfig %q: %w", obj.GetName(), err)
	}

	mc := &ModuleConfig{}
	if err := json.Unmarshal(raw, mc); err != nil {
		return nil, fmt.Errorf("unmarshal ModuleConfig: %w", err)
	}
	return mc, nil
}

func loadLegacyProviderClusterConfig(ctx context.Context, kubeCl *client.KubernetesClient, schemaStore *SchemaStore) (map[string]json.RawMessage, error) {
	secret, err := kubeCl.CoreV1().Secrets(global.ConfigsNS).Get(ctx, LegacyProviderClusterConfigSecret, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Secret %q: %w", LegacyProviderClusterConfigSecret, err)
	}
	return parseLegacyProviderClusterConfig(secret, schemaStore)
}

func parseLegacyProviderClusterConfig(secret *corev1.Secret, schemaStore *SchemaStore) (map[string]json.RawMessage, error) {
	data, ok := secret.Data["cloud-provider-cluster-configuration.yaml"]
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("cloud-provider-cluster-configuration.yaml not found in Secret or empty")
	}
	if _, err := schemaStore.Validate(&data); err != nil {
		return nil, fmt.Errorf("validate provider cluster configuration: %w", err)
	}
	var parsed map[string]json.RawMessage
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal provider cluster configuration: %w", err)
	}
	return parsed, nil
}

func (f *fromClusterMetaConfigFiller) Static(ctx context.Context, metaConfig *MetaConfig) (nilType, error) {
	// The configuration may be absent entirely: auto-discovery covers it.
	staticClusterConfig, err := f.kubeCl.CoreV1().Secrets(global.ConfigsNS).Get(ctx, "d8-static-cluster-configuration", metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	staticClusterConfigData, ok := staticClusterConfig.Data["static-cluster-configuration.yaml"]
	if !ok || len(staticClusterConfigData) == 0 {
		return nil, nil
	}

	if _, err := f.schemaStore.Validate(&staticClusterConfigData); err != nil {
		return nil, fmt.Errorf("validate static cluster configuration: %w", err)
	}

	var parsedStaticClusterConfig map[string]json.RawMessage
	if err := yaml.Unmarshal(staticClusterConfigData, &parsedStaticClusterConfig); err != nil {
		return nil, fmt.Errorf("unmarshal static cluster configuration: %w", err)
	}

	metaConfig.StaticClusterConfig = parsedStaticClusterConfig

	return nil, nil
}

func (f *fromClusterMetaConfigFiller) Incorrect(_ context.Context, metaConfig *MetaConfig) (nilType, error) {
	return nil, UnsupportedClusterTypeErr(metaConfig)
}
