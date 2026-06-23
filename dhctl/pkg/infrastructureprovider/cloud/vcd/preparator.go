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
	"strings"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/validation"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

type clientProvider func(pcc map[string]json.RawMessage, l log.Logger) (cloudClient, error)

type MetaConfigPreparator struct {
	logger         log.Logger
	clientProvider clientProvider
}

func NewMetaConfigPreparator(logger log.Logger) *MetaConfigPreparator {
	if govalue.IsNil(logger) {
		logger = log.GetSilentLogger()
	}
	return &MetaConfigPreparator{
		logger:         logger,
		clientProvider: newVcdCloudClient,
	}
}

func (p MetaConfigPreparator) Validate(_ context.Context, input config.ProviderInput) error {
	if err := validation.DefaultPrefixValidator(input.ClusterPrefix); err != nil {
		return fmt.Errorf("%v for provider %s", err, ProviderName)
	}

	raw, ok := input.ProviderClusterConfig["provider"]
	if !ok {
		return fmt.Errorf("unable to unmarshal vcd provider configuration: provider key missing")
	}

	var providerConfiguration providerConfig
	if err := json.Unmarshal(raw, &providerConfiguration); err != nil {
		return fmt.Errorf("unable to unmarshal vcd provider configuration: %v", err)
	}

	server := strings.TrimSpace(providerConfiguration.Server)
	if server == "" {
		return nil
	}

	if strings.HasSuffix(server, "/") {
		return fmt.Errorf("provider.server must not end with a slash '/'")
	}

	return nil
}

func (p MetaConfigPreparator) Prepare(ctx context.Context, input config.ProviderInput) (proto.PrepareResult, error) {
	client, err := p.clientProvider(input.ProviderClusterConfig, p.logger)
	if err != nil {
		return proto.PrepareResult{}, fmt.Errorf("Cannot get cloud client: %w", err)
	}

	apiVersion, err := client.GetVersion(ctx)
	if err != nil {
		return proto.PrepareResult{}, err
	}

	var result proto.PrepareResult
	if err := versionConstraintAction(apiVersion, p.logger, func(legacy bool) error {
		if !legacy {
			return nil
		}
		if _, ok := input.ProviderClusterConfig["legacyMode"]; ok {
			return nil
		}
		result.ProviderClusterConfig = map[string]interface{}{"legacyMode": true}
		return nil
	}); err != nil {
		return proto.PrepareResult{}, err
	}

	return result, nil
}
