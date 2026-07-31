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
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
)

const (
	autotuneStateCMName = "d8-control-plane-manager-resources-autotune-state"
	autotuneStateKey    = "state"

	autotuneMetricName  = "d8_control_plane_manager_resources_autotune_insufficient_capacity"
	autotuneMetricGroup = "D8ControlPlaneResourcesAutotuneInsufficientCapacity"
)

type autotuneComponentState struct {
	AppliedMilliCPU *int64 `json:"appliedMilliCPU,omitempty"`
	AppliedBytes    *int64 `json:"appliedBytes,omitempty"`
	LastChange      string `json:"lastChange,omitempty"`
}

type capacityBlocked struct {
	Since   string `json:"since"`
	Deficit int64  `json:"deficit"`
}

type autotuneMeasurementState struct {
	Components      map[string]autotuneComponentState `json:"components,omitempty"`
	CapacityBlocked *capacityBlocked                  `json:"capacityBlocked,omitempty"`
}

// autotuneState nests by measurement (cpu/memory) so a manual override can delete
// a whole measurement branch for all four components in one patch.
type autotuneState struct {
	CPU    *autotuneMeasurementState `json:"cpu,omitempty"`
	Memory *autotuneMeasurementState `json:"memory,omitempty"`
}

func (s *autotuneState) measurement(resourceName resourceKind) *autotuneMeasurementState {
	switch resourceName {
	case resourceCPU:
		return s.CPU
	case resourceMemory:
		return s.Memory
	default:
		return nil
	}
}

func (s *autotuneState) setMeasurement(resourceName resourceKind, m *autotuneMeasurementState) {
	switch resourceName {
	case resourceCPU:
		s.CPU = m
	case resourceMemory:
		s.Memory = m
	}
}

func (s *autotuneState) deleteMeasurement(resourceName resourceKind) {
	s.setMeasurement(resourceName, nil)
}

func applyAutotuneStateFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	raw, ok := cm.Data[autotuneStateKey]
	if !ok || raw == "" {
		return &autotuneState{}, nil
	}
	var st autotuneState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil, fmt.Errorf("unmarshal autotune state: %w", err)
	}
	return &st, nil
}

func readAutotuneState(input *go_hook.HookInput) (*autotuneState, error) {
	snapshots := input.Snapshots.Get("AutotuneState")
	if len(snapshots) == 0 {
		return &autotuneState{}, nil
	}
	var st autotuneState
	if err := snapshots[0].UnmarshalTo(&st); err != nil {
		return nil, fmt.Errorf("unmarshal AutotuneState snapshot: %w", err)
	}
	return &st, nil
}

func persistAutotuneState(input *go_hook.HookInput, state *autotuneState) error {
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

	gvks, _, err := scheme.Scheme.ObjectKinds(cm)
	if err == nil && len(gvks) > 0 {
		cm.SetGroupVersionKind(gvks[0])
	}

	input.PatchCollector.CreateOrUpdate(cm)
	return nil
}

func autotuneNodesBinding(onSync bool) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       "NodesResources",
		ApiVersion: "v1",
		Kind:       "Node",
		LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "node-role.kubernetes.io/control-plane",
				Operator: metav1.LabelSelectorOpExists,
			},
		}},
		FilterFunc:                   applyNodesResourcesFilter,
		ExecuteHookOnEvents:          ptr.To(false),
		ExecuteHookOnSynchronization: ptr.To(onSync),
	}
}

func autotuneStateBinding(onSync bool) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       "AutotuneState",
		ApiVersion: "v1",
		Kind:       "ConfigMap",
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{MatchNames: []string{kubeSystemNS}},
		},
		NameSelector: &types.NameSelector{
			MatchNames: []string{autotuneStateCMName},
		},
		FilterFunc:                   applyAutotuneStateFilter,
		ExecuteHookOnEvents:          ptr.To(false),
		ExecuteHookOnSynchronization: ptr.To(onSync),
	}
}

func measurementHasAnyApplied(m *autotuneMeasurementState, resourceName resourceKind) bool {
	if m == nil || m.Components == nil {
		return false
	}
	for _, comp := range controlPlaneComponents {
		if appliedValue(m.Components[comp], resourceName) > 0 {
			return true
		}
	}
	return false
}

// fillMissingAppliedFromFallback writes %-split baselines into empty applied*
// slots so the first values snapshot covers every component in one ModuleRun.
func fillMissingAppliedFromFallback(state *autotuneState, resourceName resourceKind, combinedBudget int64) bool {
	if combinedBudget <= 0 {
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
	changed := false
	for _, comp := range controlPlaneComponents {
		if appliedValue(m.Components[comp], resourceName) > 0 {
			continue
		}
		cs := m.Components[comp]
		val := fallbackSplit(combinedBudget, componentFallbackPercent[comp])
		switch resourceName {
		case resourceCPU:
			cs.AppliedMilliCPU = ptr.To(val)
		case resourceMemory:
			cs.AppliedBytes = ptr.To(val)
		}
		m.Components[comp] = cs
		changed = true
	}
	return changed
}

// projectComponentsToValues builds the internal values map from persistent state.
func projectComponentsToValues(state *autotuneState, cpuOverridden, memoryOverridden bool) map[string]any {
	components := map[string]any{}
	for _, comp := range controlPlaneComponents {
		entry := map[string]any{}
		if !cpuOverridden {
			if m := state.measurement(resourceCPU); m != nil {
				if cs, ok := m.Components[comp]; ok && cs.AppliedMilliCPU != nil {
					entry["milliCPU"] = *cs.AppliedMilliCPU
				}
			}
		}
		if !memoryOverridden {
			if m := state.measurement(resourceMemory); m != nil {
				if cs, ok := m.Components[comp]; ok && cs.AppliedBytes != nil {
					entry["memoryBytes"] = *cs.AppliedBytes
				}
			}
		}
		if len(entry) > 0 {
			components[comp] = entry
		}
	}
	return components
}

func repopulateComponents(input *go_hook.HookInput, state *autotuneState, cpuOverridden, memoryOverridden bool) {
	components := projectComponentsToValues(state, cpuOverridden, memoryOverridden)
	if len(components) == 0 {
		if input.Values.Exists(pathComponents) {
			input.Values.Remove(pathComponents)
		}
		return
	}
	// Set the whole map so JSON-patch does not need intermediate parents.
	input.Values.Set(pathComponents, components)
}

func emitCapacityBlockedMetrics(input *go_hook.HookInput, state *autotuneState) {
	for _, res := range []resourceKind{resourceCPU, resourceMemory} {
		m := state.measurement(res)
		if m == nil || m.CapacityBlocked == nil {
			continue
		}
		input.MetricsCollector.Set(
			autotuneMetricName,
			float64(m.CapacityBlocked.Deficit),
			map[string]string{"resource": string(res)},
			metrics.WithGroup(autotuneMetricGroup),
		)
	}
}

// discardAutotuneForLegacy clears per-component internal values and persistent
// autotune state so templates use the fixed %-split of milliCpuControlPlane /
// memoryControlPlane from the legacy calculate hook.
func discardAutotuneForLegacy(input *go_hook.HookInput) error {
	input.Logger.Info("autotune: prometheus or prometheus-metrics-adapter disabled, discarding autotune and falling back to legacy combined budget")
	input.MetricsCollector.Expire(autotuneMetricGroup)
	if input.Values.Exists(pathComponents) {
		input.Values.Remove(pathComponents)
	}
	input.PatchCollector.Delete("v1", "ConfigMap", kubeSystemNS, autotuneStateCMName)
	return nil
}
