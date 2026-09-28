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

// Emits a metric for every (Cluster)RoleBinding whose roleRef still points at a DEPRECATED RBACv2
// role name (d8:manage:* / d8:use:role:*). The RBACv2 role model was renamed; the old names are kept
// alive for one release by the aliases in templates/rbacv2-compat/. This metric drives the
// D8UserAuthzDeprecatedRBACv2RoleInUse alert that nudges operators to migrate their bindings to the
// new names (d8:{system,subsystem,namespace,project}:*) before the aliases are removed next release.
//
// The bindings to an aliased name are published as a release requirement value: the release that
// removes the aliases carries the deprecatedRBACv2BindingsCount requirement
// (modules/140-user-authz/requirements) and stays Pending while such a binding is still in the
// cluster, so nobody loses access by upgrading with it in place. A binding to a capability name has no
// alias to lose -- removing the aliases changes nothing for it -- so it raises its alert but does not
// hold the release.
//
// A module that renames a role of its own keeps the old name as an alias the same way, in its own
// templates/rbacv2-compat/: a ClusterRole labelled rbac.deckhouse.io/deprecated with the new name in
// rbac.deckhouse.io/deprecated-replaced-by (state-snapshotter keeps
// d8:use:capability:module:state-snapshotter:backup_agent for its backup agent role). A binding to such
// an alias still grants access, whatever d8: name the alias has outside the platform's own role
// families (d8:system:, d8:subsystem:, d8:namespace:, d8:project:, d8:custom:, d8:dict, and the
// d8:binding: / d8:prb: names); the module, not a release of the platform, removes the alias, so the
// binding raises its own alert and does not hold the release. An alias without the replaced-by
// annotation is ignored with a warning: the alert would have nothing to suggest.

package hooks

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	deprecatedRBACv2Metric = "d8_rbacv2_deprecated_role_in_use"

	// DeprecatedRBACv2BindingsValueKey is the requirements memory-storage key holding the sorted
	// descriptions of the bindings that still reference a deprecated RBACv2 role name. The
	// requirement check function (modules/140-user-authz/requirements) reads it when a release
	// carries the deprecatedRBACv2BindingsCount requirement.
	DeprecatedRBACv2BindingsValueKey = "userAuthz:deprecatedRBACv2Bindings"

	deprecatedLabel      = "rbac.deckhouse.io/deprecated"
	replacedByAnnotation = "rbac.deckhouse.io/deprecated-replaced-by"

	// Values of the aliased label of the metric.
	aliasedByPlatform = "true"
	notAliased        = "false"
	aliasedByModule   = "module"
)

// Legacy RBACv2 name families replaced by the new model. Two tiers matter for the operator:
//
//   - ALIASED (roles): d8:manage:<all|subsystem>:<level> and d8:use:role:<level>[:kubernetes] are kept
//     alive for one release by the compat aliases (templates/rbacv2-compat/). A binding to these
//     still authorizes the *new* role's capabilities (admin lost token/impersonate; manage CRBs no
//     longer drive automated namespace RoleBindings). The alert nudges the operator to migrate
//     before the aliases are removed.
//
//   - NOT ALIASED (capabilities): d8:manage:permission:* and d8:use:capability:* are aggregation
//     building blocks that were never meant to be bound directly and have NO compat alias. A binding
//     to these no longer grants access after the upgrade — the alert flags it as needing an immediate
//     fix. (d8:manage:permission:* is a subset of the d8:manage: prefix, so it is matched there and
//     then reclassified as a capability below.)
//
// d8:use:dict is intentionally absent: user-authz-controller migrates its bindings to d8:dict.
var deprecatedRoleNamePrefixes = []string{
	"d8:manage:",
	"d8:use:role:",
	"d8:use:capability:",
}

