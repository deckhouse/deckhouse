/*
Copyright 2026 Flant JSC

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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func applyIngressFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingress",
			ApiVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubernetes-api"},
			},
			FilterFunc: applyIngressFilter,
		},
	},
}, discoverIngress)

func discoverIngress(_ context.Context, input *go_hook.HookInput) error {
	const (
		publishAPIEnabled = "userAuthn.internal.publishAPIEnabled"
	)
	fmt.Println(input.Snapshots.Get("ingress"))
	if len(input.Snapshots.Get("ingress")) == 0 {
		fmt.Println("Set publish api internal value to false")
		input.Values.Set(publishAPIEnabled, false)
	} else {
		fmt.Println("Set publish api internal value to true")
		input.Values.Set(publishAPIEnabled, true)
	}
	return nil
}
