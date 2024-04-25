package phases

import (
	"errors"
	"fmt"

	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type (
	OnPhaseFunc[OperationPhaseDataT any] func(completedPhase OperationPhase, completedPhaseState DhctlState, completedPhaseData OperationPhaseDataT, nextPhase OperationPhase, nextPhaseCritical bool) error

	PhasedExecutionContext[OperationPhaseDataT any] interface {
		InitPipeline(stateCache dstate.Cache) error
		Finalize(stateCache dstate.Cache) error
		StartPhase(phase OperationPhase, isCritical bool, stateCache dstate.Cache) (bool, error)
		CompletePhase(stateCache dstate.Cache, completedPhaseData OperationPhaseDataT) error
		CompletePipeline(stateCache dstate.Cache) error
		SwitchPhase(phase OperationPhase, isCritical bool, stateCache dstate.Cache, completedPhaseData OperationPhaseDataT) (bool, error)
		CompletePhaseAndPipeline(stateCache dstate.Cache, completedPhaseData OperationPhaseDataT) error
		GetLastState() DhctlState
	}

	DefaultPhasedExecutionContext PhasedExecutionContext[interface{}]
	DefaultOnPhaseFunc            OnPhaseFunc[interface{}]
)

type phasedExecutionContext[OperationPhaseDataT any] struct {
	onPhaseFunc               OnPhaseFunc[OperationPhaseDataT]
	lastState                 DhctlState
	completedPhase            OperationPhase
	completedPhaseData        OperationPhaseDataT
	failedPhase               OperationPhase
	currentPhase              OperationPhase
	stopOperationCondition    bool
	pipelineCompletionCounter int
}

func NewDefaultPhasedExecutionContext(onPhaseFunc DefaultOnPhaseFunc) *phasedExecutionContext[interface{}] {
	return NewPhasedExecutionContext[interface{}](OnPhaseFunc[interface{}](onPhaseFunc))
}

func NewPhasedExecutionContext[OperationPhaseDataT any](onPhaseFunc OnPhaseFunc[OperationPhaseDataT]) *phasedExecutionContext[OperationPhaseDataT] {
	return &phasedExecutionContext[OperationPhaseDataT]{onPhaseFunc: onPhaseFunc}
}

func (pec *phasedExecutionContext[OperationPhaseDataT]) setLastState(stateCache dstate.Cache) error {
	state, err := ExtractDhctlState(stateCache)
	if err != nil {
		return fmt.Errorf("unable to extract dhctl state: %w", err)
	}
	pec.lastState = state
	return nil
}

func (pec *phasedExecutionContext[OperationPhaseDataT]) callOnPhase(completedPhase OperationPhase, completedPhaseState DhctlState, completedPhaseData OperationPhaseDataT, nextPhase OperationPhase, nextPhaseIsCritical bool, stateCache dstate.Cache) (bool, error) {
	if pec.onPhaseFunc == nil {
		return false, nil
	}

	onPhaseErr := pec.onPhaseFunc(completedPhase, completedPhaseState, completedPhaseData, nextPhase, nextPhaseIsCritical)

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

// InitPipeline initializes phasedExecutionContext before usage.
// It is not possible to use phasedExecutionContext before InitPipeline called.
func (pec *phasedExecutionContext[OperationPhaseDataT]) InitPipeline(stateCache dstate.Cache) error {
	if err := pec.setLastState(stateCache); err != nil {
		return err
	}
	pec.pipelineCompletionCounter++
	return nil
}

// Finalize supposed to always be called when errors or no errors have occured (use defer pec.Finalize() for example).
// Call Finalize in the same scope where InitPipeline has been called.
//
// It is not possible to use phasedExecutionContext after Finalize called.
func (pec *phasedExecutionContext[OperationPhaseDataT]) Finalize(stateCache dstate.Cache) error {
	if pec.stopOperationCondition {
		return nil
	}
	return pec.setLastState(stateCache)
}

// StartPhase starts a new phase of some process behind current phasedExecutionContext.
// StartPhase could be called either after InitPipeline to start first phase or after CompletePhase to start N-th phase.
func (pec *phasedExecutionContext[OperationPhaseDataT]) StartPhase(phase OperationPhase, isCritical bool, stateCache dstate.Cache) (bool, error) {
	if pec.stopOperationCondition {
		return true, nil
	}

	pec.currentPhase = phase
	return pec.callOnPhase(pec.completedPhase, pec.lastState, pec.completedPhaseData, phase, isCritical, stateCache)
}

// CompletePhase stops previously started phase and saves current snapshot of state::Cache into the phasedExecutionContext.
func (pec *phasedExecutionContext[OperationPhaseDataT]) CompletePhase(stateCache dstate.Cache, completedPhaseData OperationPhaseDataT) error {
	if pec.stopOperationCondition {
		return nil
	}
	pec.completedPhase = pec.currentPhase
	pec.completedPhaseData = completedPhaseData
	return pec.setLastState(stateCache)
}

func (pec *phasedExecutionContext[OperationPhaseDataT]) commitState(stateCache dstate.Cache) error {
	if pec.stopOperationCondition {
		return nil
	}
	return pec.setLastState(stateCache)
}

// CompletePipeline stops whole phased process execution pipeline (onPhaseFunc will be called).
// CompletePipeline or CompletePhaseAndPipeline could be called only once for a given phasedExecutionContext.
// CompletePipeline or CompletePhaseAndPipeline should be called in the same scope where InitPipeline has been called.
func (pec *phasedExecutionContext[OperationPhaseDataT]) CompletePipeline(stateCache dstate.Cache) error {
	pec.pipelineCompletionCounter--
	if pec.stopOperationCondition {
		return nil
	}
	pec.completedPhase = pec.currentPhase
	if pec.completedPhase == "" {
		return nil
	}
	if pec.pipelineCompletionCounter == 0 {
		_, err := pec.callOnPhase(pec.completedPhase, pec.lastState, pec.completedPhaseData, "", false, stateCache)
		return err
	}
	return nil
}

// SwitchPhase is a shortcut to complete current phase & start next phase in one-step.
func (pec *phasedExecutionContext[OperationPhaseDataT]) SwitchPhase(phase OperationPhase, isCritical bool, stateCache dstate.Cache, completedPhaseData OperationPhaseDataT) (bool, error) {
	if err := pec.CompletePhase(stateCache, completedPhaseData); err != nil {
		return false, err
	}
	return pec.StartPhase(phase, isCritical, stateCache)
}

// CompletePhaseAndPipeline is a shortcut to commit current phase & complete phasedExecutionContext phased process execution pipeline.
// Complete or CompletePhaseAndPipeline could be called only once for a given phasedExecutionContext.
// Complete or CompletePhaseAndPipeline should be called in the same scope where InitPipeline has been called.
func (pec *phasedExecutionContext[OperationPhaseDataT]) CompletePhaseAndPipeline(stateCache dstate.Cache, completedPhaseData OperationPhaseDataT) error {
	if err := pec.CompletePhase(stateCache, completedPhaseData); err != nil {
		return err
	}
	return pec.CompletePipeline(stateCache)
}

// GetLastState gets last committed state from phasedExecutionContext.
func (pec *phasedExecutionContext[OperationPhaseDataT]) GetLastState() DhctlState {
	return pec.lastState
}
