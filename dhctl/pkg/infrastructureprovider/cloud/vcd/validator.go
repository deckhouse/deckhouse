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

// MetaConfigValidator checks the provider.server format and, when
// validatePrefix is set, the cluster prefix. The in-cluster hook validates with
// validatePrefix=false (no prefix there, same as before the module-config
// migration); dhctl operations validate with true. Validation never mutates the
// config; the legacyMode rewrite lives separately in EnsureLegacyMode.
type MetaConfigValidator struct {
	validatePrefix bool
}

func NewMetaConfigValidator(validatePrefix bool) *MetaConfigValidator {
	return &MetaConfigValidator{validatePrefix: validatePrefix}
}

func (p *MetaConfigValidator) Validate(_ context.Context, input config.ProviderInput) error {
	if p.validatePrefix {
		if err := validation.DefaultPrefixValidator(input.ClusterPrefix); err != nil {
			return fmt.Errorf("%v for provider %s", err, ProviderName)
		}
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

// EnsureLegacyMode sets providerClusterConfiguration.legacyMode for VCD APIs
// older than the current contract. This is the only provider-side rewrite of a
// parsed config in dhctl; it is deliberately not part of any validator and is
// invoked as an explicit vcd special case when the infrastructure provider is
// built. Idempotent: an already-present legacyMode key (user-set or from a
// previous call) is left untouched, so at most one VCD API request is made.
func EnsureLegacyMode(ctx context.Context, metaConfig *config.MetaConfig) error {
	return ensureLegacyMode(ctx, metaConfig, newVcdCloudClient)
}

func ensureLegacyMode(ctx context.Context, metaConfig *config.MetaConfig, clients clientProvider) error {
	if _, ok := metaConfig.ProviderClusterConfig["legacyMode"]; ok {
		return nil
	}

	client, err := clients(metaConfig.ProviderClusterConfig)
	if err != nil {
		return fmt.Errorf("Cannot get cloud client: %w", err)
	}

	apiVersion, err := client.GetVersion(ctx)
	if err != nil {
		return err
	}

	return versionConstraintAction(ctx, apiVersion, func(legacy bool) error {
		raw, err := json.Marshal(legacy)
		if err != nil {
			return fmt.Errorf("marshal legacyMode: %w", err)
		}
		if metaConfig.ProviderClusterConfig == nil {
			metaConfig.ProviderClusterConfig = make(map[string]json.RawMessage, 1)
		}
		metaConfig.ProviderClusterConfig["legacyMode"] = raw
		return nil
	})
}
