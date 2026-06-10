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

import (
	"fmt"
	"strings"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
)

// ValidateInvariants runs DVP validation rules for the current cluster state.
func ValidateInvariants(state *cpval.State) cpval.Result {
	result := cpval.Result{}
	if state == nil || cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return result
	}

	result.Merge(ValidateModuleConfig(state))
	result.Merge(ValidateCredentials(state, state.ModuleEnabled()))
	result.Merge(ValidateInstanceClasses(state))

	return result
}

// ValidateModuleConfig checks ModuleConfig presence and module-specific invariants.
func ValidateModuleConfig(state *cpval.State) cpval.Result {
	result := cpval.Result{}
	if state.ModuleConfig == nil {
		if len(state.LegacyProviderClusterConfig) == 0 {
			result.AddError("ModuleConfig", "module_config_required", "ModuleConfig is required")
		}
		return result
	}

	moduleConfig := state.ModuleConfig
	if moduleConfig.Name != "" && moduleConfig.Name != ModuleName {
		result.AddError("ModuleConfig.metadata.name", "invalid_module_config_name", fmt.Sprintf("must be %q", ModuleName))
	}

	return result
}

// ValidateCredentials checks managed credential Secrets in the module namespace.
func ValidateCredentials(state *cpval.State, requirePrimary bool) cpval.Result {
	result := cpval.Result{}
	secrets := make([]cpapi.CredentialSecret, 0, len(state.CredentialSecrets))
	foundPrimary := false

	for _, secret := range state.CredentialSecrets {
		if secret.Namespace != "" && secret.Namespace != Namespace {
			continue
		}

		if !secret.IsManaged() {
			continue
		}

		if secret.Name == cpapi.CredentialSecretName {
			foundPrimary = true
		}

		if secret.Type != cpapi.CredentialsSecretType {
			result.AddError(
				fmt.Sprintf("Secret/%s.type", secret.Name),
				"invalid_credential_secret_type",
				fmt.Sprintf("credential Secret type must be %q", cpapi.CredentialsSecretType),
			)
		}

		secrets = append(secrets, secret)
	}

	result.Merge(cpval.ValidateCredentialSecrets(secrets, AllowedCredentialAuthSchemes))

	if requirePrimary && !foundPrimary {
		result.AddError(
			fmt.Sprintf("Secret/%s", cpapi.CredentialSecretName),
			"credential_secret_required",
			fmt.Sprintf("credential Secret %q is required", cpapi.CredentialSecretName),
		)
	}

	return result
}

// ValidateInstanceClasses checks DVP InstanceClass attachment invariants.
func ValidateInstanceClasses(state *cpval.State) cpval.Result {
	return cpval.ValidateInstanceClassEtcdDiskAttachment(
		InstanceClassKind,
		state.NodeGroups,
		state.InstanceClasses,
	)
}

// ValidateInstanceClassDelete checks whether an InstanceClass can be safely deleted.
func ValidateInstanceClassDelete(state *cpval.State, className string, deletedClass *cpapi.InstanceClass) cpval.Result {
	result := cpval.Result{}
	if strings.TrimSpace(className) == "" && deletedClass != nil {
		className = deletedClass.Name
	}
	if strings.TrimSpace(className) == "" {
		return result
	}

	for _, nodeGroup := range state.NodeGroups {
		if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
			continue
		}
		ref := nodeGroup.Spec.CloudInstances.ClassReference
		if ref.Kind == InstanceClassKind && ref.Name == className {
			result.AddError(InstanceClassKind+"/"+className, "instance_class_in_use", fmt.Sprintf("InstanceClass is used by NodeGroup %q", nodeGroup.Name))
		}
	}

	if deletedClass != nil && len(deletedClass.Status.NodeGroupConsumers) > 0 {
		result.AddError(
			InstanceClassKind+"/"+className,
			"instance_class_has_consumers",
			fmt.Sprintf("DVPInstanceClass is used by %d NodeGroup consumers", len(deletedClass.Status.NodeGroupConsumers)),
		)
	}

	return result
}
