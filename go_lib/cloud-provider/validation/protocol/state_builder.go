// Copyright 2026 Flant JSC
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

package protocol

import (
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
)

// StateBuilderConfig holds provider-specific settings for dhctl protocol state building.
type StateBuilderConfig struct {
	// InstanceClassKind is the provider InstanceClass resource kind.
	InstanceClassKind string
	// NamespaceName is the module namespace used for credential Secrets.
	NamespaceName string
	// ModuleName is the cloud-provider ModuleConfig name.
	ModuleName string
	// AllowedCredentialAuthSchemes lists auth schemes supported by the provider.
	AllowedCredentialAuthSchemes []cpapi.AuthScheme
	// MigrationRules configures migration completeness checks for dhctl protocol input.
	// When nil, MigrationStatus is not derived from legacy ProviderClusterConfiguration.
	MigrationRules *cpval.MigrationRules
}

// StateBuilder decodes dhctl provider input into a validation State.
type StateBuilder struct {
	config StateBuilderConfig
}

// NewStateBuilder creates a protocol state builder for the given provider configuration.
func NewStateBuilder(config StateBuilderConfig) *StateBuilder {
	return &StateBuilder{config: config}
}

// Build decodes dhctl input and applies provider context from the builder configuration.
func (b *StateBuilder) Build(input proto.PrepareInput, vars *proto.CloudProviderVars) (*cpval.State, error) {
	state := &cpval.State{
		LegacyProviderClusterConfig: input.ProviderClusterConfig,
	}

	moduleConfig, err := cpval.DecodeModuleConfigForModule(b.config.ModuleName, input.ModuleConfig)
	if err != nil {
		return nil, err
	}
	state.ModuleConfig = moduleConfig

	if vars != nil {
		state.CredentialSecrets, err = cpval.DecodeCredentialSecrets(vars)
		if err != nil {
			return nil, err
		}

		state.NodeGroups, err = cpval.DecodeNodeGroups(vars.NodeGroups)
		if err != nil {
			return nil, err
		}

		state.InstanceClasses, err = cpval.DecodeInstanceClasses(vars.InstanceClasses)
		if err != nil {
			return nil, err
		}
	}

	b.applyProviderContext(state)

	if b.config.MigrationRules != nil {
		state.MigrationStatus = cpval.MigrationStatusFromState(state, b.config.MigrationRules)
	}

	return state, nil
}

func (b *StateBuilder) applyProviderContext(state *cpval.State) {
	state.ModuleName = b.config.ModuleName
	state.NamespaceName = b.config.NamespaceName
	state.InstanceClassKind = b.config.InstanceClassKind
	state.AllowedCredentialAuthSchemes = b.config.AllowedCredentialAuthSchemes

	if state.ModuleConfig != nil && state.ModuleConfig.Name == "" {
		state.ModuleConfig.Name = b.config.ModuleName
	}
}
