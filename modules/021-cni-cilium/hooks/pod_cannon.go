// Copyright 2021 Flant JSC
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

package hooks

import (
	"context"
	"fmt"
	"time"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "test_hook",
			ApiVersion: "v1",
			Kind:       "Pod",
			FilterFunc: applyTestHookFilter,
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"pod-cannon"},
				},
			},
		},
	},
}, runTestHook)

func applyTestHookFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := v1core.Pod{}
	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, err
	}
	return pod, nil
}

func runTestHook(_ context.Context, input *go_hook.HookInput) error {
	fmt.Println("[TEST HOOK] get pods")
	pods, err := sdkobjectpatch.UnmarshalToStruct[v1core.Pod](input.Snapshots, "test_hook")
	if err != nil {
		return fmt.Errorf("failed to unmarshal pods: %w", err)
	}
	fmt.Println("[TEST HOOK] len of pods", len(pods))
	for _, pod := range pods {
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"test-hook/self-trigger": fmt.Sprintf("%s", time.Now().UnixNano()),
				},
			},
		}
		input.PatchCollector.PatchWithMerge(
			patch,
			"v1",
			"Pod",
			pod.GetNamespace(),
			pod.GetName(),
		)
		fmt.Println("[TEST HOOK] patched pod", pod.GetName())
	}
	fmt.Println("[TEST HOOK] sleeping for 20 seconds")
	fmt.Println("[TEST HOOK] done")
	return nil
}
