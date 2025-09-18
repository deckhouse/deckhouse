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

	return metaConfig, nil
}
