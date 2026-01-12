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
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

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

func (f *fromClusterMetaConfigFiller) Cloud(ctx context.Context, metaConfig *MetaConfig) (nilType, error) {
	providerClusterConfig, err := f.kubeCl.CoreV1().Secrets(global.ConfigsNS).Get(ctx, "d8-provider-cluster-configuration", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	providerClusterConfigData, ok := providerClusterConfig.Data["cloud-provider-cluster-configuration.yaml"]
	if !ok || len(providerClusterConfigData) == 0 {
		return nil, fmt.Errorf("cloud-provider-cluster-configuration.yaml not found in secret or empty")
	}

	_, err = f.schemaStore.Validate(&providerClusterConfigData)
	if err != nil {
		return nil, err
	}

	var parsedProviderClusterConfig map[string]json.RawMessage
	if err := yaml.Unmarshal(providerClusterConfigData, &parsedProviderClusterConfig); err != nil {
		return nil, err
	}

	metaConfig.ProviderClusterConfig = parsedProviderClusterConfig

	return nil, nil
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