// platformRoleNamePrefixes are the role families of the 1.78 model and the other names the platform
// binds in bulk. A module never keeps an alias under them, so their bindings are not kept in the
// snapshot: in a large cluster they are most of the bindings.
var platformRoleNamePrefixes = []string{
	"d8:system:",
	"d8:subsystem:",
	"d8:namespace:",
	"d8:project:",
	"d8:custom:",
	"d8:dict",
	"d8:binding:",
	"d8:prb:",
}

// capabilityNamePrefixes are the deprecated names that are capabilities (no compat alias). A binding
// to one of these does NOT keep working after the upgrade.
var capabilityNamePrefixes = []string{
	"d8:manage:permission:",
	"d8:use:capability:",
}

// aliasedForOneRelease reports whether a deprecated role name is kept working by a compat alias (a
// role) as opposed to a capability that has no alias and stops granting access after the upgrade.
func aliasedForOneRelease(name string) bool {
	for _, prefix := range capabilityNamePrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authz/deprecated-rbacv2-bindings",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deprecated_clusterrolebindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
			FilterFunc: filterDeprecatedClusterRoleBinding,
		},
		{
			Name:       "deprecated_rolebindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
			FilterFunc: filterDeprecatedRoleBinding,
		},
		{
			// The aliases modules keep for their renamed roles; the platform's own aliases are
			// user-authz's and are told apart by name above.
			Name:       "module_rbacv2_aliases",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{deprecatedLabel: "true", "heritage": "deckhouse"},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "module", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"user-authz"}},
				},
			},
			FilterFunc: filterModuleAlias,
		},
	},
}, handleDeprecatedRBACv2Bindings)

// moduleAlias is an alias a module keeps for the old name of a role of its own.
type moduleAlias struct {
	Name       string `json:"name"`
	ReplacedBy string `json:"replaced_by"`
}

func filterModuleAlias(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return &moduleAlias{Name: obj.GetName(), ReplacedBy: obj.GetAnnotations()[replacedByAnnotation]}, nil
}

// deprecatedBinding is the minimal projection of a binding that still references a deprecated role.
// The FilterFunc returns nil for every other binding, so the snapshot only holds the offenders — the
// hook never keeps the full set of cluster bindings in memory.
type deprecatedBinding struct {
	BindingKind string `json:"binding_kind"`
	BindingName string `json:"binding_name"`
	Namespace   string `json:"namespace"`
	RoleName    string `json:"role_name"`
	// Aliased is "true" when the referenced name is still served by a one-release compat alias (a
	// role) and "false" when it is a capability with no alias (the binding no longer grants access).
	// The hook sets "module" when a module keeps the name as an alias of a role of its own. It is
	// empty for a name outside the legacy families, which only a module alias can make deprecated.
	Aliased string `json:"aliased"`
}

func deprecatedRoleName(name string) bool {
	for _, prefix := range deprecatedRoleNamePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// moduleAliasCandidate reports whether name could be an alias a module keeps for a role of its own:
// any d8: name outside the platform's role families. Whether it is one is known only from the alias
// snapshot, which a filter does not see.
func moduleAliasCandidate(name string) bool {
	if !strings.HasPrefix(name, "d8:") {
		return false
	}
	for _, prefix := range platformRoleNamePrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

// watchedRoleName reports whether a binding to name belongs in the snapshot: a legacy name, or a name a
// module alias may carry.
func watchedRoleName(name string) bool {
	return deprecatedRoleName(name) || moduleAliasCandidate(name)
}

// aliasedLabel classifies a binding target: a legacy name by its family, any other candidate is left
// for the hook to match against the module aliases.
func aliasedLabel(name string) string {
	if deprecatedRoleName(name) {
		return boolLabel(aliasedForOneRelease(name))
	}
	return ""
}

func filterDeprecatedClusterRoleBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	binding := new(rbacv1.ClusterRoleBinding)
	if err := sdk.FromUnstructured(obj, binding); err != nil {
		return nil, err
	}
	if binding.RoleRef.Kind != "ClusterRole" || !watchedRoleName(binding.RoleRef.Name) {
		return nil, nil
	}
	return &deprecatedBinding{
		BindingKind: "ClusterRoleBinding",
		BindingName: binding.Name,
		RoleName:    binding.RoleRef.Name,
		Aliased:     aliasedLabel(binding.RoleRef.Name),
	}, nil
}

func filterDeprecatedRoleBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	binding := new(rbacv1.RoleBinding)
	if err := sdk.FromUnstructured(obj, binding); err != nil {
		return nil, err
	}
	if binding.RoleRef.Kind != "ClusterRole" || !watchedRoleName(binding.RoleRef.Name) {
		return nil, nil
	}
	return &deprecatedBinding{
		BindingKind: "RoleBinding",
		BindingName: binding.Name,
		Namespace:   binding.Namespace,
		RoleName:    binding.RoleRef.Name,
		Aliased:     aliasedLabel(binding.RoleRef.Name),
	}, nil
}

