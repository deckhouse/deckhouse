/*
Copyright 2025 Flant JSC

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

// ToDo can be deleted after 1.75

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
	"github.com/deckhouse/module-sdk/pkg/utils/ptr"
)

type GrafanaV8Resource struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

// We cannot delete resources directly; only resources with the "heritage: deckhouse" label should be removed.
// This is important because users may create Services or Ingresses with the same names for backward compatibility.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Queue:       "/modules/prometheus/grafana_v8_cleanup",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "grafana-v8-deployments",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: v1.LabelSelectorOpIn,
						Values: []string{
							"deckhouse",
						},
					},
				},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"grafana",
				"grafana-v8-dex-authenticator",
			}},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterResources,
		},
		{
			Name:       "grafana-v8-services",
			ApiVersion: "v1",
			Kind:       "Service",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: v1.LabelSelectorOpIn,
						Values: []string{
							"deckhouse",
						},
					},
				},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"grafana",
				"grafana-v8-dex-authenticator",
			}},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterResources,
		},
		{
			Name:       "grafana-v8-ingresses",
			ApiVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: v1.LabelSelectorOpIn,
						Values: []string{
							"deckhouse",
						},
					},
				},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"grafana",
				"grafana-v8-dex-authenticator",
				"grafana-v8-dex-authenticator-sign-out",
			}},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterResources,
		},
		{
			Name:       "grafana-v8-pdb",
			ApiVersion: "policy/v1",
			Kind:       "PodDisruptionBudget",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: v1.LabelSelectorOpIn,
						Values: []string{
							"deckhouse",
						},
					},
				},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"grafana-v8-dex-authenticator",
			}},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterResources,
		},
	},
}, grafanaV8ResourcesHandler)

func filterResources(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var resource GrafanaV8Resource

	err := sdk.FromUnstructured(obj, &resource)
	if err != nil {
		return nil, err
	}

	resource.APIVersion = obj.GetAPIVersion()
	resource.Metadata.Name = obj.GetName()
	resource.Metadata.Namespace = obj.GetNamespace()
	resource.Kind = obj.GetKind()

	return resource, nil
}

func grafanaV8ResourcesHandler(_ context.Context, input *go_hook.HookInput) error {
	resources := make([][]GrafanaV8Resource, 0)
	for _, resource := range []string{"grafana-v8-deployments", "grafana-v8-services", "grafana-v8-ingresses", "grafana-v8-pdb"} {
		snapshots, err := sdkobjectpatch.UnmarshalToStruct[GrafanaV8Resource](input.Snapshots, resource)
		if err != nil {
			return fmt.Errorf("failed to unmarshal %s snapshot: %w", resource, err)
		}

		resources = append(resources, snapshots)
	}

	for _, snap := range resources {
		for _, resource := range snap {
			input.PatchCollector.Delete(resource.APIVersion, resource.Kind, resource.Metadata.Namespace, resource.Metadata.Name)
		}
	}

	return nil
}
