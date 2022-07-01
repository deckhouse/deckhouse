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
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	carSnapshot = "cluster_authorization_rules"
	nsSnapshot  = "namespaces"
)

func applyClusterAuthorizationRuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var car internal.ClusterAuthorizationRule

	err := sdk.FromUnstructured(obj, &car)
	if err != nil {
		return nil, err
	}
	return &car, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue(carSnapshot),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       carSnapshot,
			ApiVersion: "deckhouse.io/v1",
			Kind:       "ClusterAuthorizationRule",
			FilterFunc: applyClusterAuthorizationRuleFilter,
		},
		{
			// need for multitenancy rules
			Name:       nsSnapshot,
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: filterNS,
		},
	},
}, clusterAuthorizationRulesHandler)

func filterNS(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func clusterAuthorizationRulesHandler(input *go_hook.HookInput) error {
	simpleCARs := make([]internal.ValuesClusterAuthorizationRule, 0)
	multitenancyCARS := make([]internal.ValuesClusterAuthorizationRule, 0)

	snapshots := input.Snapshots[nsSnapshot]
	allNamespaces := make([]string, 0, len(snapshots))
	for _, ns := range snapshots {
		allNamespaces = append(allNamespaces, ns.(string))
	}

	snapshots = input.Snapshots[carSnapshot]
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		car := snapshot.(*internal.ClusterAuthorizationRule)
		if car.IsMultitenancy() {
			multiRule := processMultitenancyCAR(input, car, allNamespaces)
			multitenancyCARS = append(multitenancyCARS, multiRule)
		} else {
			simpleCARs = append(simpleCARs, car.ToValues())
		}
	}

	input.Values.Set("userAuthz.internal.crds", simpleCARs)
	input.Values.Set("userAuthz.internal.multitenancyCRDs", multitenancyCARS)

	return nil
}

func processMultitenancyCAR(input *go_hook.HookInput, rule *internal.ClusterAuthorizationRule, namespaces []string) internal.ValuesClusterAuthorizationRule {
	valuesRule := rule.ToValues()

	nsRegexps := make([]string, 0)
	if len(rule.Spec.LimitNamespaces) > 0 {
		nsRegexps = append(nsRegexps, rule.Spec.LimitNamespaces...)
	}

	if rule.Spec.AllowAccessToSystemNamespaces {
		nsRegexps = append(nsRegexps, systemNSRegexp...)
	}

	if len(nsRegexps) == 0 {
		return valuesRule
	}

	calculatedNamespaces := make([]string, 0)

	for _, regns := range nsRegexps {
		reg, err := regexp.Compile(regns)
		if err != nil {
			input.LogEntry.Warnf("compile NS regexp failed: %s", err)
			continue
		}

		for _, ns := range namespaces {
			if reg.MatchString(ns) {
				calculatedNamespaces = append(calculatedNamespaces, ns)
			}
		}
	}

	valuesRule.Spec.LimitNamespaces = calculatedNamespaces

	return valuesRule
}

var (
	systemNSRegexp = []string{"kube-.*", "d8-.*", "loghouse", "default"}
)
