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

package autotune

// Shared by the two hooks through a ConfigMap rather than values, so that the
// requests survive a restart without a metrics API round-trip.

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
	// previous one ran a few minutes early. Also the unit the cooldowns count in.
	metricsRunInterval = 20 * time.Hour
)

type autotuneComponentState struct {
	AppliedMilliCPU *int64 `json:"appliedMilliCPU,omitempty"`
	AppliedBytes    *int64 `json:"appliedBytes,omitempty"`
	LastChange      string `json:"lastChange,omitempty"`
}

type autotuneMeasurementState struct {
	Components      map[string]autotuneComponentState `json:"components,omitempty"`
	PendingRaiseSum int64                             `json:"pendingRaiseSum,omitempty"`
	LastMetricsRun  string                            `json:"lastMetricsRun,omitempty"`
	// Normalized to millicpu or bytes, or "2" and "2000m" would look like a change
	// on every run. nil means "under autotune".
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

func (s autotuneState) commit(kind resourceKind, requests requestsByComponent, changedAt string) []string {
	measurement := s.getOrCreateMeasurement(kind)

	var changed []string
	for _, comp := range controlPlaneComponents {
		request := requests[comp]
		if request <= 0 {
			continue
		}
		if measurement.setAppliedIfChanged(comp, kind, request, changedAt) {
			changed = append(changed, comp)
		}
	}
	return changed
}

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

func (m *autotuneMeasurementState) handOverToFallback() {
	if m == nil {
		return
	}
	m.AppliedOverride = nil
	m.PendingRaiseSum = 0
}

func (m *autotuneMeasurementState) dueForMetricsRun(now time.Time) bool {
	if m == nil || m.LastMetricsRun == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, m.LastMetricsRun)
	return err != nil || now.Sub(last) >= metricsRunInterval
}

func (m *autotuneMeasurementState) setAppliedIfChanged(comp string, kind resourceKind, val int64, ts string) bool {
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
	cs.setApplied(kind, val)
	cs.LastChange = ts
	m.Components[comp] = cs
	return true
}

func (m *autotuneMeasurementState) appliedRequest(comp string, kind resourceKind) int64 {
	if m == nil {
		return 0
	}
	cs := m.Components[comp]
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

func (m *autotuneMeasurementState) countApplied(kind resourceKind) int {
	n := 0
	for _, comp := range controlPlaneComponents {
		if m.appliedRequest(comp, kind) > 0 {
			n++
		}
	}
	return n
}

func (m *autotuneMeasurementState) isComplete(kind resourceKind) bool {
	return m.countApplied(kind) == len(controlPlaneComponents)
}

func (cs *autotuneComponentState) setApplied(kind resourceKind, val int64) {
	switch kind {
	case resourceCPU:
		cs.AppliedMilliCPU = ptr.To(val)
	case resourceMemory:
		cs.AppliedBytes = ptr.To(val)
	}
}

// Never fails: recomputing from scratch beats the template default.
func readStateOrEmpty(input *go_hook.HookInput) autotuneState {
	state, err := readState(input)
	if err != nil {
		input.Logger.Warn("autotune: unreadable state, recomputing from scratch", "error", err)
		setAutotuneDegraded(input, autotuneDegradedMetricGroup, degradedReasonBadState)
		return make(autotuneState)
	}
	return state
}

func readState(input *go_hook.HookInput) (autotuneState, error) {
	snapshots := input.Snapshots.Get(snapshotAutotune)
	if len(snapshots) == 0 {
		return make(autotuneState), nil
	}

	var raw autotuneStateRaw
	if err := snapshots[0].UnmarshalTo(&raw); err != nil {
		return nil, fmt.Errorf("unmarshal AutotuneState snapshot: %w", err)
	}
	return parseAutotuneState(raw.State)
}

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

func (s autotuneState) persist(input *go_hook.HookInput) error {
	raw, err := json.Marshal(s)
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
