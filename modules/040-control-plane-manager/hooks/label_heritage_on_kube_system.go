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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ns",
			ApiVersion:                   "v1",
			Kind:                         "Namespace",
			NameSelector:                 &types.NameSelector{MatchNames: []string{"kube-system"}},
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
			FilterFunc:                   filterResource,
		},
	},
}, labelHeritage)

func filterResource(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if unstructured.GetLabels()["heritage"] == "deckhouse" {
		return nil, nil
	}
	return unstructured.GetName(), nil
}

func labelHeritage(_ context.Context, input *go_hook.HookInput) error {
	nsPatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"heritage": "deckhouse",
			},
		},
	}

	snaps := input.Snapshots.Get("ns")

	if len(snaps) == 1 {
		var name string
		err := snaps[0].UnmarshalTo(&name)
		if err != nil {
			return fmt.Errorf("failed to unmarshal 'ns' snapshot: %w", err)
		}

		input.PatchCollector.PatchWithMerge(nsPatch, "v1", "Namespace", "", name)
	}

	return nil
}
