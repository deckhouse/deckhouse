// Copyright 2023 Flant JSC
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

package phases

import (
	"errors"
	"fmt"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type OnPhaseFunc func(completedPhase OperationPhase, completedPhaseState DhctlState, nextPhase OperationPhase, nextPhaseCritical bool) error

type OperationPhase string

const (
	BaseInfraPhase                         OperationPhase = "BaseInfra"
	ExecuteBashibleBundlePhase             OperationPhase = "ExecuteBashibleBundle"
	InstallDeckhousePhase                  OperationPhase = "InstallDeckhouse"
	CreateResourcesPhase                   OperationPhase = "CreateResources"
	InstallAdditionalMastersAndStaticNodes OperationPhase = "InstallAdditionalMastersAndStaticNodes"
	DeleteResourcesPhase                   OperationPhase = "DeleteResources"
	ExecPostBootstrapPhase                 OperationPhase = "ExecPostBootstrap"
	AllNodesPhase                          OperationPhase = "AllNodes"
	FinalizationPhase                      OperationPhase = "Finalization"
)

var (
	StopOperationCondition = errors.New("StopOperationCondition")
)

type PhasedExecutionContext struct {
	onPhaseFunc            OnPhaseFunc
	lastState              DhctlState
	completedPhase         OperationPhase
	failedPhase            OperationPhase
	currentPhase           OperationPhase
	stopOperationCondition bool
}

func NewPhasedExecutionContext(onPhaseFunc OnPhaseFunc) *PhasedExecutionContext {
	return &PhasedExecutionContext{onPhaseFunc: onPhaseFunc}
}

func (pec *PhasedExecutionContext) setLastState(stateCache dstate.Cache) error {
	state, err := ExtractDhctlState(stateCache)
	if err != nil {
		return fmt.Errorf("unable to extract dhctl state: %w", err)
	}
	pec.lastState = state
	return nil
}

func (pec *PhasedExecutionContext) callOnPhase(completedPhase OperationPhase, completedPhaseState DhctlState, nextPhase OperationPhase, nextPhaseIsCritical bool) (bool, error) {
	if pec.onPhaseFunc == nil {
		return false, nil
	}
	err := pec.onPhaseFunc(completedPhase, completedPhaseState, nextPhase, nextPhaseIsCritical)
	if errors.Is(err, StopOperationCondition) {
		return true, nil
	} else {
		return false, err
	}
}

func (pec *PhasedExecutionContext) Init(stateCache dstate.Cache) error {
	return pec.setLastState(stateCache)
}

func (pec *PhasedExecutionContext) Finalize(stateCache dstate.Cache) error {
	return pec.CommitState(stateCache)
}

func (pec *PhasedExecutionContext) StartPhase(phase OperationPhase, isCritical bool) (bool, error) {
	if pec.stopOperationCondition {
		return false, nil
	}

	pec.currentPhase = phase
	return pec.callOnPhase(pec.completedPhase, pec.lastState, phase, isCritical)
}

func (pec *PhasedExecutionContext) CommitState(stateCache dstate.Cache) error {
	if pec.stopOperationCondition {
		return nil
	}
	return pec.setLastState(stateCache)
}

func (pec *PhasedExecutionContext) Complete() error {
	if pec.stopOperationCondition {
		return nil
	}
	pec.completedPhase = pec.currentPhase
	if pec.completedPhase == "" {
		return nil
	}
	_, err := pec.callOnPhase(pec.completedPhase, pec.lastState, "", false)
	return err
}

// SwitchPhase — is a shortcut to call stop-current-phase & start-next-phase
func (pec *PhasedExecutionContext) SwitchPhase(phase OperationPhase, isCritical bool, stateCache dstate.Cache) (bool, error) {
	if err := pec.CommitState(stateCache); err != nil {
		return false, err
	}
	return pec.StartPhase(phase, isCritical)
}

// StopAndFinalize — is a shortcut to call stop-current-phase & finalize execution
func (pec *PhasedExecutionContext) CommitAndComplete(stateCache dstate.Cache) error {
	if err := pec.CommitState(stateCache); err != nil {
		return err
	}
	return pec.Complete()
}

func (pec *PhasedExecutionContext) GetLastState() DhctlState {
	return pec.lastState
}
