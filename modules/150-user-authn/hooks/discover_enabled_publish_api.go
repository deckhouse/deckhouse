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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

func applyIngressFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ingress := &networkingv1.Ingress{}

	return ingress, nil
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

	ingressSnapshots, err := sdkobjectpatch.UnmarshalToStruct[networkingv1.Ingress](input.Snapshots, "ingress")
	if err != nil {
		return fmt.Errorf("cannot get publish API ingress from snaphots: failed to iterate over 'ingress' snapshot: %w", err)
	}

	if len(ingressSnapshots) > 0 {
		fmt.Println("Set publish api internal value to true")
		input.Values.Set(publishAPIEnabled, true)
	} else {
		fmt.Println("Set publish api internal value to false")
		input.Values.Set(publishAPIEnabled, false)
	}

	return nil
}
