/*
Copyright 2021 Flant JSC

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
)

/* Migration hook: delete it after deploying to all clusters */

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, addDexAuthenticatorHelmLabelsToSecret)

func addDexAuthenticatorHelmLabelsToSecret(input *go_hook.HookInput) error {
	patchAnnotations(input, "dashboard", "d8-dashboard", "dashboard")
	patchAnnotations(input, "grafana", "d8-monitoring", "prometheus")
	patchAnnotations(input, "deckhouse-web", "d8-system", "deckhouse-web")
	patchAnnotations(input, "upmeter", "d8-upmeter", "upmeter")
	patchAnnotations(input, "status", "d8-upmeter", "upmeter")
	patchAnnotations(input, "openvpn", "d8-upmeter", "openvpn")
	patchAnnotations(input, "istio", "d8-istio", "istio")

	return nil
}

func patchAnnotations(input *go_hook.HookInput, name, namespace, moduleName string) {
	/*
	  annotations:
	    meta.helm.sh/release-name: prometheus
	    meta.helm.sh/release-namespace: d8-system
	  labels:
	    app.kubernetes.io/managed-by: Helm
	*/
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"meta.helm.sh/release-name":      moduleName,
				"meta.helm.sh/release-namespace": "d8-system",
			},
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": "Helm",
			},
		},
	}
	input.PatchCollector.MergePatch(patch, "deckhouse.io/v1", "DexAuthenticator", namespace, name, object_patch.IgnoreMissingObject())
}
