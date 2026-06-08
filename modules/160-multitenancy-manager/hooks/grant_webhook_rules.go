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
	"github.com/deckhouse/module-sdk/pkg/utils/ptr"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// filterRegistrations is the snapshot filter for ClusterGrantableResource objects.
func filterRegistrations(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj, nil
}

// grantableWebhookRules derives the admission webhook rules from the registered, Managed-enforcement
// ClusterGrantableResources: one CREATE/UPDATE rule per (group, resource) of their usageReferences'
// rules. Versions are matched with "*" (the controller selects the right path per version). Snapshots
// must be collected under the name "registrations".
func grantableWebhookRules(input *go_hook.HookInput) []admissionregistrationv1.RuleWithOperations {
	rules := make([]admissionregistrationv1.RuleWithOperations, 0)
	seen := make(map[string]struct{})

	for _, snap := range input.Snapshots.Get("registrations") {
		reg := &unstructured.Unstructured{}
		if err := snap.UnmarshalTo(reg); err != nil {
			continue
		}
		// External-enforcement resources are not intercepted by our webhooks.
		if enforcement, _, _ := unstructured.NestedString(reg.Object, "spec", "enforcement"); enforcement == "External" {
			continue
		}
		refs, found, _ := unstructured.NestedSlice(reg.Object, "spec", "usageReferences")
		if !found {
			continue
		}
		for _, ref := range refs {
			refMap, ok := ref.(map[string]any)
			if !ok {
				continue
			}
			rule, ok := refMap["rule"].(map[string]any)
			if !ok {
				continue
			}
			for _, g := range toStringSlice(rule["apiGroups"]) {
				if g == "*" {
					// A wildcard-group rule would intercept everything; rely on the in-handler
					// check instead and skip it from the static webhook rules.
					continue
				}
				for _, res := range toStringSlice(rule["resources"]) {
					key := g + "/" + res
					if _, dup := seen[key]; dup {
						continue
					}
					seen[key] = struct{}{}
					rules = append(rules, admissionregistrationv1.RuleWithOperations{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{g},
							APIVersions: []string{"*"},
							Resources:   []string{res},
							Scope:       ptr.To(admissionregistrationv1.NamespacedScope),
						},
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create, admissionregistrationv1.Update,
						},
					})
				}
			}
		}
	}
	return rules
}

func toStringSlice(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
