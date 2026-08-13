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

package resources

// autotuneState is what the two hooks share: hook_autotune.go writes it into a
// ConfigMap, hook_sync.go reads that ConfigMap back and projects it into values.
// A ConfigMap rather than values directly, because the requests must survive a
// Deckhouse restart without a metrics API round-trip.

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	autotuneStateCMName = "d8-control-plane-manager-resources-autotune-state"
	autotuneStateKey    = "state"

	// 20h and not 24h so that the 03:00 cron run is never skipped because the
	// previous run happened a few minutes early or the scheduler drifted.
	metricsRunInterval = 20 * time.Hour
)

type autotuneComponentState struct {
	AppliedMilliCPU *int64 `json:"appliedMilliCPU,omitempty"`
	AppliedBytes    *int64 `json:"appliedBytes,omitempty"`
	LastChange      string `json:"lastChange,omitempty"`
}

// PendingRaiseSum is the last raise total that failed the headroom gate (no
// since/deficit in CM — deficit is emitted as a metric only).
type autotuneMeasurementState struct {
	Components      map[string]autotuneComponentState `json:"components,omitempty"`
	PendingRaiseSum int64                             `json:"pendingRaiseSum,omitempty"`

	// RFC3339, UTC. The hook is woken by events as well as by cron; this is what
	// keeps raise/lower inside the daily window.
	LastMetricsRun string `json:"lastMetricsRun,omitempty"`
	// Normalized to millicpu or bytes — as a raw string "2" and "2000m" would look
	// like a change every run. nil means the measurement is under autotune.
	AppliedOverride *int64 `json:"appliedOverride,omitempty"`
}

type autotuneState map[resourceKind]*autotuneMeasurementState

// Carried unparsed because the filter runs inside the informer, where an error
// aborts snapshot creation and the hook never runs at all.
type autotuneStateRaw struct {
	State string `json:"state,omitempty"`
}

func (s autotuneState) getOrCreateMeasurement(kind resourceKind) *autotuneMeasurementState {
	m := s[kind]
	if m == nil {
		m = &autotuneMeasurementState{Components: map[string]autotuneComponentState{}}
		s[kind] = m
	}
	if m.Components == nil {
		m.Components = map[string]autotuneComponentState{}
	}
	return m
}

// commit writes a resolution into the state the ConfigMap is rendered from and
// returns the components whose value actually changed.
//
// Through setAppliedIfChanged rather than a plain overwrite: LastChange must move
// only on a real change, or every run would reset the raise/lower cooldowns to
// "just now" and the anti-flap window would never elapse.
func (s autotuneState) commit(kind resourceKind, requests requestsByComponent, changedAt string) []string {
	measurement := s.getOrCreateMeasurement(kind)

	var changed []string
	for _, comp := range controlPlaneComponents {
		request := requests[comp]
		if request <= 0 {
			// The resolver chain guarantees a positive value for every component; a zero
			// would reach the static-pod manifests as a literal request of 0. Skipping
			// leaves that component on the template fallback, and the sync hook reports
			// the incomplete map through autotuneIncompleteMetricName.
			continue
		}
		if setAppliedIfChanged(measurement, comp, kind, request, changedAt) {
			changed = append(changed, comp)
		}
	}
	return changed
}

// refreshPendingRaiseDeficit rechecks a raise an earlier run had to hold back:
// headroom is recomputed on every run, so the blocked total may fit by now.
func (s autotuneState) refreshPendingRaiseDeficit(kind resourceKind, headroom int64) int64 {
	m := s[kind]
	if m == nil || m.PendingRaiseSum <= 0 {
		return 0
	}
	if m.PendingRaiseSum <= headroom {
		m.PendingRaiseSum = 0
		return 0
	}
	return m.PendingRaiseSum - headroom
}

// LastChange is touched only on an actual change: rewriting it every run would
// keep it permanently "just now" and block the next raise/lower on cooldown.
func setAppliedIfChanged(m *autotuneMeasurementState, comp string, kind resourceKind, val int64, ts string) bool {
	cs := m.Components[comp]
	switch kind {
	case resourceCPU:
		if cs.AppliedMilliCPU != nil && *cs.AppliedMilliCPU == val {
			return false
		}
	case resourceMemory:
		if cs.AppliedBytes != nil && *cs.AppliedBytes == val {
			return false
		}
	}
	setApplied(&cs, kind, val)
	cs.LastChange = ts
	m.Components[comp] = cs
	return true
}

func setApplied(cs *autotuneComponentState, kind resourceKind, val int64) {
	switch kind {
	case resourceCPU:
		cs.AppliedMilliCPU = ptr.To(val)
	case resourceMemory:
		cs.AppliedBytes = ptr.To(val)
	}
}

func appliedValue(cs autotuneComponentState, kind resourceKind) int64 {
	switch kind {
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

func countApplied(m *autotuneMeasurementState, kind resourceKind) int {
	if m == nil || m.Components == nil {
		return 0
	}
	n := 0
	for _, comp := range controlPlaneComponents {
		if appliedValue(m.Components[comp], kind) > 0 {
			n++
		}
	}
	return n
}

func measurementIsComplete(m *autotuneMeasurementState, kind resourceKind) bool {
	return countApplied(m, kind) == len(controlPlaneComponents)
}

// metricsRunDue keeps event- and sync-driven runs from touching raise/lower
// outside the daily window.
func metricsRunDue(m *autotuneMeasurementState, now time.Time) bool {
	if m == nil || m.LastMetricsRun == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, m.LastMetricsRun)
	return err != nil || now.Sub(t) >= metricsRunInterval
}

// readStateOrEmpty never fails: recomputing the whole ConfigMap from scratch
// beats leaving the control-plane on the template fallback, so an unreadable
// state is reported and then treated as no state at all.
func readStateOrEmpty(input *go_hook.HookInput) autotuneState {
	snapshots := input.Snapshots.Get(snapshotAutotune)
	if len(snapshots) == 0 {
		return make(autotuneState)
	}

	var raw autotuneStateRaw
	if err := snapshots[0].UnmarshalTo(&raw); err != nil {
		return emptyStateAfter(input, fmt.Errorf("unmarshal AutotuneState snapshot: %w", err))
	}

	state, err := parseAutotuneState(raw.State)
	if err != nil {
		return emptyStateAfter(input, err)
	}
	return state
}

func emptyStateAfter(input *go_hook.HookInput, err error) autotuneState {
	input.Logger.Warn("autotune: unreadable state, recomputing from scratch", "error", err)
	setAutotuneDegraded(input, autotuneDegradedMetricGroup, degradedReasonBadState)
	return make(autotuneState)
}

// parseAutotuneState never returns a nil map on success: an absent ConfigMap key
// and a literal JSON null both mean "no state yet", which callers must be able to
// write into.
func parseAutotuneState(raw string) (autotuneState, error) {
	if raw == "" {
		return make(autotuneState), nil
	}

	var state autotuneState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, fmt.Errorf("unmarshal autotune state: %w", err)
	}
	if state == nil {
		return make(autotuneState), nil
	}
	return state, nil
}

func persistAutotuneState(input *go_hook.HookInput, state autotuneState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal autotune state: %w", err)
	}

	cm := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      autotuneStateCMName,
			Namespace: kubeSystemNS,
			Labels: map[string]string{
				"heritage": "deckhouse",
				"module":   "control-plane-manager",
			},
		},
		Data: map[string]string{
			autotuneStateKey: string(raw),
		},
	}

	input.PatchCollector.CreateOrUpdate(cm)
	return nil
}
