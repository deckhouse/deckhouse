/*
Copyright 2026 Flant JSC

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

package hooks

import (
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/utils/ptr"
)

const (
	// Anti-flap cooldowns — Go constants, not config-values.
	// DEBUG timings (restore before production — see PR/commit notes):
	//   raiseCooldown:  5 * time.Minute   ← prod 24 * time.Hour
	//   lowerCooldown:  15 * time.Minute  ← prod 72 * time.Hour
	raiseCooldown = 5 * time.Minute
	lowerCooldown = 15 * time.Minute
)

type decideAction int

const (
	decideSkip decideAction = iota
	decideRaise
	decideLower
)

// decide returns whether a recommendation should be committed given asymmetric
// deadband and cooldown. Pure function — covered by table tests.
func decide(rec, applied int64, lastChange, now time.Time) decideAction {
	if applied <= 0 {
		// First commit: treat as raise with no cooldown.
		if rec > 0 {
			return decideRaise
		}
		return decideSkip
	}
	if !significantResourceChange(rec, applied) {
		return decideSkip
	}
	delta := float64(rec-applied) / float64(applied)
	switch {
	case delta > raiseThreshold:
		if now.Sub(lastChange) >= raiseCooldown || lastChange.IsZero() {
			return decideRaise
		}
	case delta < -lowerThreshold:
		if now.Sub(lastChange) >= lowerCooldown || lastChange.IsZero() {
			return decideLower
		}
	}
	return decideSkip
}

func actionName(a decideAction) string {
	switch a {
	case decideRaise:
		return "raise"
	case decideLower:
		return "lower"
	default:
		return "skip"
	}
}

func appliedValue(cs autotuneComponentState, resourceName resourceKind) int64 {
	switch resourceName {
	case resourceCPU:
		if cs.AppliedMilliCPU != nil {
			return *cs.AppliedMilliCPU
		}
	case resourceMemory:
		if cs.AppliedBytes != nil {
			return *cs.AppliedBytes
		}
	}
	return 0
}

// effectiveApplied is the request currently rendered for a component: persisted
// applied* when present, otherwise the %-split of the combined budget.
func effectiveApplied(cs autotuneComponentState, resourceName resourceKind, combined int64, comp string) int64 {
	if v := appliedValue(cs, resourceName); v > 0 {
		return v
	}
	return fallbackSplit(combined, componentFallbackPercent[comp])
}

func parseLastChange(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

type initialSnapshotPlan struct {
	InitialCPU bool
	InitialMem bool
	CPUReady   bool
	MemReady   bool
}

// planInitialSnapshot decides whether each measurement may evaluate on this run.
// When both measurements need a first commit, both must have complete recs so
// cpu+memory land in one values write.
func planInitialSnapshot(
	state *autotuneState,
	cpuOverridden, memoryOverridden bool,
	recsCPU, recsMem map[string]int64,
) initialSnapshotPlan {
	p := initialSnapshotPlan{
		InitialCPU: !cpuOverridden && !measurementHasAnyApplied(state.CPU, resourceCPU),
		InitialMem: !memoryOverridden && !measurementHasAnyApplied(state.Memory, resourceMemory),
	}
	p.CPUReady = !p.InitialCPU || completeComponentRecs(recsCPU)
	p.MemReady = !p.InitialMem || completeComponentRecs(recsMem)
	if p.InitialCPU && p.InitialMem && (!p.CPUReady || !p.MemReady) {
		p.CPUReady = false
		p.MemReady = false
	}
	return p
}

func evaluateMeasurement(
	input *go_hook.HookInput,
	state *autotuneState,
	resourceName resourceKind,
	recs map[string]int64,
	nodeBudget int64,
	combinedBudget int64,
	now time.Time,
) bool {
	if len(recs) == 0 {
		return false
	}

	m := state.measurement(resourceName)
	if m == nil {
		m = &autotuneMeasurementState{Components: map[string]autotuneComponentState{}}
		state.setMeasurement(resourceName, m)
	}
	if m.Components == nil {
		m.Components = map[string]autotuneComponentState{}
	}

	proposed := make(map[string]int64, len(controlPlaneComponents))
	actions := make(map[string]decideAction, len(controlPlaneComponents))
	anyRaise := false

	for _, comp := range controlPlaneComponents {
		eff := effectiveApplied(m.Components[comp], resourceName, combinedBudget, comp)
		proposed[comp] = eff

		rec, hasRec := recs[comp]
		if !hasRec {
			actions[comp] = decideSkip
			continue
		}

		lastChange := parseLastChange(m.Components[comp].LastChange)
		// Compare against the currently rendered request (applied* or %-split
		// fallback) so the first snapshot after clearing a manual override does
		// not unconditionally rewrite every component.
		action := decide(rec, eff, lastChange, now)
		actions[comp] = action
		if action == decideRaise || action == decideLower {
			proposed[comp] = rec
		}
		if action == decideRaise {
			anyRaise = true
		}
	}

	changed := false

	if anyRaise {
		var sum int64
		for _, comp := range controlPlaneComponents {
			sum += proposed[comp]
		}
		if sum > nodeBudget {
			deficit := sum - nodeBudget
			for _, comp := range controlPlaneComponents {
				if actions[comp] == decideRaise {
					proposed[comp] = effectiveApplied(m.Components[comp], resourceName, combinedBudget, comp)
					actions[comp] = decideSkip
				}
			}
			if m.CapacityBlocked == nil {
				m.CapacityBlocked = &capacityBlocked{Since: now.Format(time.RFC3339), Deficit: deficit}
			} else {
				m.CapacityBlocked.Deficit = deficit
			}
			changed = true
			input.Logger.Info("autotune: raise blocked by capacity gate",
				"resource", resourceName, "deficit", deficit, "budget", nodeBudget, "proposedSum", sum)
		} else if m.CapacityBlocked != nil {
			m.CapacityBlocked = nil
			changed = true
		}
	} else if m.CapacityBlocked != nil {
		m.CapacityBlocked = nil
		changed = true
	}

	for _, comp := range controlPlaneComponents {
		action := actions[comp]
		if action == decideSkip {
			continue
		}
		cs := m.Components[comp]
		val := proposed[comp]
		switch resourceName {
		case resourceCPU:
			cs.AppliedMilliCPU = ptr.To(val)
		case resourceMemory:
			cs.AppliedBytes = ptr.To(val)
		}
		cs.LastChange = now.Format(time.RFC3339)
		m.Components[comp] = cs
		changed = true
		input.Logger.Info("autotune: committed recommendation",
			"component", comp, "resource", resourceName, "action", actionName(action), "value", val)
	}

	return changed
}
