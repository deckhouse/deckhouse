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
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

// StateBuilderConfig holds provider-specific settings for dhctl protocol state building.
type StateBuilderConfig struct {
	// NamespaceName is the module namespace used for credential Secrets.
	NamespaceName string
	// ModuleName is the cloud-provider ModuleConfig name.
	ModuleName string
}

// StateBuilderFactory produces a fresh StateBuilder per dhctl validation request.
//
// It mirrors the admission factory so both surfaces are constructed the same way. Unlike
// admission, the dhctl protocol delivers every resource in a single payload, so the builder
// needs no chain of Add* steps: Build decodes the whole input at once.
type StateBuilderFactory[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
] struct {
	config StateBuilderConfig
}

// NewStateBuilderFactory creates a protocol state builder factory for the given provider configuration.
func NewStateBuilderFactory[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
](config StateBuilderConfig) *StateBuilderFactory[IC, S, PCC] {
	return &StateBuilderFactory[IC, S, PCC]{config: config}
}

// CreateBuilder returns a builder for a single dhctl validation request.
func (f *StateBuilderFactory[IC, S, PCC]) CreateBuilder() *StateBuilder[IC, S, PCC] {
	return &StateBuilder[IC, S, PCC]{config: f.config}
}

// StateBuilder decodes dhctl provider input into a validation State.
type StateBuilder[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
] struct {
	config StateBuilderConfig
}

// Build decodes dhctl input and applies provider context from the builder configuration.
func (b *StateBuilder[IC, S, PCC]) Build(input proto.ValidateInput) (*cpvalapi.State[IC, S, PCC], error) {
	var err error

	state := &cpvalapi.State[IC, S, PCC]{
		NamespaceName: b.config.NamespaceName,
		ModuleName:    b.config.ModuleName,
	}

	state.ProviderClusterConfig, err = cpval.DecodeProviderClusterConfig[PCC](input.ProviderClusterConfig)
	if err != nil {
		return nil, err
	}

	if input.CloudProviderVars != nil {
		state.ModuleConfig, err = cpval.DecodeModuleConfig[S](b.config.ModuleName, input.CloudProviderVars.Settings)
		if err != nil {
			return nil, err
		}

		state.CredentialSecrets, err = cpval.DecodeCredentialSecrets(input.CloudProviderVars.Secrets)
		if err != nil {
			return nil, err
		}

		nodeGroups, err := cpval.DecodeNodeGroups(input.CloudProviderVars.NodeGroups)
		if err != nil {
			return nil, err
		}

		// State.NodeGroups holds CloudPermanent NodeGroups only: keep this surface in sync
		// with the admission state builder so a rule behaves the same in dhctl preflight
		// and in the webhook.
		state.NodeGroups = make([]cpapi.NodeGroup, 0, len(nodeGroups))
		for _, nodeGroup := range nodeGroups {
			if nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
				continue
			}

			state.NodeGroups = append(state.NodeGroups, nodeGroup)
		}

		state.InstanceClasses, err = cpval.DecodeInstanceClasses[IC](input.CloudProviderVars.InstanceClasses)
		if err != nil {
			return nil, err
		}
	}

	if state.ModuleConfig != nil && state.ModuleConfig.Name == "" {
		state.ModuleConfig.Name = b.config.ModuleName
	}

	state.MigrationStatus = cpvalapi.MigrationStatusFromState(state)

	return state, nil
}
