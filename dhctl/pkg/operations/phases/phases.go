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
	RegistryPackagesProxyPhase             OperationPhase = "RegistryPackagesProxyBundle"
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
	onPhaseFunc               OnPhaseFunc
	lastState                 DhctlState
	completedPhase            OperationPhase
	failedPhase               OperationPhase
	currentPhase              OperationPhase
	stopOperationCondition    bool
	pipelineCompletionCounter int
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

func (pec *PhasedExecutionContext) callOnPhase(completedPhase OperationPhase, completedPhaseState DhctlState, nextPhase OperationPhase, nextPhaseIsCritical bool, stateCache dstate.Cache) (bool, error) {
	if pec.onPhaseFunc == nil {
		return false, nil
	}

	onPhaseErr := pec.onPhaseFunc(completedPhase, completedPhaseState, nextPhase, nextPhaseIsCritical)

	if onPhaseErr != nil {
		if err := pec.setLastState(stateCache); err != nil {
			return false, err
		}

		pec.stopOperationCondition = true

		if errors.Is(onPhaseErr, StopOperationCondition) {
			return true, nil
		} else {
			return false, onPhaseErr
		}
	}

	return false, nil
}

// InitPipeline initializes PhasedExecutionContext before usage.
// It is not possible to use PhasedExecutionContext before InitPipeline called.
func (pec *PhasedExecutionContext) InitPipeline(stateCache dstate.Cache) error {
	if err := pec.setLastState(stateCache); err != nil {
		return err
	}
	pec.pipelineCompletionCounter++
	return nil
}

// Finalize supposed to always be called when errors or no errors have occured (use defer pec.Finalize() for example).
// Call Finalize in the same scope where InitPipeline has been called.
//
// It is not possible to use PhasedExecutionContext after Finalize called.
func (pec *PhasedExecutionContext) Finalize(stateCache dstate.Cache) error {
	if pec.stopOperationCondition {
		return nil
	}
	return pec.setLastState(stateCache)
}

// StartPhase starts a new phase of some process behind current PhasedExecutionContext.
// StartPhase could be called either after InitPipeline to start first phase or after CompletePhase to start N-th phase.
func (pec *PhasedExecutionContext) StartPhase(phase OperationPhase, isCritical bool, stateCache dstate.Cache) (bool, error) {
	if pec.stopOperationCondition {
		return true, nil
	}

	pec.currentPhase = phase
	return pec.callOnPhase(pec.completedPhase, pec.lastState, phase, isCritical, stateCache)
}

// CompletePhase stops previously started phase and saves current snapshot of state::Cache into the PhasedExecutionContext.
func (pec *PhasedExecutionContext) CompletePhase(stateCache dstate.Cache) error {
	if pec.stopOperationCondition {
		return nil
	}
	pec.completedPhase = pec.currentPhase
	return pec.setLastState(stateCache)
}

func (pec *PhasedExecutionContext) commitState(stateCache dstate.Cache) error {
	if pec.stopOperationCondition {
		return nil
	}
	return pec.setLastState(stateCache)
}

// CompletePipeline stops whole phased process execution pipeline (onPhaseFunc will be called).
// CompletePipeline or CompletePhaseAndPipeline could be called only once for a given PhasedExecutionContext.
// CompletePipeline or CompletePhaseAndPipeline should be called in the same scope where InitPipeline has been called.
func (pec *PhasedExecutionContext) CompletePipeline(stateCache dstate.Cache) error {
	pec.pipelineCompletionCounter--
	if pec.stopOperationCondition {
		return nil
	}
	pec.completedPhase = pec.currentPhase
	if pec.completedPhase == "" {
		return nil
	}
	if pec.pipelineCompletionCounter == 0 {
		_, err := pec.callOnPhase(pec.completedPhase, pec.lastState, "", false, stateCache)
		return err
	}
	return nil
}

// SwitchPhase is a shortcut to complete current phase & start next phase in one-step.
func (pec *PhasedExecutionContext) SwitchPhase(phase OperationPhase, isCritical bool, stateCache dstate.Cache) (bool, error) {
	if err := pec.CompletePhase(stateCache); err != nil {
		return false, err
	}
	return pec.StartPhase(phase, isCritical, stateCache)
}

// CompletePhaseAndPipeline is a shortcut to commit current phase & complete PhasedExecutionContext phased process execution pipeline.
// Complete or CompletePhaseAndPipeline could be called only once for a given PhasedExecutionContext.
// Complete or CompletePhaseAndPipeline should be called in the same scope where InitPipeline has been called.
func (pec *PhasedExecutionContext) CompletePhaseAndPipeline(stateCache dstate.Cache) error {
	if err := pec.CompletePhase(stateCache); err != nil {
		return err
	}
	return pec.CompletePipeline(stateCache)
}

// GetLastState gets last committed state from PhasedExecutionContext.
func (pec *PhasedExecutionContext) GetLastState() DhctlState {
	return pec.lastState
}
