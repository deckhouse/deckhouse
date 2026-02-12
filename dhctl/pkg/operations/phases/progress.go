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
	clusterConfig  ClusterConfig
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
	Action ProgressAction
}

func NewProgressTracker(operation Operation, onProgressFunc func(Progress) error) *ProgressTracker {
	phases, _ := operationPhases(operation, phasesOpts{})

	return &ProgressTracker{
		progress:       Progress{Operation: operation, Progress: 0, Phases: phases},
		onProgressFunc: onProgressFunc,
	}
}

// SetClusterConfig sets the cluster config and syncs the phase list immediately.
// Call as soon as meta config is parsed, before any phase is reported.
func (p *ProgressTracker) SetClusterConfig(cfg ClusterConfig) {
	p.mx.Lock()
	defer p.mx.Unlock()

	if p.clusterConfig == cfg {
		return
	}

	phases, _ := operationPhases(p.progress.Operation, phasesOpts{clusterConfig: cfg})
	p.clusterConfig = cfg
	p.progress.Phases = phases
}

// FindLastCompletedPhase returns the last completed phase when completedPhase is empty
// It determines the phase preceding nextPhase, indicating if phases were skipped
func (p *ProgressTracker) FindLastCompletedPhase(completedPhase, nextPhase OperationPhase) (OperationPhase, bool) {
	if completedPhase != "" {
		return completedPhase, false
	}

	if nextPhase == "" {
		return completedPhase, false
	}

	p.mx.Lock()
	defer p.mx.Unlock()

	nextPhaseIndex := slices.IndexFunc(p.progress.Phases, func(phases PhaseWithSubPhases) bool {
		return phases.Phase == nextPhase
	})

	return nOrEmpty(p.progress.Phases, nextPhaseIndex-1).Phase, true
}

// Progress updates the progress state with a completed phase or subphase.
// currentPhase is the phase we're in now; when non-empty it is used as-is (so progress is correct when phases are skipped).
// When empty, current phase is inferred as the next in list after completedPhase.
func (p *ProgressTracker) Progress(
	completedPhase, currentPhase OperationPhase, completedSubPhase OperationSubPhase, opts ProgressOpts,
) error {
	if p.onProgressFunc == nil {
		return nil
	}

	p.mx.Lock()

	var progress Progress
	if (completedPhase == "" && completedSubPhase == "") || completedPhase != "" {
		progress = calculatePhaseProgress(p.progress, completedPhase, currentPhase, opts)
	} else {
		progress = calculateSubPhaseProgress(p.progress, completedSubPhase, opts)
	}

	p.progress = progress
	clonedProgress := progress.Clone()
	p.mx.Unlock()

	return p.onProgressFunc(clonedProgress)
}

// Complete marks the operation as complete, handling skipped phases
// It ensures that progress is correctly calculated even when some phases were skipped
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
			p.progress.Phases[i].Action = ptr.To(ProgressActionDefault)
			continue
		}
		p.progress.Phases[i].Action = ptr.To(ProgressActionSkip)
	}

	// Check if the last completed phase was skipped
	isLastCompletedPhaseSkipped := false
	if lastCompletedPhaseIndex >= 0 {
		lastPhaseAction := p.progress.Phases[lastCompletedPhaseIndex].Action
		if lastPhaseAction != nil && *lastPhaseAction == ProgressActionSkip {
			isLastCompletedPhaseSkipped = true
		}
	} else {
		isLastCompletedPhaseSkipped = true
	}

	// If the last phase was not skipped - progress 1
	if !isLastCompletedPhaseSkipped {
		p.progress.Progress = 1.0
		p.progress.CompletedPhase = nOrEmpty(p.progress.Phases, len(p.progress.Phases)-1).Phase
		p.progress.CurrentPhase = ""
		p.progress.NextPhase = ""
		clonedProgress := p.progress.Clone()

		p.mx.Unlock()

		return p.onProgressFunc(clonedProgress)
	}

	// If the last phase was skipped - find the last non-skipped phase
	lastNonSkippedPhaseIndex := -1
	for i := lastCompletedPhaseIndex; i >= 0; i-- {
		if p.progress.Phases[i].Action != nil && *p.progress.Phases[i].Action != ProgressActionSkip {
			lastNonSkippedPhaseIndex = i
			break
		}
	}

	if lastNonSkippedPhaseIndex >= 0 {
		// Found non-skipped phase - calculate progress based on it
		p.progress.Progress = float64(lastNonSkippedPhaseIndex+1) / float64(len(p.progress.Phases))
		p.progress.CompletedPhase = p.progress.Phases[lastNonSkippedPhaseIndex].Phase
	} else {
		// All phases were skipped - progress 0
		p.progress.Progress = 0.0
		p.progress.CompletedPhase = ""
	}

	p.progress.CurrentPhase = ""
	p.progress.NextPhase = ""
	clonedProgress := p.progress.Clone()

	p.mx.Unlock()

	return p.onProgressFunc(clonedProgress)
}

