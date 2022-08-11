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
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

// this hook is needed only for release 1.34.12
// after that release bashible-apiserver is updated and works without this

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/remove_bashible_apiserver",
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 5,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "bashible-apiserver-remove",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{bashibleNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{bashibleName},
			},
			ExecuteHookOnEvents: pointer.BoolPtr(false),
			FilterFunc:          bashibleDeploymentFilter,
		},
	},
}, removeBashibleHandler)

func removeBashibleHandler(input *go_hook.HookInput) error {
	snap := input.Snapshots["bashible-apiserver-remove"]
	if len(snap) == 0 {
		return nil
	}

	dep := snap[0].(migrateBashibleDeployment)

	if dep.NeedReboot {
		input.PatchCollector.Delete("apps/v1", "Deployment", dep.Namespace, dep.Name)
	}

	return nil
}

func bashibleDeploymentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var dep v1.Deployment
	err := sdk.FromUnstructured(obj, &dep)
	if err != nil {
		return nil, err
	}

	needReboot := true
	if v, ok := dep.Annotations["node.deckhouse.io/migrated"]; ok {
		if v == "1.34" {
			needReboot = false
		}
	}

	return migrateBashibleDeployment{
		Namespace:  dep.Namespace,
		Name:       dep.Name,
		NeedReboot: needReboot,
	}, nil
}

type migrateBashibleDeployment struct {
	Namespace  string
	Name       string
	NeedReboot bool
}
