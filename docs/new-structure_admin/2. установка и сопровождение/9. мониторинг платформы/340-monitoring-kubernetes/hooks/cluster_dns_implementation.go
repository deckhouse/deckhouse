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
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

func nameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/monitoring-kubernetes",
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 10.0,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kube_dns_deployment",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{
						"kube-system",
					},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "k8s-app",
						Operator: v1.LabelSelectorOpIn,
						Values: []string{
							"kube-dns",
							"coredns",
						},
					},
				},
			},
			FilterFunc: nameFilter,
		},
	},
}, setDNSImplementation)

func setDNSImplementation(input *go_hook.HookInput) error {
	enabledModules := set.NewFromValues(input.Values, "global.enabledModules")

	if enabledModules.Has("kube-dns") {
		input.Values.Set("monitoringKubernetes.internal.clusterDNSImplementation", "coredns")
		return nil
	}

	kubeDNSDeployments := input.Snapshots["kube_dns_deployment"]
	if len(kubeDNSDeployments) != 1 {
		return errors.New("ERROR: can't determine cluster DNS implementation")
	}

	input.Values.Set("monitoringKubernetes.internal.clusterDNSImplementation", kubeDNSDeployments[0].(string))
	return nil
}
