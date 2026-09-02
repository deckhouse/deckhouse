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

package admission

import (
	admissionv1 "k8s.io/api/admission/v1"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	dvpicv1alpha1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/instanceclass/v1alpha1"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
)

// ValidateCredentialSecret validates credential Secret admission requests.
func ValidateCredentialSecret(state *dvpval.State, operation admissionv1.Operation) cpvalapi.Result {
	result, ok := validationResult(state)
	if !ok {
		return result
	}

	switch operation {
	case admissionv1.Create, admissionv1.Update:
		result.Merge(cpval.ValidateCredentialSecretContent(state, cpapi.CredentialSecretName, dvpval.CredentialsValidator))
	}

	return result
}

// ValidateInstanceClass validates InstanceClass admission requests.
// deletedClass must be set when operation is Delete.
func ValidateInstanceClass(
	state *dvpval.State,
	operation admissionv1.Operation,
	deletedClass *dvpicv1alpha1.DVPInstanceClass,
) cpvalapi.Result {
	result, ok := validationResult(state)
	if !ok {
		return result
	}

	switch operation {
	case admissionv1.Create, admissionv1.Update:
		result.Merge(cpval.ValidateInstanceClassesEtcdDisk(state))
	case admissionv1.Delete:
		result.Merge(cpval.ValidateInstanceClassDeletion(state, deletedClass))
	}

	return result
}

// ValidateNodeGroup validates NodeGroup admission requests.
func ValidateNodeGroup(state *dvpval.State, operation admissionv1.Operation) cpvalapi.Result {
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

func validationResult(state *dvpval.State) (cpvalapi.Result, bool) {
	if state == nil {
		return cpvalapi.ResultForNilState(), false
	}

	if cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return cpvalapi.Result{}, false
	}

	return cpvalapi.Result{}, true
}
