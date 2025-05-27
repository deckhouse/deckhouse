/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"fmt"

	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

const (
	ConditionTypeReady                   = "Ready"
	ConditionTypeNodeServices            = "NodeServicesReady"
	ConditionTypeNodeServicesCleanup     = "CleanupNodeServices"
	ConditionTypeInClusterProxy          = "InClusterProxyReady"
	ConditionTypeInClusterProxyCleanup   = "CleanupInClusterProxy"
	ConditionTypeBashibleTransitionStage = "TransitionContainerdConfigReady"
	ConditionTypeBashibleFinalStage      = "FinalContainerdConfigReady"

	ConditionReasonReady      = "Ready"
	ConditionReasonProcessing = "Processing"
	ConditionReasonError      = "Error"
)

var supportedConditions = map[string]struct{}{
	ConditionTypeReady:                   {},
	ConditionTypeNodeServices:            {},
	ConditionTypeNodeServicesCleanup:     {},
	ConditionTypeInClusterProxy:          {},
	ConditionTypeInClusterProxyCleanup:   {},
	ConditionTypeBashibleTransitionStage: {},
	ConditionTypeBashibleFinalStage:      {},
}

var _ error = ErrTransitionNotSupported{}

type ErrTransitionNotSupported struct {
	From, To registry_const.ModeType
}

func (err ErrTransitionNotSupported) Error() string {
	return fmt.Sprintf("mode transition from %v to %v not supported", err.From, err.To)
}
