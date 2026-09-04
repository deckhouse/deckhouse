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

package internal

import (
	"context"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	// AggregatedBindingKindLabel marks the binding that grants a rule the aggregated
	// custom ClusterRole of its access level (user-authz:<level>:custom).
	AggregatedBindingKindLabel = "user-authz.deckhouse.io/binding-kind"
	// AggregatedBindingKindValue is the value of AggregatedBindingKindLabel.
	AggregatedBindingKindValue = "aggregated-custom"

	accessLevelSuperAdmin = "SuperAdmin"
)

// accessLevelKebab mirrors the sprig `kebabcase` applied to .spec.accessLevel in
// templates/cluster-role-bindings.yaml, so the hook derives the very same object names.
var accessLevelKebab = map[string]string{
	"User":                "user",
	"PrivilegedUser":      "privileged-user",
	"Editor":              "editor",
	"Admin":               "admin",
	"ClusterEditor":       "cluster-editor",
	"ClusterAdmin":        "cluster-admin",
	accessLevelSuperAdmin: "super-admin",
}

type authorizationRule struct {
	Name      string                 `json:"name"`
	Spec      map[string]interface{} `json:"spec"`
	Namespace string                 `json:"namespace,omitempty"`
	// LegacyCustomRoleBindings tells the chart to keep rendering one binding per custom
	// ClusterRole for this rule. It stays true until the aggregated binding of the rule
	// exists in the cluster, so the per-role bindings are never removed before their
	// replacement is in place (the release engine may delete before it creates).
	LegacyCustomRoleBindings bool `json:"legacyCustomRoleBindings"`
}

type aggregatedBinding struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

func ApplyAuthorizationRuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if !found {
		return nil, fmt.Errorf(`".spec is not a map[string]interface{} or contains non-string values in the map: %s`, spew.Sdump(obj.Object))
	}
	if err != nil {
		return nil, err
	}

	car := &authorizationRule{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Spec:      spec,
	}

	return car, nil
}

// ApplyAggregatedBindingFilter keeps only the coordinates of a binding labeled as the
// aggregated custom binding of a rule.
func ApplyAggregatedBindingFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return &aggregatedBinding{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}, nil
}

// AuthorizationRulesHandler publishes the rules of snapshotKey into valuesPath and marks
// every rule whose aggregated custom binding (from bindingsSnapshotKey) is not yet present,
// so the chart keeps its per-role bindings for now.
func AuthorizationRulesHandler(valuesPath, snapshotKey, bindingsSnapshotKey string) func(_ context.Context, input *go_hook.HookInput) error {
	return func(_ context.Context, input *go_hook.HookInput) error {
		authorizationRules, err := snapshotsToAuthorizationRulesSlice(input.Snapshots.Get(snapshotKey))
		if err != nil {
			return fmt.Errorf("failed to convert '%s' snapshot to authorization rules: %w", snapshotKey, err)
		}

		bindings, err := snapshotsToAggregatedBindingsSet(input.Snapshots.Get(bindingsSnapshotKey))
		if err != nil {
			return fmt.Errorf("failed to convert '%s' snapshot to aggregated bindings: %w", bindingsSnapshotKey, err)
		}

		for i := range authorizationRules {
			authorizationRules[i].LegacyCustomRoleBindings = needsLegacyCustomRoleBindings(authorizationRules[i], bindings)
		}

		input.Values.Set(valuesPath, authorizationRules)

		return nil
	}
}

// needsLegacyCustomRoleBindings reports whether the per-role bindings of the rule must still
// be rendered: only rules with an access level get custom-role bindings at all, SuperAdmin never
// gets them, and for the rest the answer is "until the aggregated binding exists".
func needsLegacyCustomRoleBindings(rule authorizationRule, bindings map[string]struct{}) bool {
	level, _ := rule.Spec["accessLevel"].(string)
	if level == "" || level == accessLevelSuperAdmin {
		return false
	}

	kebab, known := accessLevelKebab[level]
	if !known {
		// An unknown level is rejected by the template anyway; keep the safe default.
		return true
	}

	_, present := bindings[bindingKey(rule.Namespace, fmt.Sprintf("user-authz:%s:%s:custom", rule.Name, kebab))]

	return !present
}

func bindingKey(namespace, name string) string {
	return namespace + "/" + name
}

func snapshotsToAuthorizationRulesSlice(snapshots []pkg.Snapshot) ([]authorizationRule, error) {
	ars := make([]authorizationRule, 0, len(snapshots))

	for ar, err := range sdkobjectpatch.SnapshotIter[authorizationRule](snapshots) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshot: %w", err)
		}

		ars = append(ars, ar)
	}

	return ars, nil
}

func snapshotsToAggregatedBindingsSet(snapshots []pkg.Snapshot) (map[string]struct{}, error) {
	bindings := make(map[string]struct{}, len(snapshots))

	for binding, err := range sdkobjectpatch.SnapshotIter[aggregatedBinding](snapshots) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over snapshot: %w", err)
		}

		bindings[bindingKey(binding.Namespace, binding.Name)] = struct{}{}
	}

	return bindings, nil
}
