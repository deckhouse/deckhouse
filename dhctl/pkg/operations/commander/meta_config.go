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
	"encoding/json"
	"fmt"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

func ParseMetaConfig(ctx context.Context, stateCache state.Cache, params *CommanderModeParams, logger log.Logger) (*config.MetaConfig, error) {
	clusterUUIDBytes, err := stateCache.Load("uuid")
	if err != nil {
		return nil, fmt.Errorf("error loading cluster uuid from state cache: %w", err)
	}
	clusterUUID := string(clusterUUIDBytes)

	configData := fmt.Sprintf("%s\n---\n%s", params.ClusterConfigurationData, params.ProviderClusterConfigurationData)
	metaConfig, err := config.ParseConfigFromData(
		ctx,
		configData,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(logger),
		),
		config.ValidateOptionCommanderMode(true),
		config.ValidateOptionStrictUnmarshal(true),
		config.ValidateOptionValidateExtensions(true),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config: %w", err)
	}
	metaConfig.UUID = clusterUUID

	// OpenAPI validation normalizes string values (e.g. strips trailing \n from block scalars).
	// This causes sshPublicKey to differ from what was stored in cluster Secrets at deploy time,
	// leading to different terraform resource name hashes and false destructive changes.
	// Fix: restore original string values from raw bytes, keeping any defaults added by validation.
	metaConfig.ProviderClusterConfig = mergeRawOverValidated(
		params.ProviderClusterConfigurationData,
		metaConfig.ProviderClusterConfig,
	)
	metaConfig.ClusterConfig = mergeRawOverValidated(
		params.ClusterConfigurationData,
		metaConfig.ClusterConfig,
	)

	return metaConfig, nil
}

// mergeRawOverValidated parses rawBytes directly (preserving original string values like
// trailing \n in block scalars) and merges in any fields from validatedConfig that are
// missing in the raw parse (i.e. defaults added by OpenAPI validation).
func mergeRawOverValidated(rawBytes []byte, validatedConfig map[string]json.RawMessage) map[string]json.RawMessage {
	if len(rawBytes) == 0 || len(validatedConfig) == 0 {
		return validatedConfig
	}

	var rawConfig map[string]json.RawMessage
	if err := yaml.Unmarshal(rawBytes, &rawConfig); err != nil || len(rawConfig) == 0 {
		return validatedConfig
	}

	// Add fields from validatedConfig that are missing in raw (OpenAPI-added defaults).
	for k, v := range validatedConfig {
		if _, exists := rawConfig[k]; !exists {
			rawConfig[k] = v
		}
	}

	return rawConfig
}
