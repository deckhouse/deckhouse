// Copyright 2023 Flant JSC
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

package commander

import (
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

// ParseMetaConfig parses commander-mode config. operation
// (infrastructureprovider.DhctlOperation*) reaches the provider preparator,
// which skips bootstrap-only checks on other operations.
func ParseMetaConfig(ctx context.Context, stateCache state.Cache, params *CommanderModeParams, operation infrastructureprovider.DhctlOperation, kubeClient config.KubeClientGetter) (*config.MetaConfig, error) {
	clusterUUIDBytes, err := stateCache.Load(ctx, "uuid")
	if err != nil {
		return nil, fmt.Errorf("error loading cluster uuid from state cache: %w", err)
	}
	clusterUUID := string(clusterUUIDBytes)
	if clusterUUID == "" {
		return nil, fmt.Errorf("error loading cluster uuid from state cache: uuid is empty")
	}

	preparatorParams := infrastructureprovider.NewPreparatorProviderParams()

	// Commander does not send registry_config, so the external provider bundle
	// registry is unknown from the request. Read it from the target cluster and
	// deliver the bundle before parsing; the parse below then finds it on disk
	// and skips the registry-less download.
	if kubeClient != nil {
		if err := config.EnsureExternalProviderBundle(ctx, kubeClient, string(params.ClusterConfigurationData), nil); err != nil {
			return nil, fmt.Errorf("ensure provider bundle from cluster: %w", err)
		}
	}

	configData := fmt.Sprintf("%s\n---\n%s", params.ClusterConfigurationData, params.ProviderClusterConfigurationData)
	metaConfig, err := config.ParseConfigFromDataEnsureProvider(
		ctx,
		configData,
		string(params.RegistryConfigurationData),
		infrastructureprovider.MetaConfigPreparatorProvider(preparatorParams),
		nil,
		config.ValidateOptionCommanderMode(true),
		config.ValidateOptionStrictUnmarshal(true),
		config.ValidateOptionValidateExtensions(true),
		config.ValidateOptionOperation(operation),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config: %w", err)
	}
	metaConfig.UUID = clusterUUID

	return metaConfig, nil
}
