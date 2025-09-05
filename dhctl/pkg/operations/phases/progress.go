// Copyright 2025 Flant JSC
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
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"
)

type PhaseWithSubPhases struct {
	Phase     OperationPhase      `json:"phase"`
	SubPhases []OperationSubPhase `json:"subPhases,omitempty"`
}

var BootstrapPhases = []PhaseWithSubPhases{
	{Phase: BaseInfraPhase},
	{Phase: RegistryPackagesProxyPhase},
	{Phase: ExecuteBashibleBundlePhase},
	{
		Phase: InstallDeckhousePhase,
		SubPhases: []OperationSubPhase{
			InstallDeckhouseSubPhaseConnect,
			InstallDeckhouseSubPhaseInstall,
			InstallDeckhouseSubPhaseWait,
		},
	},
	{Phase: InstallAdditionalMastersAndStaticNodes},
	{Phase: CreateResourcesPhase},
	{Phase: ExecPostBootstrapPhase},
	{Phase: FinalizationPhase},
}

var ConvergePhases = []PhaseWithSubPhases{
	{Phase: BaseInfraPhase},
	{Phase: InstallDeckhousePhase},
	{Phase: AllNodesPhase},
	{Phase: ScaleToMultiMasterPhase},
	{Phase: DeckhouseConfigurationPhase},
}

var CheckPhases = []PhaseWithSubPhases{ // currently no phases for this operation
	{Phase: OperationPhase(OperationCheck)},
}

var DestroyPhases = []PhaseWithSubPhases{
	{Phase: DeleteResourcesPhase},
	{Phase: AllNodesPhase},
	{Phase: BaseInfraPhase},
}

var CommanderAttachPhases = []PhaseWithSubPhases{
	{Phase: CommanderAttachScanPhase},
	{Phase: CommanderAttachCheckPhase},
	{Phase: CommanderAttachCheckPhase},
}

var CommanderDetachPhases = []PhaseWithSubPhases{ // currently no phases for this operation
	{Phase: OperationPhase(OperationCommanderDetach)},
}

var operationPhases = map[Operation][]PhaseWithSubPhases{
	OperationBootstrap:       BootstrapPhases,
	OperationConverge:        ConvergePhases,
	OperationCheck:           CheckPhases,
	OperationDestroy:         DestroyPhases,
	OperationCommanderAttach: CommanderAttachPhases,
	OperationCommanderDetach: CommanderDetachPhases,
}

type OnProgressFunc func(Progress) error

type ProgressTracker struct {
	progress Progress
	mx       sync.Mutex

	onProgressFunc func(Progress) error
}

type Progress struct {
	Operation Operation `json:"operation"`
	Progress  float64   `json:"progress"`

	CompletedPhase OperationPhase `json:"completedPhase,omitempty"`
	CurrentPhase   OperationPhase `json:"currentPhase,omitempty"`
	NextPhase      OperationPhase `json:"nextPhase,omitempty"`

	CompletedSubPhase OperationSubPhase `json:"completedSubPhase,omitempty"`
	CurrentSubPhase   OperationSubPhase `json:"currentSubPhase,omitempty"`
	NextSubPhase      OperationSubPhase `json:"nextSubPhase,omitempty"`

	Phases []PhaseWithSubPhases `json:"phases"`
}

func NewProgressTracker(operation Operation, onProgressFunc func(Progress) error) *ProgressTracker {
	return &ProgressTracker{
		progress:       Progress{Operation: operation, Progress: 0, Phases: operationPhases[operation]},
		onProgressFunc: onProgressFunc,
	}
}

func (p *ProgressTracker) LastCompletedPhase(
	completedPhase, nextPhase OperationPhase,
) OperationPhase {
	if completedPhase != "" {
		return completedPhase
	}

	if nextPhase == "" {
		return completedPhase
	}

	phases, ok := operationPhases[p.progress.Operation]
	if !ok {
		return completedPhase
	}

	nextPhaseIndex := slices.IndexFunc(phases, func(phases PhaseWithSubPhases) bool {
		return phases.Phase == nextPhase
	})

	return nOrEmpty(phases, nextPhaseIndex-1).Phase
}

