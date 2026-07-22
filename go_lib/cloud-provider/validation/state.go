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

package validation

import cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"

// State holds decoded provider resources used by validation rules.
type State struct {
	// InstanceClassKind is the provider InstanceClass resource kind.
	InstanceClassKind string
	// NamespaceName is the module namespace used for credential Secrets and migration markers.
	NamespaceName string
	// ModuleName is the cloud-provider ModuleConfig name.
	ModuleName string
	// ModuleConfig is the decoded cloud-provider ModuleConfig resource.
	ModuleConfig *cpapi.ModuleConfig
	// CredentialSecrets holds managed credential Secrets from the module namespace.
	CredentialSecrets []cpapi.CredentialSecret
	// NodeGroups holds CloudPermanent NodeGroups used for cross-resource validation.
	NodeGroups []cpapi.NodeGroup
	// InstanceClasses holds provider InstanceClass resources of InstanceClassKind.
	InstanceClasses []cpapi.InstanceClass
	// LegacyProviderClusterConfig holds the legacy providerClusterConfiguration section.
	LegacyProviderClusterConfig map[string]any
	// MigrationStatus controls whether new-model validation should run.
	MigrationStatus cpapi.MigrationStatus
}
