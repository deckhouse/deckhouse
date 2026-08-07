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
	"context"
	"fmt"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// Hook B: ConfigMap → values.internal.resourcesRequests.components only.
// Does not know how Hook A calculated the numbers (MC split / PodMetrics / legacy).
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        autotuneQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		autotuneStateBinding(true, true),
	},
}, syncResourcesRequestsFromConfigMap)

func syncResourcesRequestsFromConfigMap(_ context.Context, input *go_hook.HookInput) error {
	states, err := sdkobjectpatch.UnmarshalToStruct[autotuneState](input.Snapshots, snapshotAutotune)
	if err != nil {
		return fmt.Errorf("unmarshal AutotuneState snapshots: %w", err)
	}
	if len(states) == 0 {
		if input.Values.Exists(pathComponents) {
			input.Values.Remove(pathComponents)
		}
		return nil
	}
	return projectAutotuneStateToValues(input, states[0])
}

func projectAutotuneStateToValues(input *go_hook.HookInput, state autotuneState) error {
	if state == nil {
		if input.Values.Exists(pathComponents) {
			input.Values.Remove(pathComponents)
		}
		return nil
	}

	components := map[string]any{}
	for _, comp := range controlPlaneComponents {
		entry := map[string]any{}
		if m := state[resourceCPU]; m != nil {
			if cs, ok := m.Components[comp]; ok && cs.AppliedMilliCPU != nil {
				entry["milliCPU"] = *cs.AppliedMilliCPU
			}
		}
		if m := state[resourceMemory]; m != nil {
			if cs, ok := m.Components[comp]; ok && cs.AppliedBytes != nil {
				entry["memoryBytes"] = strconv.FormatInt(*cs.AppliedBytes, 10)
			}
		}
		if len(entry) > 0 {
			components[comp] = entry
		}
	}

	if len(components) == 0 {
		if input.Values.Exists(pathComponents) {
			input.Values.Remove(pathComponents)
		}
		return nil
	}
	input.Values.Set(pathComponents, components)
	return nil
}
