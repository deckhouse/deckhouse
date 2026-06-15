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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg/utils/ptr"
)

// filterRegistrations is the snapshot filter for GrantableClusterResourceDefinition objects.
func filterRegistrations(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj, nil
}

// filterReferences is the snapshot filter for GrantableClusterResourceReference objects.
func filterReferences(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj, nil
}

// grantableWebhookRules derives the admission webhook rules from the registered
// GrantableClusterResourceReference paths: one CREATE/UPDATE rule per (group, resource) of their rule,
// but only for references whose target GrantableClusterResourceDefinition exists and is Managed.
// Versions are matched with "*" (the controller selects the right path per version). Snapshots must be
// collected under the names "registrations" (definitions) and "references".
func grantableWebhookRules(input *go_hook.HookInput) []admissionregistrationv1.RuleWithOperations {
	// Enforcement mode by definition name; absent ⇒ the reference is dangling and intercepts nothing.
	enforcement := make(map[string]string)
	for _, snap := range input.Snapshots.Get("registrations") {
		def := &unstructured.Unstructured{}
		if err := snap.UnmarshalTo(def); err != nil {
			continue
		}
		e, _, _ := unstructured.NestedString(def.Object, "spec", "enforcement")
		enforcement[def.GetName()] = e
	}

	rules := make([]admissionregistrationv1.RuleWithOperations, 0)
	seen := make(map[string]struct{})

	for _, snap := range input.Snapshots.Get("references") {
		ref := &unstructured.Unstructured{}
		if err := snap.UnmarshalTo(ref); err != nil {
			continue
		}
		defName, _, _ := unstructured.NestedString(ref.Object, "spec", "grantableClusterResourceName")
		// Skip dangling references and those pointing at External-enforcement definitions.
		if e, ok := enforcement[defName]; !ok || e == "External" {
			continue
		}
		rule, found, _ := unstructured.NestedMap(ref.Object, "spec", "rule")
		if !found {
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