func boolLabel(v bool) string {
	if v {
		return aliasedByPlatform
	}
	return notAliased
}

// describe names a binding the way the requirement error shows it:
// "RoleBinding team-a/legacy-viewer -> d8:use:role:viewer".
func (b deprecatedBinding) describe() string {
	name := b.BindingName
	if b.Namespace != "" {
		name = b.Namespace + "/" + name
	}

	return b.BindingKind + " " + name + " -> " + b.RoleName
}

// handleDeprecatedRBACv2Bindings publishes the sorted list of offending bindings as a requirement
// value and one alert metric per binding on every synchronization/event, so the release unblocks
// itself (and the alert resolves) as soon as the operator recreates the bindings on the new names.
func handleDeprecatedRBACv2Bindings(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(deprecatedRBACv2Metric)

	replacedBy := map[string]string{}

	for alias, err := range sdkobjectpatch.SnapshotIter[moduleAlias](input.Snapshots.Get("module_rbacv2_aliases")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'module_rbacv2_aliases' snapshot: %w", err)
		}

		// Without the new name the alert could not say what to bind instead: such an alias is not
		// trusted, and a binding to it keeps the classification of its name.
		if alias.ReplacedBy == "" {
			input.Logger.Warn("ignoring an RBACv2 alias of a module without the replaced-by annotation",
				"cluster_role", alias.Name, "annotation", replacedByAnnotation)
			continue
		}
		replacedBy[alias.Name] = alias.ReplacedBy
	}

	described := make([]string, 0)

	for _, snapshotName := range []string{"deprecated_clusterrolebindings", "deprecated_rolebindings"} {
		for binding, err := range sdkobjectpatch.SnapshotIter[deprecatedBinding](input.Snapshots.Get(snapshotName)) {
			if err != nil {
				return fmt.Errorf("failed to iterate over '%s' snapshot: %w", snapshotName, err)
			}
			replacement, moduleAliased := replacedBy[binding.RoleName]
			switch {
			case moduleAliased:
				binding.Aliased = aliasedByModule
			case binding.Aliased == "":
				// A d8: name outside the legacy families that no module keeps as an alias: a
				// current role.
				continue
			}

			// Only a binding to an alias of the platform loses access when the platform removes them.
			if binding.Aliased == aliasedByPlatform {
				described = append(described, binding.describe())
			}

			input.MetricsCollector.Set(
				deprecatedRBACv2Metric, 1,
				map[string]string{
					"binding_kind": binding.BindingKind,
					"binding_name": binding.BindingName,
					"namespace":    binding.Namespace,
					"role_name":    binding.RoleName,
					"aliased":      binding.Aliased,
					"replaced_by":  replacement,
				},
				metrics.WithGroup(deprecatedRBACv2Metric),
			)
		}
	}

	slices.Sort(described)
	requirements.SaveValue(DeprecatedRBACv2BindingsValueKey, described)

	return nil
}
