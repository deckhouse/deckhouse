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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "labeled_ingress",
			ApiVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"ingress.deckhouse.io/discard-metrics": "true",
				},
			},
			FilterFunc: nameFilter,
		},
		{
			Name:       "labeled_ns",
			ApiVersion: "v1",
			Kind:       "Namespace",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"ingress.deckhouse.io/discard-metrics": "true",
				},
			},
			FilterFunc: nameFilter,
		},
	},
}, handleExcludes)

func handleExcludes(input *go_hook.HookInput) error {
	nss := make([]string, 0)
	ings := make([]string, 0)

	snap := input.Snapshots["labeled_ingress"]
	for _, sn := range snap {
		res := sn.(discardedIngress)
		ings = append(ings, res.String())
	}

	snap = input.Snapshots["labeled_ns"]
	for _, sn := range snap {
		res := sn.(discardedIngress)
		nss = append(nss, res.Name)
	}

	input.Values.Set("ingressNginx.internal.discardMetricResources.namespaces", nss)
	input.Values.Set("ingressNginx.internal.discardMetricResources.ingresses", ings)

	return nil
}

func nameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return discardedIngress{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}, nil
}

type discardedIngress struct {
	Name      string
	Namespace string
}

func (di discardedIngress) String() string {
	return strings.Join([]string{di.Namespace, di.Name}, ":")
}
