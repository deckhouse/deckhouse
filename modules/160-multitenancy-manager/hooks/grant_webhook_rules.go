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
	"cmp"
	"context"
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const grantWebhookRulesValuesPath = "multitenancyManager.internal.grantWebhookRules"

// grantWebhookRule is one (group, resource) the cluster-objects grant webhooks intercept.
// templates/cluster-objects-controller/grant-webhooks.yaml turns each into a CREATE/UPDATE rule
// on every version of the namespaced resource.
type grantWebhookRule struct {
	APIGroup string `json:"apiGroup"`
	Resource string `json:"resource"`
}

// The rules are values, not objects: Helm renders both webhook configurations, so they are part
// of the module release and go away with it when the module is disabled. Any values change made
// here, from this queue too, schedules a module run, and Helm applies the new rules. OnBeforeHelm
// recomputes them before every render, so the release never sees rules from an older snapshot.
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/160-multitenancy-manager",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "registrations",
			ApiVersion: "multitenancy.deckhouse.io/v1alpha1",
			Kind:       "GrantableClusterResourceDefinition",
			FilterFunc: filterRegistrations,
		},
		{
			Name:       "references",
			ApiVersion: "multitenancy.deckhouse.io/v1alpha1",
			Kind:       "GrantableClusterResourceReference",
			FilterFunc: filterReferences,
		},
	},
}, discoverGrantWebhookRules)

func discoverGrantWebhookRules(_ context.Context, input *go_hook.HookInput) error {
	input.Values.Set(grantWebhookRulesValuesPath, grantableWebhookRules(input))
	return nil
}

// filterRegistrations is the snapshot filter for GrantableClusterResourceDefinition objects.
func filterRegistrations(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj, nil
}

// filterReferences is the snapshot filter for GrantableClusterResourceReference objects.
func filterReferences(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj, nil
}

// grantableWebhookRules derives the admission webhook rules from the registered
// GrantableClusterResourceReference paths: one rule per (group, resource) of their rule, but only for
// references whose target GrantableClusterResourceDefinition exists and is Managed. Versions are
// matched with "*" in the template (the controller selects the right path per version). The result is
// sorted, so the values change only when the set of rules does.
func grantableWebhookRules(input *go_hook.HookInput) []grantWebhookRule {
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

	rules := make([]grantWebhookRule, 0)
	seen := make(map[grantWebhookRule]struct{})

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
				// The CRD refuses "*" in apiGroups now (a wildcard would silently intercept
				// nothing, see the schema description). This stays as the safety net for an
				// object stored before that rule existed: it is omitted from the static webhook
				// rules rather than turned into a catch-all on every namespaced object.
				continue
			}
			for _, res := range toStringSlice(rule["resources"]) {
				r := grantWebhookRule{APIGroup: g, Resource: res}
				if _, dup := seen[r]; dup {
					continue
				}
				seen[r] = struct{}{}
				rules = append(rules, r)
			}
		}
	}

	slices.SortFunc(rules, func(a, b grantWebhookRule) int {
		return cmp.Or(cmp.Compare(a.APIGroup, b.APIGroup), cmp.Compare(a.Resource, b.Resource))
	})
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
