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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

// There is CNIPlugin trafficRedirectionSetupMode in Istio module by default
// To change this mode to InitContainer we should create secret
// d8-istio-configuration in d8-istio namespace with trafficRedirectionSetupMode key
// $ kubectl -n d8-istio create secret generic d8-istio-configuration --from-literal=trafficRedirectionSetupMode=InitContainer
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        lib.Queue("istio-cni"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "istio-cni",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-istio-configuration"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-istio"},
				},
			},
			FilterFunc: applyDiscoveryIstioCniModeFilter,
		},
	},
}, setInternalIstioCniMode)

func applyDiscoveryIstioCniModeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return false, fmt.Errorf("cannot convert secret to struct: %v", err)
	}

	mode, ok := secret.Data["trafficRedirectionSetupMode"]
	if !ok {
		return "", fmt.Errorf("cannot get `trafficRedirectionSetupMode` key from Secret %s", secret.Name)
	}
	return string(mode), nil
}

func setInternalIstioCniMode(input *go_hook.HookInput) error {
	snapshots := input.Snapshots["istio-cni"]

	if len(snapshots) == 1 && snapshots[0].(string) != "" {
		input.Values.Set("istio.internal.dataPlane.trafficRedirectionSetupMode", snapshots[0].(string))
		return nil
	}
	input.Values.Set("istio.internal.dataPlane.trafficRedirectionSetupMode", "CNIPlugin")
	return nil
}
