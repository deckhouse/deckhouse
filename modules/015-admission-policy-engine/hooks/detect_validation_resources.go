/*
Copyright 2022 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "constraint-exporter-cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-admission-policy-engine"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"constraint-exporter"},
			},
			FilterFunc: filterExporterCM,
		},
	},
}, dependency.WithExternalDependencies(handleValidationKinds))

func handleValidationKinds(input *go_hook.HookInput, dc dependency.Container) error {
	snap := input.Snapshots["constraint-exporter-cm"]
	if len(snap) == 0 {
		input.LogEntry.Info("no exporter cm found")
		return nil
	}

	kindsRaw := snap[0].(string)

	var matchKinds []matchKind

	err := yaml.Unmarshal([]byte(kindsRaw), &matchKinds)
	if err != nil {
		return err
	}

	if len(matchKinds) == 0 {
		return nil
	}

	res := make([]matchResource, 0, len(matchKinds))

	k8s, err := dc.GetK8sClient()
	if err != nil {
		return err
	}
	apiRes, err := restmapper.GetAPIGroupResources(k8s.Discovery())
	if err != nil {
		return err
	}

	rmapper := restmapper.NewDiscoveryRESTMapper(apiRes)

	for _, mk := range matchKinds {
		uniqGroups := make(map[string]struct{})
		uniqResources := make(map[string]struct{})

		for _, apiGroup := range mk.APIGroups {
			for _, kind := range mk.Kinds {
				rm, err := rmapper.RESTMapping(schema.GroupKind{
					Group: apiGroup,
					Kind:  kind,
				})
				if err != nil {
					input.LogEntry.Warnf("Resource mapping failed. Group: %s, Kind: %s. Error: %s", apiGroup, kind, err)
					continue
				}

				uniqGroups[rm.Resource.Group] = struct{}{}
				uniqResources[rm.Resource.Resource] = struct{}{}
			}
		}

		groups := make([]string, 0, len(mk.APIGroups))
		resources := make([]string, 0, len(mk.Kinds))

		for k := range uniqGroups {
			groups = append(groups, k)
		}

		for k := range uniqResources {
			resources = append(resources, k)
		}

		res = append(res, matchResource{
			APIGroups: groups,
			Resources: resources,
		})
	}

	input.Values.Set("admissionPolicyEngine.internal.trackedResources", res)

	return nil
}

func filterExporterCM(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap

	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return nil, err
	}

	return cm.Data["validate-kinds.yaml"], nil
}

type matchKind struct {
	APIGroups []string `json:"apiGroups"`
	Kinds     []string `json:"kinds"`
}

type matchResource struct {
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
}
