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
	"fmt"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"

	dvppccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/pcc/v1"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
)

const (
	CodePCCInvalidKubeconfigSecret = "pcc_invalid_kubeconfig_secret"
)

// ValidatePreflight checks resources required before cluster bootstrap or converge.
func ValidatePreflight(state *dvpval.State) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}
	if state.HasProviderClusterConfig() {
		result.Merge(
			validateKubeconfig(state.ProviderClusterConfig),
		)
	}

	if cpapi.ShouldSkipNewModelValidation(state.MigrationStatus) {
		return result
	}

	result.Merge(
		cpval.ValidateModuleConfig(state),
		cpval.ValidateCredentialSecretPresence(state, cpapi.CredentialSecretName),
		cpval.ValidateCredentialSecretContent(state, cpapi.CredentialSecretName, dvpval.CredentialsValidator),
		cpval.ValidateMasterNodeGroupPresence(state),
		cpval.ValidateNodeGroupsClassReference(state, true),
		cpval.ValidateInstanceClassesEtcdDisk(state),
	)

	return result
}

func validateKubeconfig(pcc *dvppccv1.DVPProviderClusterConfiguration) cpvalapi.Result {
	result := cpvalapi.Result{}

	if err := cpval.ValidateKubeconfigBase64(pcc.Provider.KubeconfigDataBase64); err != nil {
		result.AddError(
			"ProviderClusterConfiguration.provider.kubeconfigDataBase64",
			CodePCCInvalidKubeconfigSecret,
			"masked",
			fmt.Sprintf("invalid kubeconfig: %v", err),
		)
	}

	return result
}
