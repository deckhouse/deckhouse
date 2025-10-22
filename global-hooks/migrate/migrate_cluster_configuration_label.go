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

/*
This migration adds label name: d8-cluster-configuration to the d8-cluster-configuration secret
TODO: remove after deckhouse 1.68
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

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "clusterConfiguration",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-cluster-configuration"}},
			// it's enough to start it only on the first run
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterName,
		},
	},
}, handleClusterConfigurationLabel)

// Required to run the hook when the k8s version has been changed
func filterName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetLabels(), nil
}

func handleClusterConfigurationLabel(_ context.Context, input *go_hook.HookInput) error {
	snap, err := sdkobjectpatch.UnmarshalToStruct[map[string]string](input.Snapshots, "clusterConfiguration")
	if err != nil {
		return fmt.Errorf("failed to unmarshal clusterConfiguration snapshot: %w", err)
	}
	if len(snap) == 0 {
		return nil
	}
	labels := snap[0]

	if _, ok := labels["name"]; ok {
		return nil
	}

	m := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"name": "d8-cluster-configuration",
			},
		},
	}

	input.PatchCollector.PatchWithMerge(m, "v1", "Secret", "kube-system", "d8-cluster-configuration")
	return nil
}
