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

package api

// ModuleConfig is a typed view of the cloud-provider ModuleConfig resource.
//
// Design note: we use a lightweight typed wrapper here instead of importing
// the canonical ModuleConfig from deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1
// because that type stores settings as *MappedFields (runtime.RawExtension,
// untyped) and lives in the heavyweight deckhouse-controller go.mod. This
// wrapper gives us typed access to the provider-specific settings fields
// (Provider, Storage, Nodes, Ccm) without dragging in the full controller stack.
type ModuleConfig struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleConfigSpec `json:"spec,omitempty"`
}

// ModuleConfigSpec holds the enabled flag, schema version, and module settings.
type ModuleConfigSpec struct {
	Enabled  *bool                    `json:"enabled,omitempty"`
	Version  int                      `json:"version,omitempty"`
	Settings ModuleConfigSpecSettings `json:"settings,omitempty"`
}

// ModuleConfigSpecSettings groups provider and subsystem settings.
type ModuleConfigSpecSettings struct {
	Provider *ModuleConfigSpecProviderSettings  `json:"provider,omitempty"`
	Storage  *ModuleConfigSpecSubsystemSettings `json:"storage,omitempty"`
	Nodes    *ModuleConfigSpecSubsystemSettings `json:"nodes,omitempty"`
	CCM      *ModuleConfigSpecSubsystemSettings `json:"ccm,omitempty"`
}

// ModuleConfigSpecProviderSettings holds provider-level enablement flags.
type ModuleConfigSpecProviderSettings struct {
	Parameters map[string]any `json:"parameters,omitempty"`
}

// ModuleConfigSpecSubsystemSettings holds subsystem disablement and parameters.
type ModuleConfigSpecSubsystemSettings struct {
	Disabled   *bool          `json:"disabled,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
}
