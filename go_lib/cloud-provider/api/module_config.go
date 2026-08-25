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

// ModuleSettingsObject is provider ModuleConfig settings usable by common validation rules.
//
// A section is present when its value differs from the zero value of the section type.
type ModuleSettingsObject interface {
	// HasProviderSection reports whether the provider settings section is set.
	HasProviderSection() bool
	// HasNodesSection reports whether the nodes settings section is set.
	HasNodesSection() bool
	// HasStorageSection reports whether the storage settings section is set.
	HasStorageSection() bool
	// HasCCMSection reports whether the ccm settings section is set.
	HasCCMSection() bool
}

// ModuleConfig is a typed view of the cloud-provider ModuleConfig resource.
//
// Design note: we use a lightweight typed wrapper here instead of importing
// the canonical ModuleConfig from deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1
// because that type stores settings as *MappedFields (runtime.RawExtension,
// untyped) and lives in the heavyweight deckhouse-controller go.mod. The settings
// type parameter lets every provider plug in its own generated settings type.
type ModuleConfig[S ModuleSettingsObject] struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec ModuleConfigSpec[S] `json:"spec,omitempty"`
}

// ModuleConfigSpec holds the enabled flag, schema version, and typed module settings.
type ModuleConfigSpec[S ModuleSettingsObject] struct {
	Enabled  *bool `json:"enabled,omitempty"`
	Version  int   `json:"version,omitempty"`
	Settings S     `json:"settings,omitempty"`
}
