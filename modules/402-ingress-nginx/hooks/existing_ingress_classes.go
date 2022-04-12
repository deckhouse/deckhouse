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
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "external-ingress-class",
			ApiVersion: "networking.k8s.io/v1",
			Kind:       "IngressClass",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"ingress-class.deckhouse.io/external": "true",
				},
			},
			FilterFunc: filterIngressClass,
		},
	},
}, handleExternalIngressClasses)

func filterIngressClass(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ic v1.IngressClass

	err := sdk.FromUnstructured(obj, &ic)
	if err != nil {
		return nil, err
	}

	return ic.Name, nil
}

func handleExternalIngressClasses(input *go_hook.HookInput) error {
	snap := input.Snapshots["external-ingress-class"]

	externalIngressClasses := make([]string, 0, len(snap))

	for _, sn := range snap {
		externalIngressClasses = append(externalIngressClasses, sn.(string))
	}

	input.Values.Set("ingressNginx.internal.externalIngressClasses", externalIngressClasses)

	return nil
}
