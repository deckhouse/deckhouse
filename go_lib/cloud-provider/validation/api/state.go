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

import (
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

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

func (s State) ListCredentialSecrets() []cpapi.CredentialSecret {
	secrets := make([]cpapi.CredentialSecret, 0, len(s.CredentialSecrets))
	for _, secret := range s.CredentialSecrets {
		if secret.Namespace != "" && secret.Namespace != s.NamespaceName {
			continue
		}

		if !secret.IsManaged() {
			continue
		}

		secrets = append(secrets, secret)
	}

	return secrets
}

func (s State) ExistsCredentialSecret(name string) bool {
	for _, secret := range s.CredentialSecrets {
		if secret.Name != name {
			continue
		}

		if secret.Namespace != "" && secret.Namespace != s.NamespaceName {
			continue
		}

		if !secret.IsManaged() {
			continue
		}

		return true
	}

	return false
}

func (s State) ExistsNodeGroup(name string) bool {
	for _, nodeGroup := range s.NodeGroups {
		if nodeGroup.Name == name {
			return true
		}
	}

	return false
}

func (s State) ExistsInstanceClass(name string) bool {
	for _, class := range s.InstanceClasses {
		if class.Name == name {
			return true
		}
	}

	return false
}

func (s State) ListInstanceClassConsumers() map[string][]string {
	result := make(map[string][]string, len(s.NodeGroups))
	for _, nodeGroup := range s.NodeGroups {
		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}

		classRef := nodeGroup.Spec.CloudInstances.ClassReference
		if classRef.Kind != s.InstanceClassKind || classRef.Name == "" {
			continue
		}

		result[classRef.Name] = append(result[classRef.Name], nodeGroup.Name)
	}

	return result
}
