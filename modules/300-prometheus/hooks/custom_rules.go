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
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type CustomRule struct {
	Name   string
	Groups []interface{}
}

func filterCustomRule(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cr := new(CustomRule)
	cr.Name = obj.GetName()

	groupsRaw, ok, err := unstructured.NestedSlice(obj.Object, "spec", "groups")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("no groups field")
	}

	cr.Groups = append(cr.Groups, groupsRaw...)
	return cr, nil
}

func filterInternalRule(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/custom_prometheus_rules",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "rules",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "CustomPrometheusRules",
			FilterFunc: filterCustomRule,
		},
		{
			Name:       "internal_rules",
			ApiVersion: "monitoring.coreos.com/v1",
			Kind:       "PrometheusRule",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"module":     "prometheus",
					"heritage":   "deckhouse",
					"app":        "prometheus",
					"prometheus": "main",
					"component":  "rules",
					"origin":     "custom",
				},
			},
			FilterFunc: filterInternalRule,
		},
	},
}, customRulesHandler)

func customRulesHandler(_ context.Context, input *go_hook.HookInput) error {
	tmpMap := make(map[string]bool)

	rulesSnap := input.Snapshots.Get("rules")

	for rule, err := range sdkobjectpatch.SnapshotIter[CustomRule](rulesSnap) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'rules' snapshot: %v", err)
		}

		internalRule := createPrometheusRule(rule.Name, rule.Groups)
		input.PatchCollector.CreateOrUpdate(&internalRule)

		tmpMap[internalRule.GetName()] = true
	}

	internalRulesSnap := input.Snapshots.Get("internal_rules")

	// delete absent prometheus rules
	for internalRuleName, err := range sdkobjectpatch.SnapshotIter[string](internalRulesSnap) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'internal_rules' snapshot: %v", err)
		}

		if _, ok := tmpMap[internalRuleName]; !ok {
			input.PatchCollector.Delete("monitoring.coreos.com/v1", "PrometheusRule", "d8-monitoring", internalRuleName)
		}
	}

	return nil
}

func createPrometheusRule(name string, groups []interface{}) unstructured.Unstructured {
	customName := fmt.Sprintf("d8-custom-%s", name)

	un := unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "monitoring.coreos.com/v1",
		"kind":       "PrometheusRule",
		"metadata": map[string]interface{}{
			"name":      customName,
			"namespace": "d8-monitoring",
			"labels": map[string]interface{}{
				"module":     "prometheus",
				"heritage":   "deckhouse",
				"app":        "prometheus",
				"prometheus": "main",
				"component":  "rules",
				"origin":     "custom",
			},
		},
		"spec": map[string]interface{}{
			"groups": groups,
		},
	}}

	return un
}
