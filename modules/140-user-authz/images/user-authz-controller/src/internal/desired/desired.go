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

// Package desired computes the (Cluster)RoleBindings a ClusterAuthorizationRule or an
// AuthorizationRule must have. It is a pure function of the rule: no cluster access, no state.
//
// Object names, roleRefs and labels are exactly those the Helm chart of the module used to
// render, so existing bindings are taken over in place during the migration.
package desired

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/validation/path"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "user-authz-controller/api/v1"
	"user-authz-controller/api/v1alpha1"
)

const (
	// LabelHeritage and LabelModule are the Deckhouse module labels every binding carries.
	LabelHeritage = "heritage"
	LabelModule   = "module"
	// LabelManagedBy marks bindings owned by the controller.
	LabelManagedBy = "user-authz.deckhouse.io/managed-by"
	// LabelBindingKind distinguishes the binding to the aggregated custom ClusterRole.
	LabelBindingKind = "user-authz.deckhouse.io/binding-kind"

	ModuleName           = "user-authz"
	HeritageValue        = "deckhouse"
	ManagedByValue       = "user-authz-controller"
	BindingKindAggregate = "aggregated-custom"

	// NamePrefix is the common prefix of every binding of every rule.
	NamePrefix = "user-authz:"

	AccessLevelUser           = "User"
	AccessLevelPrivilegedUser = "PrivilegedUser"
	AccessLevelEditor         = "Editor"
	AccessLevelAdmin          = "Admin"
	AccessLevelClusterEditor  = "ClusterEditor"
	AccessLevelClusterAdmin   = "ClusterAdmin"
	AccessLevelSuperAdmin     = "SuperAdmin"

	// clusterRoleKind is the only kind additionalRoles may reference: a binding of a rule always
	// points at a ClusterRole (the chart ignored the kind field and did the same).
	clusterRoleKind = "ClusterRole"
)

// ErrInvalidSpec is returned when the rule cannot be rendered into bindings.
var ErrInvalidSpec = errors.New("invalid rule spec")

// kebab mirrors the sprig kebabcase the chart applied to accessLevel.
var kebab = map[string]string{
	AccessLevelUser:           "user",
	AccessLevelPrivilegedUser: "privileged-user",
	AccessLevelEditor:         "editor",
	AccessLevelAdmin:          "admin",
	AccessLevelClusterEditor:  "cluster-editor",
	AccessLevelClusterAdmin:   "cluster-admin",
	AccessLevelSuperAdmin:     "super-admin",
}

// namespacedLevels are the access levels an AuthorizationRule may use (its RoleBinding is
// namespaced, cluster-wide levels make no sense there). Mirrors rbac.check.valid.spec of the chart.
var namespacedLevels = map[string]bool{
	AccessLevelUser:           true,
	AccessLevelPrivilegedUser: true,
	AccessLevelEditor:         true,
	AccessLevelAdmin:          true,
}

// Rule is the scope-independent view of a ClusterAuthorizationRule or AuthorizationRule.
type Rule struct {
	Name       string
	Namespace  string // empty for a ClusterAuthorizationRule
	UID        types.UID
	Generation int64
	APIVersion string
	Kind       string

	AccessLevel     string
	PortForwarding  bool
	AllowScale      bool
	Subjects        []rbacv1.Subject
	AdditionalRoles []v1.AdditionalRole
}

// Namespaced reports whether the rule renders RoleBindings (true) or ClusterRoleBindings (false).
func (r Rule) Namespaced() bool { return r.Namespace != "" }

// FromClusterAuthorizationRule adapts a ClusterAuthorizationRule.
func FromClusterAuthorizationRule(car *v1.ClusterAuthorizationRule) Rule {
	return Rule{
		Name:            car.Name,
		UID:             car.UID,
		Generation:      car.Generation,
		APIVersion:      v1.SchemeGroupVersion.String(),
		Kind:            "ClusterAuthorizationRule",
		AccessLevel:     car.Spec.AccessLevel,
		PortForwarding:  car.Spec.PortForwarding,
		AllowScale:      car.Spec.AllowScale,
		Subjects:        car.Spec.Subjects,
		AdditionalRoles: car.Spec.AdditionalRoles,
	}
}

// FromAuthorizationRule adapts an AuthorizationRule.
func FromAuthorizationRule(ar *v1alpha1.AuthorizationRule) Rule {
	return Rule{
		Name:           ar.Name,
		Namespace:      ar.Namespace,
		UID:            ar.UID,
		Generation:     ar.Generation,
		APIVersion:     v1alpha1.SchemeGroupVersion.String(),
		Kind:           "AuthorizationRule",
		AccessLevel:    ar.Spec.AccessLevel,
		PortForwarding: ar.Spec.PortForwarding,
		AllowScale:     ar.Spec.AllowScale,
		Subjects:       ar.Spec.Subjects,
	}
}

// Binding is one (Cluster)RoleBinding of a rule. Namespace is empty for a ClusterRoleBinding.
type Binding struct {
	Name      string
	Namespace string
	RoleRef   string
	Subjects  []rbacv1.Subject
	Labels    map[string]string
}

