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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/loki/tokens",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "grafana_token",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"d8-monitoring",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"prometheus-token",
			}},
			FilterFunc: filterTokenSecret,
		},
		{
			Name:       "log_shipper_token",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{
					"d8-log-shipper",
				}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{
				"log-shipper-token",
			}},
			FilterFunc: filterTokenSecret,
		},
	},
}, handleTokens)

func filterTokenSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(corev1.Secret)

	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	return string(secret.Data["token"]), nil
}

func handleTokens(input *go_hook.HookInput) error {
	grafanaTokenSnapshots := input.Snapshots["grafana_token"]

	if len(grafanaTokenSnapshots) > 0 {
		input.Values.Set("loki.internal.grafanaToken", grafanaTokenSnapshots[0].(string))
	}

	logShipperTokenSnapshots := input.Snapshots["log_shipper_token"]

	if len(logShipperTokenSnapshots) > 0 {
		input.Values.Set("loki.internal.logShipperToken", logShipperTokenSnapshots[0].(string))
	}

	return nil
}
