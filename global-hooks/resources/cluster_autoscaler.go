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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_autoscaler_resources",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cluster-autoscaler"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyAutoscalerResourcesFilter,
		},
	},
}, removeResourcesLimitsForClusterAutoscaler)

func applyAutoscalerResourcesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var depl appsv1.Deployment
	err := sdk.FromUnstructured(obj, &depl)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	return depl.Spec.Template.Spec.Containers[0].Resources, nil
}

// removeResourcesLimitsForClusterAutoscaler
// If there is a Deployment kube-system/cluster-autoscaler in cluster,
// it must not have section `resources.limits` because extended-monitoring will alert at throttling.
func removeResourcesLimitsForClusterAutoscaler(_ context.Context, input *go_hook.HookInput) error {
	resourcesSnap, err := sdkobjectpatch.UnmarshalToStruct[corev1.ResourceRequirements](input.Snapshots, "cluster_autoscaler_resources")
	if err != nil {
		return fmt.Errorf("cannot unmarshal cluster_autoscaler_resources snapshot: %w", err)
	}

	if len(resourcesSnap) == 0 {
		return nil
	}

	resources := resourcesSnap[0]
	if len(resources.Limits) == 0 {
		return nil
	}

	filterResourceLimits := func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var depl appsv1.Deployment
		err := sdk.FromUnstructured(u, &depl)
		if err != nil {
			return nil, fmt.Errorf("from unstructured: %w", err)
		}

		// Remove resource limits from the first container
		depl.Spec.Template.Spec.Containers[0].Resources.Limits = nil

		return sdk.ToUnstructured(&depl)
	}

	input.PatchCollector.PatchWithMutatingFunc(filterResourceLimits, "apps/v1", "Deployment", "kube-system", "cluster-autoscaler")

	return nil
}
