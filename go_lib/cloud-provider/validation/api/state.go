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
	"reflect"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

// State holds decoded provider resources used by validation rules.
//
// IC, S and PCC are the provider InstanceClass, ModuleConfig settings and
// providerClusterConfiguration types; they are instantiated with pointer types.
type State[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
] struct {
	// NamespaceName is the module namespace used for credential Secrets and migration markers.
	NamespaceName string
	// ModuleName is the cloud-provider ModuleConfig name.
	ModuleName string
	// ModuleConfig is the decoded cloud-provider ModuleConfig resource.
	ModuleConfig *cpapi.ModuleConfig[S]
	// CredentialSecrets holds managed credential Secrets from the module namespace.
	CredentialSecrets []cpapi.CredentialSecret
	// NodeGroups holds the NodeGroups used for cross-resource validation.
	//
	// The set is not restricted to CloudPermanent: rules that only concern cloud-permanent
	// nodes filter by nodeType themselves (FindCloudPermanentNodeGroup,
	// ListInstanceClassConsumers), while ValidateInstanceClassDeletion deliberately looks at
	// every node type — a CloudEphemeral NodeGroup still using a class must block its removal.
	NodeGroups []cpapi.NodeGroup
	// InstanceClasses holds provider InstanceClass resources of InstanceClassKind.
	InstanceClasses []IC
	// ProviderClusterConfig holds the legacy providerClusterConfiguration resource.
	ProviderClusterConfig PCC
	// MigrationStatus controls whether new-model validation should run.
	MigrationStatus cpapi.MigrationStatus
}

// IsResourceAbsent reports whether a provider resource value is unset:
// an invalid value, a nil pointer or reference type, or a zero struct.
func IsResourceAbsent(value any) bool {
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return true
	}

	switch reflected.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return reflected.IsNil()
	default:
		return reflected.IsZero()
	}
}

// HasProviderClusterConfig reports whether the legacy providerClusterConfiguration is present.
func (s State[IC, S, PCC]) HasProviderClusterConfig() bool {
	return !IsResourceAbsent(s.ProviderClusterConfig)
}

// ListCredentialSecrets returns managed credential Secrets that belong to the module namespace.
//
// A Secret with an empty Namespace is treated as belonging to the module: on the dhctl protocol
// path secrets are keyed as <namespace>/<name> and metadata.namespace may be left unset, so the
// key carries the namespace instead of the object. This is a deliberate allowance, not a gap.
func (s State[IC, S, PCC]) ListCredentialSecrets() []cpapi.CredentialSecret {
	secrets := make([]cpapi.CredentialSecret, 0, len(s.CredentialSecrets))
	for _, secret := range s.CredentialSecrets {
		if !s.isModuleCredentialSecret(secret) {
			continue
		}

		secrets = append(secrets, secret)
	}

	return secrets
}

// FindCredentialSecret returns the managed credential Secret with the given name.
// It applies the same namespace rules as ListCredentialSecrets.
func (s State[IC, S, PCC]) FindCredentialSecret(name string) (cpapi.CredentialSecret, bool) {
	for _, secret := range s.CredentialSecrets {
		if secret.Name != name {
			continue
		}

		if !s.isModuleCredentialSecret(secret) {
			continue
		}

		return secret, true
	}

	return cpapi.CredentialSecret{}, false
}

// ExistsCredentialSecret reports whether a managed credential Secret with the given name exists.
func (s State[IC, S, PCC]) ExistsCredentialSecret(name string) bool {
	_, ok := s.FindCredentialSecret(name)
	return ok
}

// isModuleCredentialSecret reports whether the Secret is managed and lives in the module namespace.
// See ListCredentialSecrets for why an empty namespace is accepted.
func (s State[IC, S, PCC]) isModuleCredentialSecret(secret cpapi.CredentialSecret) bool {
	if secret.Namespace != "" && secret.Namespace != s.NamespaceName {
		return false
	}

	return secret.IsManaged()
}

// FindNodeGroup returns the NodeGroup with the given name, whatever its nodeType.
func (s State[IC, S, PCC]) FindNodeGroup(name string) (cpapi.NodeGroup, bool) {
	for _, nodeGroup := range s.NodeGroups {
		if nodeGroup.Name == name {
			return nodeGroup, true
		}
	}

	return cpapi.NodeGroup{}, false
}

// ExistsNodeGroup reports whether a NodeGroup with the given name exists, whatever its nodeType.
func (s State[IC, S, PCC]) ExistsNodeGroup(name string) bool {
	_, ok := s.FindNodeGroup(name)
	return ok
}

// FindCloudPermanentNodeGroup returns the CloudPermanent NodeGroup with the given name.
//
// Rules about cloud-permanent nodes must use this instead of FindNodeGroup: the state may
// carry NodeGroups of any node type, so a name match alone would accept, say, a Static
// NodeGroup named "master".
func (s State[IC, S, PCC]) FindCloudPermanentNodeGroup(name string) (cpapi.NodeGroup, bool) {
	for _, nodeGroup := range s.NodeGroups {
		if nodeGroup.Name == name && nodeGroup.Spec.NodeType == cpapi.NodeTypeCloudPermanent {
			return nodeGroup, true
		}
	}

	return cpapi.NodeGroup{}, false
}

// ExistsCloudPermanentNodeGroup reports whether a CloudPermanent NodeGroup with the given name exists.
func (s State[IC, S, PCC]) ExistsCloudPermanentNodeGroup(name string) bool {
	_, ok := s.FindCloudPermanentNodeGroup(name)
	return ok
}

// FindInstanceClass returns the provider InstanceClass with the given name.
func (s State[IC, S, PCC]) FindInstanceClass(name string) (IC, bool) {
	for _, class := range s.InstanceClasses {
		if class.GetName() == name {
			return class, true
		}
	}

	var absent IC
	return absent, false
}

// ExistsInstanceClass reports whether a provider InstanceClass with the given name exists.
func (s State[IC, S, PCC]) ExistsInstanceClass(name string) bool {
	_, ok := s.FindInstanceClass(name)
	return ok
}

// ListInstanceClassConsumers maps an InstanceClass name to the names of NodeGroups referencing it.
func (s State[IC, S, PCC]) ListInstanceClassConsumers() map[string][]string {
	var absentClass IC
	expectedKind := absentClass.GroupVersionKind().Kind

	result := make(map[string][]string, len(s.NodeGroups))
	for _, nodeGroup := range s.NodeGroups {
		if nodeGroup.Spec.NodeType != cpapi.NodeTypeCloudPermanent {
			continue
		}

		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}

		classRef := nodeGroup.Spec.CloudInstances.ClassReference
		if classRef.Name == "" {
			continue
		}
		if classRef.Kind != expectedKind {
			continue
		}

		result[classRef.Name] = append(result[classRef.Name], nodeGroup.Name)
	}

	return result
}
