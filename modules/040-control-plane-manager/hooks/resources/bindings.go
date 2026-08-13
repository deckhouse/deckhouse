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

// What the hooks subscribe to, and what each object is boiled down to before it
// reaches them. Filters run inside the informer, where an error aborts snapshot
// creation and the hook never runs at all — so they only reshape, never validate.

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const (
	snapshotCPMMC    = "CPMResourcesRequests"
	snapshotGlobalMC = "GlobalResourcesRequests"
	snapshotAutotune = "AutotuneState"
	snapshotNodes    = "NodesResources"
)

// controlPlaneNodesBinding watches master Nodes.
func controlPlaneNodesBinding() go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       snapshotNodes,
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
		ExecuteHookOnSynchronization: ptr.To(true),
	}
}

func resourcesRequestsMCBinding(name, mcName string, filter go_hook.FilterFunc) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       name,
		ApiVersion: "deckhouse.io/v1alpha1",
		Kind:       "ModuleConfig",
		NameSelector: &types.NameSelector{
			MatchNames: []string{mcName},
		},
		FilterFunc:                   filter,
		ExecuteHookOnEvents:          ptr.To(true),
		ExecuteHookOnSynchronization: ptr.To(true),
	}
}

func autotuneStateBinding(onSync, onEvents bool) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       snapshotAutotune,
		ApiVersion: "v1",
		Kind:       "ConfigMap",
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{MatchNames: []string{kubeSystemNS}},
		},
		NameSelector: &types.NameSelector{
			MatchNames: []string{autotuneStateCMName},
		},
		FilterFunc:                   applyAutotuneStateFilter,
		ExecuteHookOnEvents:          ptr.To(onEvents),
		ExecuteHookOnSynchronization: ptr.To(onSync),
	}
}

func applyNodesResourcesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	n := &Node{
		Name:                node.GetName(),
		AllocatableMilliCPU: node.Status.Allocatable.Cpu().MilliValue(),
		AllocatableMemory:   node.Status.Allocatable.Memory().Value(),
		CapacityMilliCPU:    node.Status.Capacity.Cpu().MilliValue(),
		CapacityMemory:      node.Status.Capacity.Memory().Value(),
	}
	// Test fixtures and very early node objects may not report Capacity yet —
	// fall back to Allocatable. The downstream logic treats `Capacity == Allocatable`
	// as `kubelet has not subtracted its reservation yet` and applies the floor.
	if n.CapacityMilliCPU == 0 {
		n.CapacityMilliCPU = n.AllocatableMilliCPU
	}
	if n.CapacityMemory == 0 {
		n.CapacityMemory = n.AllocatableMemory
	}

	return n, nil
}

func applyAutotuneStateFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	if err := sdk.FromUnstructured(obj, cm); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	return autotuneStateRaw{State: cm.Data[autotuneStateKey]}, nil
}

func applyCPMResourcesRequestsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &v1alpha1.ModuleConfig{}
	if err := sdk.FromUnstructured(obj, mc); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	if mc.Spec.Settings == nil {
		return moduleConfigResourcesRequests{}, nil
	}
	settings := mc.Spec.Settings.GetMap()
	rr, _ := settings["resourcesRequests"].(map[string]any)
	return moduleConfigResourcesRequests{
		CPU:    quantityString(rr["cpu"]),
		Memory: quantityString(rr["memory"]),
	}, nil
}

func applyGlobalResourcesRequestsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &v1alpha1.ModuleConfig{}
	if err := sdk.FromUnstructured(obj, mc); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}
	if mc.Spec.Settings == nil {
		return moduleConfigResourcesRequests{}, nil
	}
	settings := mc.Spec.Settings.GetMap()
	modules, _ := settings["modules"].(map[string]any)
	rr, _ := modules["resourcesRequests"].(map[string]any)
	cp, _ := rr["controlPlane"].(map[string]any)
	return moduleConfigResourcesRequests{
		CPU:    quantityString(cp["cpu"]),
		Memory: quantityString(cp["memory"]),
	}, nil
}

func quantityString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		// cpu may be a bare number. %.0f would turn 0.5 into "0m" and 1.5 into "2000m".
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}
