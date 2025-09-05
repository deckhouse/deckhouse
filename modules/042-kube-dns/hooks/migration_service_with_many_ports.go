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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

type patchServiceStruct struct {
	module    string
	namespace string
	name      string
	patch     map[string]interface{}
}

var patchServiceData = []patchServiceStruct{
	{
		module:    "kube-dns",
		namespace: "kube-system",
		name:      "d8-kube-dns",
		patch: map[string]interface{}{
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"name":       "dns",
						"port":       53,
						"targetPort": "dns",
						"protocol":   "UDP",
					},
					map[string]interface{}{
						"name":       "dns-tcp",
						"port":       53,
						"targetPort": "dns-tcp",
						"protocol":   "TCP",
					},
				},
			},
		},
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 1},
}, patchServiceWithManyPorts)

func patchServiceWithManyPorts(_ context.Context, input *go_hook.HookInput) error {
	for _, service := range patchServiceData {
		input.PatchCollector.PatchWithMerge(
			service.patch,
			"v1",
			"Service",
			service.namespace,
			service.name,
		)
	}
	return nil
}
