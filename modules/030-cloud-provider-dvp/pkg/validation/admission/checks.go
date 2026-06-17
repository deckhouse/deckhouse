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

	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation/meta"
)

// ValidateModuleConfig validates ModuleConfig admission requests.
func ValidateModuleConfig(state *cpval.State, _ admissionv1.Operation) cpval.Result {
	result, ok := validationResult(state)
	if !ok {
		return result
	}

	result.Merge(cpval.ValidateModuleConfig(state))

	return result
}

// ValidateCredentialSecret validates credential Secret admission requests.
func ValidateCredentialSecret(state *cpval.State, _ admissionv1.Operation) cpval.Result {
	result, ok := validationResult(state)
	if !ok {
		return result
	}

	result.Merge(cpval.ValidateCredentialSecretContent(state, dvpmeta.AllowedCredentialAuthSchemes))

	return result
}

// ValidateInstanceClass validates InstanceClass admission requests.
// deletedClass must be set when operation is Delete.
func ValidateInstanceClass(state *cpval.State, operation admissionv1.Operation, deletedClass *cpapi.InstanceClass) cpval.Result {
	result, ok := validationResult(state)
	if !ok {
		return result
	}

	result.Merge(cpval.ValidateMasterInstanceClassReference(state))
	result.Merge(cpval.ValidateInstanceClassesEtcdDisk(state))

	if operation == admissionv1.Delete && deletedClass != nil {
		result.Merge(cpval.ValidateInstanceClassDelete(state, deletedClass.Name, deletedClass))
	}

	return result
}

// ValidateNodeGroup validates NodeGroup admission requests.
func ValidateNodeGroup(state *cpval.State, _ admissionv1.Operation) cpval.Result {
	result, ok := validationResult(state)
	if !ok {
		return result
	}

	result.Merge(cpval.ValidateInstanceClassesEtcdDisk(state))

	return result
}

func validationResult(state *cpval.State) (cpval.Result, bool) {
	if state == nil {
		return cpval.ResultForNilState(), false
	}

	if cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return cpval.Result{}, false
	}

	return cpval.Result{}, true
}