func (p *ProgressTracker) Progress(completedPhase OperationPhase, completedSubPhase OperationSubPhase) error {
	if p.onProgressFunc == nil {
		return nil
	}

	p.mx.Lock()

	var progress Progress
	if completedPhase == "" && completedSubPhase == "" || completedPhase != "" {
		progress = calculatePhaseProgress(p.progress, completedPhase)
	} else {
		progress = calculateSubPhaseProgress(p.progress, completedSubPhase)
	}

	p.progress = progress
	p.mx.Unlock()

	return p.onProgressFunc(progress)
}

func calculatePhaseProgress(p Progress, completedPhase OperationPhase) Progress {
	progress := Progress{
		Operation:       p.Operation,
		Phases:          p.Phases,
		Progress:        max(float64(0), p.Progress),
		CompletedPhase:  completedPhase,
		CurrentPhase:    "",
		CurrentSubPhase: "",
	}

	// return progress as is if there is no known phases for given operation
	phases, ok := operationPhases[p.Operation]
	if !ok {
		return progress
	}

	// return 0 progress if no completed phase
	if completedPhase == "" {
		firstPhase := nOrEmpty(phases, 0)

		progress.CurrentPhase = firstPhase.Phase
		progress.NextPhase = nOrEmpty(phases, 1).Phase
		progress.CurrentSubPhase = nOrEmpty(firstPhase.SubPhases, 0)
		return progress
	}

	completedPhaseIndex := slices.IndexFunc(phases, func(phases PhaseWithSubPhases) bool {
		return phases.Phase == completedPhase
	})
	if completedPhaseIndex == -1 {
		return progress
	}
	currentPhaseIndex := completedPhaseIndex + 1

	// get current phase and first sub phase if exists
	curPhase := nOrEmpty(phases, currentPhaseIndex)
	nextPhase := nOrEmpty(phases, currentPhaseIndex+1)

	progress.CurrentPhase = curPhase.Phase
	progress.NextPhase = nextPhase.Phase
	progress.CurrentSubPhase = nOrEmpty(curPhase.SubPhases, 0)
	progress.NextSubPhase = nOrEmpty(curPhase.SubPhases, 1)

	progress.Progress = max(float64(currentPhaseIndex)/float64(len(phases)), p.Progress)

	return progress
}

func calculateSubPhaseProgress(p Progress, completedSubPhase OperationSubPhase) Progress {
	progress := Progress{
		Operation:         p.Operation,
		Phases:            p.Phases,
		Progress:          max(float64(0), p.Progress),
		CompletedPhase:    p.CompletedPhase,
		CurrentPhase:      p.CurrentPhase,
		NextPhase:         p.NextPhase,
		CompletedSubPhase: completedSubPhase,
		CurrentSubPhase:   "",
		NextSubPhase:      "",
	}

	// return progress as is if there is no known phases for given operation
	phases, ok := operationPhases[p.Operation]
	if !ok {
		return progress
	}

	var currentPhase PhaseWithSubPhases

	for _, phase := range phases {
		if phase.Phase == p.CurrentPhase {
			currentPhase = phase
		}
	}

	completedSubPhaseIndex := slices.Index(currentPhase.SubPhases, completedSubPhase)
	if completedSubPhaseIndex == -1 {
		return progress
	}
	currentSubPhaseIndex := completedSubPhaseIndex + 1

	progress.CurrentSubPhase = nOrEmpty(currentPhase.SubPhases, currentSubPhaseIndex)
	progress.NextSubPhase = nOrEmpty(currentPhase.SubPhases, currentSubPhaseIndex+1)

	progress.Progress += (1 / float64(len(phases))) / float64(len(currentPhase.SubPhases))

	return progress
}

func WriteProgress(path string) OnProgressFunc {
	return func(progress Progress) error {
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_SYNC, 0644)
		if err != nil {
			return err
		}
		defer file.Close()

		jsonData, err := json.Marshal(progress)
		if err != nil {
			return fmt.Errorf("marshalling progress: %w", err)
		}

		if _, err = file.WriteString(string(jsonData) + "\n"); err != nil {
			return fmt.Errorf("writing progress: %w", err)
		}

		return nil
	}
}

func nOrEmpty[T any](s []T, n int) T {
	if n >= 0 && len(s) > n {
		return s[n]
	}

	var zero T
	return zero
}
