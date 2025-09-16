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
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const pssEnforcementActionLabel = "security.deckhouse.io/pod-policy-action"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "pss_enforcement_actions",
			ApiVersion: "v1",
			Kind:       "Namespace",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      pssEnforcementActionLabel,
						Operator: metav1.LabelSelectorOpIn,
						Values: []string{
							"deny",
							"dryrun",
							"warn",
						},
					},
				},
			},
			FilterFunc: filterNamespaces,
		},
	},
}, handleActions)

func actionCode(action string) float64 {
	switch action {
	case "deny":
		return 3
	case "warn":
		return 2
	case "dryrun":
		return 1
	default:
		return 0
	}
}

func handleActions(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_admission_policy_engine_pss_default_action")

	actions := []string{strings.ToLower(input.Values.Get("admissionPolicyEngine.podSecurityStandards.enforcementAction").String())}
	input.MetricsCollector.Set("d8_admission_policy_engine_pss_default_action", actionCode(actions[0]), map[string]string{}, metrics.WithGroup("d8_admission_policy_engine_pss_default_action"))

	labels, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "pss_enforcement_actions")
	if err != nil {
		return fmt.Errorf("failed to unmarshal pss_enforcement_actions snapshot: %w", err)
	}

	for _, label := range labels {
		lbl := strings.ToLower(label)
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
