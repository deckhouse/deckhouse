/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package orchestrator

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

const (
	ConditionTypeReady                          = "Ready"
	ConditionTypeNodeServices                   = "NodeServicesReady"
	ConditionTypeNodeServicesCleanup            = "CleanupNodeServices"
	ConditionTypeInClusterProxy                 = "InClusterProxyReady"
	ConditionTypeInClusterProxyCleanup          = "CleanupInClusterProxy"
	ConditionTypeBashiblePreflightCheck         = "ContainerdConfigPreflightReady"
	ConditionTypeBashibleTransitionStage        = "TransitionContainerdConfigReady"
	ConditionTypeBashibleFinalStage             = "FinalContainerdConfigReady"
	ConditionTypeDeckhouseRegistrySwitch        = "DeckhouseRegistrySwitchReady"
	ConditionTypeRegistryContainsRequiredImages = "RegistryContainsRequiredImages"
	ConditionTypeErrTransitionNotSupported      = "ErrTransitionNotSupported"

	ConditionReasonReady      = "Ready"
	ConditionReasonProcessing = "Processing"
	ConditionReasonError      = "Error"
)

var supportedConditions = map[string]struct{}{
	ConditionTypeReady:                          {},
	ConditionTypeNodeServices:                   {},
	ConditionTypeNodeServicesCleanup:            {},
	ConditionTypeInClusterProxy:                 {},
	ConditionTypeInClusterProxyCleanup:          {},
	ConditionTypeBashiblePreflightCheck:         {},
	ConditionTypeBashibleTransitionStage:        {},
	ConditionTypeBashibleFinalStage:             {},
	ConditionTypeDeckhouseRegistrySwitch:        {},
	ConditionTypeRegistryContainsRequiredImages: {},
	ConditionTypeErrTransitionNotSupported:      {},
}

var _ error = ErrTransitionNotSupported{}

type ErrTransitionNotSupported struct {
	From, To registry_const.ModeType
}

func (err ErrTransitionNotSupported) Error() string {
	return fmt.Sprintf("mode transition from %v to %v not supported", err.From, err.To)
}

func (err ErrTransitionNotSupported) Condition() metav1.Condition {
	return metav1.Condition{
		Type:    ConditionTypeErrTransitionNotSupported,
		Status:  metav1.ConditionTrue,
		Reason:  ConditionReasonError,
		Message: err.Error(),
	}
}
