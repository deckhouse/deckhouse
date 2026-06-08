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

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdata"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const legacyProviderClusterConfigSecretName = "d8-provider-cluster-configuration"

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

// Cloud loads cloud-provider configuration from the running cluster. The
// cluster carries one of two mutually understood "markers":
//
//   - mc-flow:     ModuleConfig cloud-provider-<name> exists.
//   - legacy:      Secret d8-provider-cluster-configuration exists.
//
// Preference rules (mc-flow ALWAYS wins when present):
//
//  1. mc-flow present, legacy present (mid-migration): keep ModuleConfig,
//     **ignore** the legacy Secret. Treating PCC as authoritative during
//     migration would silently use the pre-migration values forever,
//     because extractProviderClusterFields prefers PCC over CloudProviderVars
//     for typed fields. Wiping ProviderClusterConfig here forces the
//     mc-flow path.
//  2. mc-flow present, legacy absent (post-migration): use ModuleConfig.
//  3. mc-flow absent, legacy present (pre-migration): use PCC.
//  4. Both absent: descriptive error — the cluster has no provider config.
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
		// mc-flow wins: do not touch the legacy Secret. Skipping the
		// load also avoids parseLegacyProviderClusterConfig's schema
		// validation, which is irrelevant once the ModuleConfig is the
		// authoritative source.
		return nil, nil
	}

	pcc, err := loadLegacyProviderClusterConfig(ctx, f.kubeCl, f.schemaStore)
	if err != nil {
		return nil, err
	}
	if pcc != nil {
		metaConfig.ProviderClusterConfig = pcc
		return nil, nil
	}

	return nil, fmt.Errorf(
		"cluster has neither ModuleConfig %q nor Secret %q in namespace %q",
		providerdata.CloudProviderModuleName(metaConfig.ProviderName),
		legacyProviderClusterConfigSecretName,
		global.ConfigsNS,
	)
}

func loadCloudProviderModuleConfig(ctx context.Context, kubeCl *client.KubernetesClient, providerName string, schemaStore *SchemaStore) (*ModuleConfig, error) {
	name := providerdata.CloudProviderModuleName(providerName)
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
// cluster and validates it against its registered schema. Validation matches
// the legacy PCC path (parseLegacyProviderClusterConfig also calls
// schemaStore.Validate) so a kubectl-patched invalid ModuleConfig fails fast
// here instead of surfacing as a confusing downstream error from the
// external preparator. When no schema is registered for the module (e.g.
// the module-config schemas weren't loaded into this SchemaStore) we accept
// the document — Validate-without-schema is what HasSchemaForModuleConfig
// guards us from in higher-level paths.
func moduleConfigFromUnstructured(obj *unstructured.Unstructured, schemaStore *SchemaStore) (*ModuleConfig, error) {
	raw, err := json.Marshal(obj.Object)
	if err != nil {
		return nil, fmt.Errorf("marshal ModuleConfig: %w", err)
	}

	if schemaStore != nil {
		yamlDoc, err := yaml.JSONToYAML(raw)
		if err != nil {
			return nil, fmt.Errorf("convert ModuleConfig to YAML: %w", err)
		}
		if _, err := schemaStore.Validate(&yamlDoc); err != nil && !errors.Is(err, ErrSchemaNotFound) {
			return nil, fmt.Errorf("validate ModuleConfig %q: %w", obj.GetName(), err)
		}
	}

	mc := &ModuleConfig{}
	if err := json.Unmarshal(raw, mc); err != nil {
		return nil, fmt.Errorf("unmarshal ModuleConfig: %w", err)
	}
	return mc, nil
}

func loadLegacyProviderClusterConfig(ctx context.Context, kubeCl *client.KubernetesClient, schemaStore *SchemaStore) (map[string]json.RawMessage, error) {
	secret, err := kubeCl.CoreV1().Secrets(global.ConfigsNS).Get(ctx, legacyProviderClusterConfigSecretName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get Secret %q: %w", legacyProviderClusterConfigSecretName, err)
	}
	return parseLegacyProviderClusterConfig(secret, schemaStore)
}

func parseLegacyProviderClusterConfig(secret *corev1.Secret, schemaStore *SchemaStore) (map[string]json.RawMessage, error) {
	data, ok := secret.Data["cloud-provider-cluster-configuration.yaml"]
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("cloud-provider-cluster-configuration.yaml not found in Secret or empty")
	}
	if _, err := schemaStore.Validate(&data); err != nil {
		return nil, err
	}
	var parsed map[string]json.RawMessage
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func (f *fromClusterMetaConfigFiller) Static(ctx context.Context, metaConfig *MetaConfig) (nilType, error) {
	fillEmptyStaticConfigAndReturn := func(cfg *MetaConfig) (nilType, error) {
		cfg.StaticClusterConfig = nil
		return nil, nil
	}

	staticClusterConfig, err := f.kubeCl.CoreV1().Secrets(global.ConfigsNS).Get(ctx, "d8-static-cluster-configuration", metav1.GetOptions{})
	if err != nil {
		// configuration can be not set because we have auto-discovery
		if k8serrors.IsNotFound(err) {
			return fillEmptyStaticConfigAndReturn(metaConfig)
		}

		return nil, err
	}

	// configuration can be not set because we have auto-discovery
	if len(staticClusterConfig.Data) == 0 {
		return fillEmptyStaticConfigAndReturn(metaConfig)
	}

	staticClusterConfigData, ok := staticClusterConfig.Data["static-cluster-configuration.yaml"]
	if !ok || len(staticClusterConfigData) == 0 {
		// configuration can be not set because we have auto-discovery
		return fillEmptyStaticConfigAndReturn(metaConfig)
	}

	_, err = f.schemaStore.Validate(&staticClusterConfigData)
	if err != nil {
		return nil, err
	}

	var parsedStaticClusterConfig map[string]json.RawMessage
	if err := yaml.Unmarshal(staticClusterConfigData, &parsedStaticClusterConfig); err != nil {
		return nil, err
	}

	metaConfig.StaticClusterConfig = parsedStaticClusterConfig

	return nil, nil
}

func (f *fromClusterMetaConfigFiller) Incorrect(_ context.Context, metaConfig *MetaConfig) (nilType, error) {
	return nil, UnsupportedClusterTypeErr(metaConfig)
}
