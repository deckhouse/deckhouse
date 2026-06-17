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

package preflight

import (
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"

	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation/meta"
)

// ValidatePreflight checks resources required before cluster bootstrap or converge.
func ValidatePreflight(state *cpval.State) cpval.Result {
	if state == nil {
		return cpval.ResultForNilState()
	}

	result := cpval.Result{}

	if cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return result
	}

	result.Merge(cpval.ValidateModuleConfig(state))
	result.Merge(cpval.ValidateCredentialSecretPresence(state))
	result.Merge(cpval.ValidateCredentialSecretContent(state, dvpmeta.AllowedCredentialAuthSchemes))
	result.Merge(cpval.ValidateProviderClusterConfigKubeconfig(state, dvpmeta.ProviderClusterConfigKubeconfigPath))
	result.Merge(cpval.ValidateMasterNodeGroupClassReference(state))
	result.Merge(cpval.ValidateMasterInstanceClassReference(state))
	result.Merge(cpval.ValidateInstanceClassesEtcdDisk(state))

	return result
}