func calculatePhaseProgress(p Progress, completedPhase, currentPhase OperationPhase, opts ProgressOpts) Progress {
	if len(p.Phases) == 0 {
		return p
	}

	progress := Progress{
		Operation:      p.Operation,
		Phases:         p.Phases,
		Progress:       max(float64(0), p.Progress),
		CompletedPhase: completedPhase,
	}

	// return 0 progress if no completed phase
	if completedPhase == "" {
		firstPhase := nOrEmpty(p.Phases, 0)

		progress.CurrentPhase = firstPhase.Phase
		progress.NextPhase = nOrEmpty(p.Phases, 1).Phase
		progress.CurrentSubPhase = nOrEmpty(firstPhase.SubPhases, 0)
		return progress
	}

	completedPhaseIndex := slices.IndexFunc(p.Phases, func(ph PhaseWithSubPhases) bool {
		return ph.Phase == completedPhase
	})
	if completedPhaseIndex == -1 {
		// return progress as is if there is no known completedPhase for given operation
		return p
	}

	currentPhaseIndex := completedPhaseIndex + 1
	if currentPhase != "" {
		idx := slices.IndexFunc(p.Phases, func(ph PhaseWithSubPhases) bool { return ph.Phase == currentPhase })
		if idx > completedPhaseIndex {
			for i := completedPhaseIndex + 1; i < idx; i++ {
				if p.Phases[i].Action == nil {
					p.Phases[i].Action = ptr.To(ProgressActionSkip)
				}
			}
			currentPhaseIndex = idx
		}
	}

	// iterate over all previous phases, if action was nil, set action
	// if current action is skip, then skip all previous nil actions
	for i := 0; i <= completedPhaseIndex; i++ {
		if p.Phases[i].Action == nil {
			p.Phases[i].Action = &opts.Action
		}
	}

	// get current phase and first sub phase if exists
	curPhase := nOrEmpty(p.Phases, currentPhaseIndex)
	nextPhase := nOrEmpty(p.Phases, currentPhaseIndex+1)

	progress.CurrentPhase = curPhase.Phase
	progress.NextPhase = nextPhase.Phase
	progress.CurrentSubPhase = nOrEmpty(curPhase.SubPhases, 0)
	progress.NextSubPhase = nOrEmpty(curPhase.SubPhases, 1)

	progress.Progress = max(float64(currentPhaseIndex)/float64(len(p.Phases)), p.Progress)

	return progress
}

func calculateSubPhaseProgress(p Progress, completedSubPhase OperationSubPhase, _ ProgressOpts) Progress {
	var currentPhase PhaseWithSubPhases

	for _, phase := range p.Phases {
		if phase.Phase == p.CurrentPhase {
			currentPhase = phase
			break
		}
	}
	if currentPhase.Phase == "" {
		// return progress as is if there is no known current phase in the list
		return p
	}

	completedSubPhaseIndex := slices.Index(currentPhase.SubPhases, completedSubPhase)
	if completedSubPhaseIndex == -1 {
		// return progress as is if there is no known completedSubPhase for given operation
		return p
	}
	currentSubPhaseIndex := completedSubPhaseIndex + 1

	progress := Progress{
		Operation:         p.Operation,
		Phases:            p.Phases,
		Progress:          max(float64(0), p.Progress),
		CompletedPhase:    p.CompletedPhase,
		CurrentPhase:      p.CurrentPhase,
		NextPhase:         p.NextPhase,
		CompletedSubPhase: completedSubPhase,
	}

	progress.CurrentSubPhase = nOrEmpty(currentPhase.SubPhases, currentSubPhaseIndex)
	progress.NextSubPhase = nOrEmpty(currentPhase.SubPhases, currentSubPhaseIndex+1)

	progress.Progress += (1 / float64(len(p.Phases))) / float64(len(currentPhase.SubPhases))

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
