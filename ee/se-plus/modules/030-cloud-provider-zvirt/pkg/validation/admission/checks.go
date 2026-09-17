/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package admission

import (
	admissionv1 "k8s.io/api/admission/v1"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1"
	zval "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation"
)

// ValidateCredentialSecret validates credential Secret admission requests.
func ValidateCredentialSecret(state *zval.State, operation admissionv1.Operation) cpvalapi.Result {
	result, ok := validationResult(state)
	if !ok {
		return result
	}

	switch operation {
	case admissionv1.Create, admissionv1.Update:
		result.Merge(
			cpval.ValidateCredentialSecretContent(state, cpapi.CredentialSecretName, zval.CredentialsValidator),
		)
	}

	return result
}

// ValidateModuleConfig validates ModuleConfig admission requests
func ValidateModuleConfig(state *zval.State, operation admissionv1.Operation) cpvalapi.Result {
	result, ok := validationResult(state)
	if !ok {
		return result
	}

	switch operation {
	case admissionv1.Create, admissionv1.Update:
		result.Merge(
			zval.ValidateProviderConnection(state),
		)
	}

	return result
}

// ValidateInstanceClass validates ZvirtInstanceClass admission requests.
// deletedClass must be set when operation is Delete.
func ValidateInstanceClass(
	state *zval.State,
	operation admissionv1.Operation,
	deletedClass *zicv1.ZvirtInstanceClass,
) cpvalapi.Result {
	result, ok := validationResult(state)
	if !ok {
		return result
	}

	switch operation {
	case admissionv1.Create, admissionv1.Update:
		result.Merge(
			cpval.ValidateInstanceClassesEtcdDisk(state),
		)
	case admissionv1.Delete:
		result.Merge(
			cpval.ValidateInstanceClassDeletion(state, deletedClass),
		)
	}

	return result
}

// ValidateNodeGroup validates NodeGroup admission requests.
func ValidateNodeGroup(state *zval.State, operation admissionv1.Operation) cpvalapi.Result {
	result, ok := validationResult(state)
	if !ok {
		return result
	}

	switch operation {
	case admissionv1.Create, admissionv1.Update:
		result.Merge(
			cpval.ValidateNodeGroupsClassReference(state, false),
			cpval.ValidateInstanceClassesEtcdDisk(state),
		)
	}

	return result
}

// validationResult reports whether the rules should run at all: a nil state is a programming
// error, and a half-migrated cluster must not be held to the new model yet.
func validationResult(state *zval.State) (cpvalapi.Result, bool) {
	if state == nil {
		return cpvalapi.ResultForNilState(), false
	}

	if cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return cpvalapi.Result{}, false
	}

	return cpvalapi.Result{}, true
}
