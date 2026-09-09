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

// This hook reports ClusterAuthorizationRules whose multi-tenancy options cannot take effect,
// because userAuthz.enableMultiTenancy is off and the authorization webhook that enforces them is
// therefore not deployed. Such a rule is not partially applied: it is ignored, and its subjects
// hold the rule's access level in every namespace.
//
// This used to be a `fail` in the ConfigMap template, which stopped the entire module from
// rendering. That is a poor answer to somebody creating a resource - one rule freezes every other
// change to the module, and the message reaches only whoever reads the release logs. It is a metric
// now, so it becomes an alert and nothing else stops.

package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	multitenancySnapshot   = "cluster_authorization_rules_multitenancy"
	multitenancyRuleMetric = "d8_user_authz_rule_needs_multitenancy"
	multitenancySumMetric  = "d8_user_authz_rules_needing_multitenancy"
	// A cluster can hold thousands of rules. Name the first few so the alert is actionable, and let
	// the aggregate carry the rest.
	multitenancyMaxNamed = 50
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       multitenancySnapshot,
			ApiVersion: "deckhouse.io/v1",
			Kind:       "ClusterAuthorizationRule",
			FilterFunc: applyMultitenancyOptionsFilter,
		},
	},
}, handleRulesNeedingMultitenancy)

// MultitenancyRule is a rule and the options of it that only the webhook can honour.
type MultitenancyRule struct {
	Name    string
	Options []string
}

func applyMultitenancyOptionsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	rule := MultitenancyRule{Name: obj.GetName()}

	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("read the spec of ClusterAuthorizationRule %q: %w", obj.GetName(), err)
	}
	if !found {
		return rule, nil
	}

	// Exactly the three the webhook is responsible for. Everything else in a rule is RBAC, which
	// user-authz-controller applies whether or not multi-tenancy is on.
	//
	// The value matters, not the key. A rule that spells out `allowAccessToSystemNamespaces: false`
	// or that has had its `limitNamespaces` emptied asks for nothing the webhook would enforce, and
	// reporting it would send an operator looking for a problem that is not there.
	if namespaces, ok := spec["limitNamespaces"].([]interface{}); ok && len(namespaces) > 0 {
		rule.Options = append(rule.Options, "limitNamespaces")
	}
	if selector, ok := spec["namespaceSelector"].(map[string]interface{}); ok && len(selector) > 0 {
		rule.Options = append(rule.Options, "namespaceSelector")
	}
	if allow, ok := spec["allowAccessToSystemNamespaces"].(bool); ok && allow {
		rule.Options = append(rule.Options, "allowAccessToSystemNamespaces")
	}
	return rule, nil
}

func handleRulesNeedingMultitenancy(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(multitenancyRuleMetric)

	if input.Values.Get("userAuthz.enableMultiTenancy").Bool() {
		input.MetricsCollector.Set(multitenancySumMetric, 0, nil, metrics.WithGroup(multitenancyRuleMetric))
		return nil
	}

	affected := 0
	for rule, err := range sdkobjectpatch.SnapshotIter[MultitenancyRule](input.Snapshots.Get(multitenancySnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over %q snapshot: %w", multitenancySnapshot, err)
		}
		if len(rule.Options) == 0 {
			continue
		}
		affected++
		if affected <= multitenancyMaxNamed {
			input.MetricsCollector.Set(multitenancyRuleMetric, 1, map[string]string{
				"name":    rule.Name,
				"options": strings.Join(rule.Options, ","),
			}, metrics.WithGroup(multitenancyRuleMetric))
		}
	}

	input.MetricsCollector.Set(multitenancySumMetric, float64(affected), nil, metrics.WithGroup(multitenancyRuleMetric))
	return nil
}
