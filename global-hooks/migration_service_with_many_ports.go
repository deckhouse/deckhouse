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

	"github.com/deckhouse/deckhouse/go_lib/set"
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
	{
		module:    "kube-dns",
		namespace: "kube-system",
		name:      "d8-kube-dns-redirect",
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
	{
		module:    "delivery",
		namespace: "d8-delivery",
		name:      "argocd-repo-server",
		patch: map[string]interface{}{
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"name":       "server",
						"port":       8081,
						"targetPort": "server",
						"protocol":   "TCP",
					},
					map[string]interface{}{
						"name":       "metrics",
						"port":       8084,
						"targetPort": "metrics",
						"protocol":   "TCP",
					},
				},
			},
		},
	},
	{
		module:    "delivery",
		namespace: "d8-delivery",
		name:      "argocd-server",
		patch: map[string]interface{}{
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"name":       "http",
						"port":       80,
						"targetPort": "server",
						"protocol":   "TCP",
					},
					map[string]interface{}{
						"name":       "https",
						"port":       443,
						"targetPort": "server",
						"protocol":   "TCP",
					},
				},
			},
		},
	},
	{
		module:    "deckhouse",
		namespace: "d8-system",
		name:      "deckhouse-leader",
		patch: map[string]interface{}{
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"name":       "self",
						"port":       8080,
						"targetPort": "self",
						"protocol":   "TCP",
					},
					map[string]interface{}{
						"name":       "webhook",
						"port":       4223,
						"targetPort": "webhook",
						"protocol":   "TCP",
					},
				},
			},
		},
	},
	{
		module:    "deckhouse",
		namespace: "d8-system",
		name:      "deckhouse",
		patch: map[string]interface{}{
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"name":       "self",
						"port":       8080,
						"targetPort": "self",
						"protocol":   "TCP",
					},
					map[string]interface{}{
						"name":       "webhook",
						"port":       4223,
						"targetPort": "webhook",
						"protocol":   "TCP",
					},
				},
			},
		},
	},
	{
		module:    "istio",
		namespace: "d8-istio",
		name:      "kiali",
		patch: map[string]interface{}{
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"name":       "http",
						"port":       20001,
						"targetPort": "api",
						"protocol":   "TCP",
					},
					map[string]interface{}{
						"name":       "http-metrics",
						"port":       9090,
						"targetPort": "http-metrics",
						"protocol":   "TCP",
					},
				},
			},
		},
	},
	{
		module:    "prometheus",
		namespace: "d8-monitoring",
		name:      "memcached",
		patch: map[string]interface{}{
			"spec": map[string]interface{}{
				"ports": []interface{}{
					map[string]interface{}{
						"name":       "memcached",
						"port":       11211,
						"targetPort": "memcached",
						"protocol":   "TCP",
					},
					map[string]interface{}{
						"name":       "http-metrics",
						"port":       9150,
						"targetPort": "http-metrics",
						"protocol":   "TCP",
					},
				},
			},
		},
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterAll: &go_hook.OrderedConfig{Order: 20},
}, patchServiceWithManyPorts)

func patchServiceWithManyPorts(input *go_hook.HookInput) error {
	for _, service := range patchServiceData {
		enabledModules := set.NewFromValues(input.Values, "global.enabledModules")
		if !enabledModules.Has(service.module) {
			continue
		}

		input.PatchCollector.MergePatch(
			service.patch,
			"v1",
			"Service",
			service.namespace,
			service.name,
		)
	}
	return nil
}
