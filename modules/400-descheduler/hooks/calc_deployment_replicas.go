/*
Copyright 2021 Flant CJSC

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Queue:        "/modules/descheduler",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodesFilter,
		},
	},
}, nodesCount)

func applyNodesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func nodesCount(input *go_hook.HookInput) error {
	nodes := input.Snapshots["nodes"]

	if len(nodes) > 1 {
		input.Values.Set("descheduler.internal.replicas", 1)
	} else {
		input.Values.Set("descheduler.internal.replicas", 0)
	}

	return nil
}
