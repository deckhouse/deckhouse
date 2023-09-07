package operations

import (
	"errors"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type DhctlState map[string][]byte

type OnPhaseFunc func(completedPhase OperationPhase, completedPhaseState DhctlState, nextPhase OperationPhase, nextPhaseCritical bool) error

type OperationPhase string

const (
	BaseInfraPhase             OperationPhase = "BaseInfra"
	ExecuteBashibleBundlePhase OperationPhase = "ExecuteBashibleBundle"
	InstallDeckhousePhase      OperationPhase = "InstallDeckhouse"
	CreateResourcesPhase       OperationPhase = "CreateResources"
	ExecPostBootstrapPhase     OperationPhase = "ExecPostBootstrap"
	FinalizationPhase          OperationPhase = "Finalization"
)

var (
	StopOperationCondition = errors.New("StopOperationCondition")
)

func ExtractDhctlState(stateCache state.Cache) (res DhctlState, err error) {
	err = stateCache.Iterate(func(k string, v []byte) error {
		if res == nil {
			res = make(map[string][]byte)
		}
		res[k] = v
		return nil
	})
	return
}
