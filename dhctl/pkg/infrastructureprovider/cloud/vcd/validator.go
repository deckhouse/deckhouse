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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/validation"
)

type clientProvider func(pcc map[string]json.RawMessage) (cloudClient, error)

type MetaConfigValidator struct {
	clientProvider clientProvider
}

func NewMetaConfigValidator() *MetaConfigValidator {
	return &MetaConfigValidator{
		clientProvider: newVcdCloudClient,
	}
}

func (p MetaConfigValidator) Validate(_ context.Context, input config.ProviderInput) error {
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

// PatchProviderClusterConfig injects legacyMode for VCD APIs older than the
// current contract. It is the only provider-side rewrite of a parsed config in
// dhctl, so config picks it up through an optional-method check rather than a
// shared interface.
func (p MetaConfigValidator) PatchProviderClusterConfig(ctx context.Context, input config.ProviderInput) (map[string]any, error) {
	client, err := p.clientProvider(input.ProviderClusterConfig)
	if err != nil {
		return nil, fmt.Errorf("Cannot get cloud client: %w", err)
	}

	apiVersion, err := client.GetVersion(ctx)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := versionConstraintAction(ctx, apiVersion, func(legacy bool) error {
		if !legacy {
			return nil
		}
		if _, ok := input.ProviderClusterConfig["legacyMode"]; ok {
			return nil
		}
		result = map[string]any{"legacyMode": true}
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}
