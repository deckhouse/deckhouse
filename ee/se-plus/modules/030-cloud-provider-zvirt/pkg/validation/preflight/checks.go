/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package preflight

import (
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	zval "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation"
)

// ValidatePreflight checks resources required before cluster bootstrap or converge.
func ValidatePreflight(state *zval.State) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	if state.HasProviderClusterConfig() {
		result.Merge(
			zval.ValidateLegacyProviderConnection(state.ProviderClusterConfig),
			zval.ValidateLegacyCustomNetworkConfigParameters(state.ProviderClusterConfig),
			zval.ValidateLegacyCustomNetworkConfigsCoverNodeGroupReplicas(state.ProviderClusterConfig),
		)
	}

	if cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return result
	}

	result.Merge(
		cpval.ValidateModuleConfig(state),
		cpval.ValidateCredentialSecretPresence(state, cpapi.CredentialSecretName),
		cpval.ValidateCredentialSecretContent(state, cpapi.CredentialSecretName, zval.CredentialsValidator),
		cpval.ValidateMasterNodeGroupPresence(state),
		cpval.ValidateNodeGroupsClassReference(state, true),
		cpval.ValidateInstanceClassesEtcdDisk(state),
		zval.ValidateProviderConnection(state),
		zval.ValidateCustomNetworkConfigParameters(state),
		zval.ValidateCustomNetworkConfigsCoverNodeGroupReplicas(state, true),
	)

	return result
}
