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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

type serviceInfo struct {
	Name      string
	Namespace string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "service_helm_fix",
			ApiVersion: "v1",
			Kind:       "Service",
			FilterFunc: applyServiceFilterHelmFix,
			NameSelector: &types.NameSelector{
				MatchNames: []string{"memcached"},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "migration.deckhouse.io/fix-services-broken-by-helm",
						Operator: v1.LabelSelectorOpNotIn,
						Values:   []string{"done"},
					},
				},
			},
			NamespaceSelector: lib.NsSelector(),
		},
	},
}, patchServiceWithManyPorts)

func applyServiceFilterHelmFix(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return serviceInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}, nil
}

func patchServiceWithManyPorts(input *go_hook.HookInput) error {
	serviceSnapshots := input.Snapshots["service_helm_fix"]
	for _, serviceSnapshot := range serviceSnapshots {
		serviceInfoObj := serviceSnapshot.(serviceInfo)
		input.PatchCollector.Delete(
			"v1",
			"Service",
			serviceInfoObj.Name,
			serviceInfoObj.Namespace,
			object_patch.InForeground(),
		)
	}
	return nil
}
