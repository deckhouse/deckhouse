/*
Copyright 2023 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const pssEnforcementActionLabel = "security.deckhouse.io/pod-policy-action"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pss_enforcement_actions",
			ApiVersion: "",
			Kind:       "Namespace",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      pssEnforcementActionLabel,
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"Deny",
							"deny",
							"Dryrun",
							"dryrun",
							"Warn",
							"warn",
						},
					},
				},
			},
			FilterFunc: filterNamespaces,
		},
	},
}, handleActions)

func handleActions(input *go_hook.HookInput) error {
	actions := []string{strings.ToLower(input.Values.Get("admissionPolicyEngine.podSecurityStandards.enforcementAction").String())}
	labels := input.Snapshots["pss_enforcement_actions"]

	for _, label := range labels {
		lbl := strings.ToLower(label.(string))
		if !hasItem(actions, lbl) {
			actions = append(actions, lbl)
			// all possible actions were found, it doesn't make sense to proceed
			if len(actions) == 3 {
				break
			}
		}
	}
	input.Values.Set("admissionPolicyEngine.internal.podSecurityStandards.enforcementActions", actions)

	return nil
}

func filterNamespaces(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	action, _, err := unstructured.NestedString(obj.Object, "metadata", "labels", pssEnforcementActionLabel)
	if err != nil {
		return nil, err
	}

	return action, nil
}
