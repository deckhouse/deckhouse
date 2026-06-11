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
func (b *StateBuilder) Build(input proto.PrepareInput) (*cpval.State, error) {
	var err error
	state := &cpval.State{
		InstanceClassKind:           b.config.InstanceClassKind,
		NamespaceName:               b.config.NamespaceName,
		ModuleName:                  b.config.ModuleName,
		LegacyProviderClusterConfig: input.ProviderClusterConfig,
	}

	if input.Vars != nil {
		state.ModuleConfig, err = cpval.DecodeModuleConfigForModule(b.config.ModuleName, input.Vars.Settings)
		if err != nil {
			return nil, err
		}

		state.CredentialSecrets, err = cpval.DecodeCredentialSecrets(input.Vars.Secrets)
		if err != nil {
			return nil, err
		}

		state.NodeGroups, err = cpval.DecodeNodeGroups(input.Vars.NodeGroups)
		if err != nil {
			return nil, err
		}

		state.InstanceClasses, err = cpval.DecodeInstanceClasses(input.Vars.InstanceClasses)
		if err != nil {
			return nil, err
		}
	}

	if state.ModuleConfig != nil && state.ModuleConfig.Name == "" {
		state.ModuleConfig.Name = b.config.ModuleName
	}

	if b.config.MigrationRules != nil {
		state.MigrationStatus = cpval.MigrationStatusFromState(state, b.config.MigrationRules)
	}

	return state, nil
}
