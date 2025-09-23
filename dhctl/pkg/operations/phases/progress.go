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
	"math"
	"os"
	"slices"
	"sync"

	"k8s.io/utils/ptr"
)

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

func (p Progress) Clone() Progress {
	clonedPhases := make([]PhaseWithSubPhases, len(p.Phases))
	for i, phase := range p.Phases {
		clonedPhase := PhaseWithSubPhases{
			Phase:     phase.Phase,
			SubPhases: slices.Clone(phase.SubPhases),
		}

		if phase.Action != nil {
			clonedAction := *phase.Action
			clonedPhase.Action = &clonedAction
		}

		clonedPhases[i] = clonedPhase
	}

	return Progress{
		Operation:         p.Operation,
		Progress:          p.Progress,
		CompletedPhase:    p.CompletedPhase,
		CurrentPhase:      p.CurrentPhase,
		NextPhase:         p.NextPhase,
		CompletedSubPhase: p.CompletedSubPhase,
		CurrentSubPhase:   p.CurrentSubPhase,
		NextSubPhase:      p.NextSubPhase,
		Phases:            clonedPhases,
	}
}

type ProgressOpts struct {
	Action PhaseAction
}

func NewProgressTracker(operation Operation, onProgressFunc func(Progress) error) *ProgressTracker {
	phases, _ := operationPhases(operation)

	return &ProgressTracker{
		progress:       Progress{Operation: operation, Progress: 0, Phases: phases},
		onProgressFunc: onProgressFunc,
	}
}

func (p *ProgressTracker) FindLastCompletedPhase(
	completedPhase, nextPhase OperationPhase,
) (OperationPhase, bool) {
	if completedPhase != "" {
		return completedPhase, false
	}

	if nextPhase == "" {
		return completedPhase, false
	}

	phases, ok := operationPhases(p.progress.Operation)
	if !ok {
		return completedPhase, false
	}

	nextPhaseIndex := slices.IndexFunc(phases, func(phases PhaseWithSubPhases) bool {
		return phases.Phase == nextPhase
	})

	return nOrEmpty(phases, nextPhaseIndex-1).Phase, true
}

func (p *ProgressTracker) Progress(
	completedPhase OperationPhase, completedSubPhase OperationSubPhase, opts ProgressOpts,
) error {
	if p.onProgressFunc == nil {
		return nil
	}

	p.mx.Lock()

	var progress Progress
	if completedPhase == "" && completedSubPhase == "" || completedPhase != "" {
		progress = calculatePhaseProgress(p.progress, completedPhase, opts)
	} else {
		progress = calculateSubPhaseProgress(p.progress, completedSubPhase, opts)
	}

	p.progress = progress
	p.mx.Unlock()

	return p.onProgressFunc(progress.Clone())
}

func (p *ProgressTracker) Complete(lastCompletedPhase OperationPhase) error {
	if p.onProgressFunc == nil {
		return nil
	}

	p.mx.Lock()

	const epsilon = 1e-9
	if math.Abs(p.progress.Progress-1.0) < epsilon {
		p.mx.Unlock()
		return nil
	}

	lastCompletedPhaseIndex := slices.IndexFunc(p.progress.Phases, func(phases PhaseWithSubPhases) bool {
		return phases.Phase == lastCompletedPhase
	})

	for i := range p.progress.Phases {
		if p.progress.Phases[i].Action != nil {
			continue
		}

		if i <= lastCompletedPhaseIndex {
			p.progress.Phases[i].Action = ptr.To(PhaseActionDefault)
			continue
		}
		p.progress.Phases[i].Action = ptr.To(PhaseActionSkip)
	}

	p.progress.CompletedPhase = nOrEmpty(p.progress.Phases, len(p.progress.Phases)-1).Phase
	p.progress.CurrentPhase = ""
	p.progress.NextPhase = ""
	p.progress.Progress = 1.0

	p.mx.Unlock()

	return p.onProgressFunc(p.progress.Clone())
}

func calculatePhaseProgress(p Progress, completedPhase OperationPhase, opts ProgressOpts) Progress {
	progress := Progress{
		Operation:       p.Operation,
		Phases:          p.Phases,
		Progress:        max(float64(0), p.Progress),
		CompletedPhase:  completedPhase,
		CurrentPhase:    "",
		CurrentSubPhase: "",
	}

	// return progress as is if there is no known phases for given operation
	phases, ok := operationPhases(p.Operation)
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

	// iterate over all previous phases, if action was nil, set action
	// if current action is skip, then skip all previous nil actions
	for i := 0; i <= completedPhaseIndex; i++ {
		if p.Phases[i].Action == nil {
			p.Phases[i].Action = &opts.Action
		}
	}

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

func calculateSubPhaseProgress(p Progress, completedSubPhase OperationSubPhase, _ ProgressOpts) Progress {
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
	phases, ok := operationPhases(p.Operation)
	if !ok {
		return progress
	}

	var currentPhase PhaseWithSubPhases

	for _, phase := range phases {
		if phase.Phase == p.CurrentPhase {
			currentPhase = phase
			break
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

		err = file.Sync()
		if err != nil {
			return fmt.Errorf("syncing progress: %w", err)
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
