/*
Copyright 2024 Flant JSC

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
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// should run after start pod
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 1},
}, dependency.WithExternalDependencies(setLeaderLabelToPod))

// setLeaderLabelToPod just add `deckhouse-leader` label to leader pod
func setLeaderLabelToPod(input *go_hook.HookInput, dc dependency.Container) error {
	podName := os.Getenv("DECKHOUSE_POD")

	client, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	pods, err := client.CoreV1().Pods(d8Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: "app=deckhouse"})
	if err != nil {
		input.LogEntry.Warnf("Error getting deckhouse pods: %s", err)
		return nil
	}

	for _, pod := range pods.Items {
		patch := map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					"leader": trueOrNil(pod.Name == podName),
				},
			},
		}

		input.PatchCollector.MergePatch(patch, "v1", "Pod", d8Namespace, pod.Name)
	}

	return nil
}

func trueOrNil(b bool) any {
	if b {
		return true
	}

	return nil
}
