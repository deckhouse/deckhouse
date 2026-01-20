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

package vcd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	vcdpreflight "github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/vcd/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
)

type clientProvider func(m *config.MetaConfig, l log.Logger) (cloudClient, error)

type MetaConfigPreparatorParams struct {
	PrepareMetaConfig     bool
	ValidateClusterPrefix bool
}

type MetaConfigPreparator struct {
	params         MetaConfigPreparatorParams
	logger         log.Logger
	clientProvider clientProvider
}

func NewMetaConfigPreparatorWithoutLogger(params MetaConfigPreparatorParams) *MetaConfigPreparator {
	return NewMetaConfigPreparator(params, log.GetSilentLogger())
}

func NewMetaConfigPreparator(params MetaConfigPreparatorParams, logger log.Logger) *MetaConfigPreparator {
	return &MetaConfigPreparator{
		params:         params,
		logger:         logger,
		clientProvider: newVcdCloudClient,
	}
}

func (p MetaConfigPreparator) Validate(ctx context.Context, metaConfig *config.MetaConfig) error {
	return preflightnew.RunSuite(ctx, preflightnew.NewSuite(
		vcdpreflight.ConfigCheck(vcdpreflight.ConfigDeps{
			MetaConfig:      metaConfig,
			ValidatePrefix:  p.params.ValidateClusterPrefix,
			CheckServerPath: true,
		}),
	), preflightnew.PhaseProviderConfigCheck)
}

func (p MetaConfigPreparator) Prepare(ctx context.Context, metaConfig *config.MetaConfig) error {
	if !p.params.PrepareMetaConfig {
		return nil
	}

	client, err := p.clientProvider(metaConfig, p.logger)
	if err != nil {
		return fmt.Errorf("Cannot get cloud client: %w", err)
	}

	apiVersion, err := client.GetVersion(ctx)
	if err != nil {
		return err
	}

	return versionConstraintAction(apiVersion, p.logger, func(legacy bool) error {
		if !legacy {
			return nil
		}

		if _, ok := metaConfig.ProviderClusterConfig["legacyMode"]; ok {
			return nil
		}

		legacyMode, err := json.Marshal(true)
		if err != nil {
			return fmt.Errorf("failed to marshal legacyMode: %v", err)
		}

		metaConfig.ProviderClusterConfig["legacyMode"] = legacyMode

		return nil
	})
}