// Bindings renders the bindings of the rule in the order the chart used to render them:
// additionalRoles, access level, aggregated custom roles of the level, port-forward, scale.
//
// additionalRoles are validated the way the API server validates binding names (a role name that
// cannot be a path segment would make every write of the rule fail forever) and deduplicated by
// name, because two entries with the same name render the same binding.
func Bindings(r Rule) ([]Binding, error) {
	if r.Name == "" {
		return nil, fmt.Errorf("%w: empty rule name", ErrInvalidSpec)
	}

	var out []Binding

	add := func(postfix, roleRef string, extra map[string]string) {
		labels := map[string]string{
			LabelHeritage:  HeritageValue,
			LabelModule:    ModuleName,
			LabelManagedBy: ManagedByValue,
		}
		for k, v := range extra {
			labels[k] = v
		}
		out = append(out, Binding{
			Name:      NamePrefix + r.Name + ":" + postfix,
			Namespace: r.Namespace,
			RoleRef:   roleRef,
			Subjects:  slices.Clone(r.Subjects),
			Labels:    labels,
		})
	}

	seen := make(map[string]struct{}, len(r.AdditionalRoles))
	for _, role := range r.AdditionalRoles {
		if err := validateAdditionalRole(role); err != nil {
			return nil, err
		}
		if _, dup := seen[role.Name]; dup {
			continue
		}
		seen[role.Name] = struct{}{}
		add("additional-role:"+role.Name, role.Name, nil)
	}

	if r.AccessLevel != "" {
		level, known := kebab[r.AccessLevel]
		if !known {
			return nil, fmt.Errorf("%w: unsupported accessLevel %q", ErrInvalidSpec, r.AccessLevel)
		}
		if r.Namespaced() && !namespacedLevels[r.AccessLevel] {
			return nil, fmt.Errorf("%w: accessLevel %q is not allowed for a namespaced rule", ErrInvalidSpec, r.AccessLevel)
		}

		add(level, "user-authz:"+level, nil)

		if r.AccessLevel != AccessLevelSuperAdmin {
			add(level+":custom", "user-authz:"+level+":custom", map[string]string{LabelBindingKind: BindingKindAggregate})
		}
	}

	if r.PortForwarding {
		add("port-forward", "user-authz:port-forward", nil)
	}

	if r.AllowScale {
		add("scale", "user-authz:scale", nil)
	}

	return out, nil
}

func validateAdditionalRole(role v1.AdditionalRole) error {
	if role.Kind != "" && role.Kind != clusterRoleKind {
		return fmt.Errorf("%w: additionalRoles[%q] has kind %q, only ClusterRole is supported", ErrInvalidSpec, role.Name, role.Kind)
	}
	if role.APIGroup != "" && role.APIGroup != rbacv1.GroupName {
		return fmt.Errorf("%w: additionalRoles[%q] has apiGroup %q, only %s is supported", ErrInvalidSpec, role.Name, role.APIGroup, rbacv1.GroupName)
	}
	if role.Name == "" {
		return fmt.Errorf("%w: additionalRoles entry without a name", ErrInvalidSpec)
	}
	if msgs := path.ValidatePathSegmentName(role.Name, false); len(msgs) != 0 {
		return fmt.Errorf("%w: additionalRoles[%q]: %s", ErrInvalidSpec, role.Name, strings.Join(msgs, "; "))
	}
	return nil
}

// RulePrefix is the name prefix shared by all bindings of the rule.
func RulePrefix(ruleName string) string {
	return NamePrefix + ruleName + ":"
}

// RuleNameOf extracts the rule name from a binding name of the form user-authz:<rule>:<postfix>.
// Rule names are DNS-1123 subdomains, so the second segment is unambiguous.
func RuleNameOf(bindingName string) (string, bool) {
	rest, ok := strings.CutPrefix(bindingName, NamePrefix)
	if !ok {
		return "", false
	}
	name, _, found := strings.Cut(rest, ":")
	if !found || name == "" {
		return "", false
	}
	return name, true
}

// OwnerReference builds the controller owner reference to the rule.
func OwnerReference(r Rule) metav1.OwnerReference {
	controller := true
	blockOwnerDeletion := true
	return metav1.OwnerReference{
		APIVersion:         r.APIVersion,
		Kind:               r.Kind,
		Name:               r.Name,
		UID:                r.UID,
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
}

// ClusterRoleBinding materialises a cluster-scoped binding.
func ClusterRoleBinding(b Binding, owner metav1.OwnerReference) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{APIVersion: rbacv1.SchemeGroupVersion.String(), Kind: "ClusterRoleBinding"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            b.Name,
			Labels:          b.Labels,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		RoleRef:  roleRef(b.RoleRef),
		Subjects: b.Subjects,
	}
}

// RoleBinding materialises a namespaced binding.
func RoleBinding(b Binding, owner metav1.OwnerReference) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{APIVersion: rbacv1.SchemeGroupVersion.String(), Kind: "RoleBinding"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            b.Name,
			Namespace:       b.Namespace,
			Labels:          b.Labels,
			OwnerReferences: []metav1.OwnerReference{owner},
		},
		RoleRef:  roleRef(b.RoleRef),
		Subjects: b.Subjects,
	}
}

func roleRef(name string) rbacv1.RoleRef {
	return rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: clusterRoleKind, Name: name}
}
