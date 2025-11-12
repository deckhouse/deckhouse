/*
Copyright 2023 Flant JSC

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
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			FilterFunc: applyPodCannonFilter,
		},
	},
}, labelAllPods)

func applyPodCannonFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &corev1.Pod{}
	if err := sdk.FromUnstructured(obj, pod); err != nil {
		return nil, err
	}

	return pod, nil
}

func labelAllPods(_ context.Context, input *go_hook.HookInput) error {
	for pod, err := range sdkobjectpatch.SnapshotIter[*corev1.Pod](input.Snapshots.Get("pods")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'pods' snapshots: %w", err)
		}

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"pod-cannon": time.Now().Unix(),
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, "v1", "Pod", pod.Namespace, pod.Name)
	}

	return nil
}
