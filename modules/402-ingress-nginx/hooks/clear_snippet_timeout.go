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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/ingress-nginx/set-auth-timeouts",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingresses",
			ApiVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
			FilterFunc: filterIngress,
		},
	},
}, clearSnippetTimeout)

type ingressSnapshot struct {
	name       string
	namespace  string
	annotation string
}

func filterIngress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ingress netv1.Ingress
	if err := sdk.FromUnstructured(obj, &ingress); err != nil {
		return nil, err
	}
	if ingress.Spec.IngressClassName != nil && *ingress.Spec.IngressClassName != "nginx" {
		return nil, nil
	}
	var found bool
	for annotation, val := range ingress.ObjectMeta.GetAnnotations() {
		if annotation == "nginx.ingress.kubernetes.io/auth-snippet" && strings.Contains(val, "proxy_connect_timeout") {
			found = true
			break
		}
	}
	if !found {
		return nil, nil
	}
	return &ingressSnapshot{
		name:       ingress.Name,
		namespace:  ingress.Namespace,
		annotation: ingress.Annotations["nginx.ingress.kubernetes.io/auth-snippet"],
	}, nil
}

func clearSnippetTimeout(input *go_hook.HookInput) error {
	for _, ingress := range input.Snapshots["ingresses"] {
		if ingress != nil {
			parsed := ingress.(*ingressSnapshot)
			annotationPatch := map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"nginx.ingress.kubernetes.io/auth-snippet": strings.Replace(parsed.annotation, "proxy_connect_timeout", "#proxy_connect_timeout", -1),
					},
				},
			}
			input.PatchCollector.MergePatch(
				annotationPatch,
				"networking.k8s.io/v1",
				"Ingress",
				parsed.namespace,
				parsed.name,
				object_patch.IgnoreMissingObject())
		}
	}
	return nil
}
