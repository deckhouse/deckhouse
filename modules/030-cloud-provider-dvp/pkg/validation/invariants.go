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
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
)

// ValidateInvariants runs DVP validation rules for the current cluster state.
func ValidateInvariants(state *cpval.State) cpval.Result {
	if state == nil {
		return cpval.ResultForNilState()
	}

	result := cpval.Result{}

	if cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return result
	}

	result.Merge(cpval.ValidateModuleConfig(state))
	result.Merge(cpval.ValidateCredentialSecretContent(state, AllowedCredentialAuthSchemes))
	result.Merge(cpval.ValidateInstanceClassEtcdDiskAttachment(state))

	return result
}
