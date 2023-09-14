package phases

import (
	"errors"
	"fmt"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type OnPhaseFunc func(completedPhase OperationPhase, completedPhaseState DhctlState, nextPhase OperationPhase, nextPhaseCritical bool) error

type OperationPhase string

const (
	BaseInfraPhase             OperationPhase = "BaseInfra"
	ExecuteBashibleBundlePhase OperationPhase = "ExecuteBashibleBundle"
	InstallDeckhousePhase      OperationPhase = "InstallDeckhouse"
	CreateResourcesPhase       OperationPhase = "CreateResources"
	DeleteResourcesPhase       OperationPhase = "DeleteResources"
	ExecPostBootstrapPhase     OperationPhase = "ExecPostBootstrap"
	AllNodesPhase              OperationPhase = "AllNodes"
	FinalizationPhase          OperationPhase = "Finalization"
)

var (
	StopOperationCondition = errors.New("StopOperationCondition")
)

type PhasedExecutionContext struct {
	onPhaseFunc            OnPhaseFunc
	lastState              DhctlState
	completedPhase         OperationPhase
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

func (pec *PhasedExecutionContext) Start(phase OperationPhase, isCritical bool) (bool, error) {
	if pec.stopOperationCondition {
		return false, nil
	}

	pec.currentPhase = phase
	return pec.callOnPhase(pec.completedPhase, pec.lastState, phase, isCritical)
}

func (pec *PhasedExecutionContext) Stop(stateCache dstate.Cache) error {
	if pec.stopOperationCondition {
		return nil
	}

	pec.completedPhase = pec.currentPhase
	if stateCache != nil {
		return pec.setLastState(stateCache)
	}
	return nil
}

func (pec *PhasedExecutionContext) Finalize() error {
	if pec.stopOperationCondition {
		return nil
	}
	if pec.currentPhase != pec.completedPhase {
		panic(fmt.Sprintf("unexpected condition currentPhase(%s) != completedPhase(%s), call Stop before Finalize", pec.currentPhase, pec.completedPhase))
	}
	if pec.completedPhase == "" {
		return nil
	}
	_, err := pec.callOnPhase(pec.completedPhase, pec.lastState, "", false)
	return err
}

// SwitchPhase — is a shortcut to call stop-current-phase & start-next-phase
func (pec *PhasedExecutionContext) SwitchPhase(stateCache dstate.Cache, phase OperationPhase, isCritical bool) (bool, error) {
	if err := pec.Stop(stateCache); err != nil {
		return false, err
	}
	return pec.Start(phase, isCritical)
}

// StopAndFinalize — is a shortcut to call stop-current-phase & finalize execution
func (pec *PhasedExecutionContext) StopAndFinalize(stateCache dstate.Cache) error {
	if err := pec.Stop(stateCache); err != nil {
		return err
	}
	return pec.Finalize()
}
