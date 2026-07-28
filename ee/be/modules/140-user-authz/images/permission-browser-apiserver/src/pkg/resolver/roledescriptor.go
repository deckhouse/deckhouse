/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

// Labels and annotations the role model puts on its ClusterRoles. Reading them
// is what lets the API return ready-to-display role metadata instead of making
// every client re-implement role-name parsing.
const (
	labelRoleKind    = "rbac.deckhouse.io/kind"
	labelRoleScope   = "rbac.deckhouse.io/scope"
	labelSubsystem   = "rbac.deckhouse.io/subsystem"
	labelDeprecated  = "rbac.deckhouse.io/deprecated"
	annotationPrefix = ".meta.deckhouse.io/"
)

// displayLanguages are the languages the role model publishes titles for.
var displayLanguages = []string{"en", "ru"}

// accessLevels are the levels used by the role model, in ascending order.
var accessLevels = map[string]struct{}{
	"viewer":     {},
	"user":       {},
	"manager":    {},
	"admin":      {},
	"superadmin": {},
}

// DescribeRole builds the display metadata of a role.
//
// Labels and annotations are authoritative: they are set by the role model and
// cover custom roles too. Role-name parsing is only a fallback for roles that
// predate the labels (the deprecated aliases) or come from outside the model.
func DescribeRole(meta *metav1.ObjectMeta, roleName string) v1alpha1.RoleDescriptor {
	descriptor := v1alpha1.RoleDescriptor{}

	if meta != nil {
		labels := meta.GetLabels()
		descriptor.Kind = labels[labelRoleKind]
		descriptor.Scope = labels[labelRoleScope]
		descriptor.Subsystem = labels[labelSubsystem]
		descriptor.Deprecated = labels[labelDeprecated] == "true"
		descriptor.Titles, descriptor.Descriptions = displayText(meta.GetAnnotations())
	}

	fillFromName(&descriptor, roleName)

	return descriptor
}

// displayText collects the localized title/description annotations. An operator
// may override the built-in wording with custom.meta.deckhouse.io/*, which the
// role model documents as the only supported edit of a built-in role, so that
// override wins over the shipped text.
func displayText(annotations map[string]string) (map[string]string, map[string]string) {
	if len(annotations) == 0 {
		return nil, nil
	}

	var titles, descriptions map[string]string

	for _, language := range displayLanguages {
		if title := annotations[language+annotationPrefix+"title"]; title != "" {
			titles = putText(titles, language, title)
		}
		if description := annotations[language+annotationPrefix+"description"]; description != "" {
			descriptions = putText(descriptions, language, description)
		}
	}

	if custom := annotations["custom"+annotationPrefix+"title"]; custom != "" {
		for _, language := range displayLanguages {
			titles = putText(titles, language, custom)
		}
	}
	if custom := annotations["custom"+annotationPrefix+"description"]; custom != "" {
		for _, language := range displayLanguages {
			descriptions = putText(descriptions, language, custom)
		}
	}

	return titles, descriptions
}

func putText(target map[string]string, language, value string) map[string]string {
	if target == nil {
		target = make(map[string]string, len(displayLanguages))
	}
	target[language] = value

	return target
}

// fillFromName derives whatever the labels did not provide from the role naming
// convention: d8:system:<level>, d8:subsystem:<name>:<level>,
// d8:namespace:<level>, d8:project:<level>, d8:custom:*, d8:*-capability:*.
func fillFromName(descriptor *v1alpha1.RoleDescriptor, roleName string) {
	parts := strings.Split(roleName, ":")
	if len(parts) < 2 || parts[0] != "d8" {
		return
	}

	if descriptor.Kind == "" {
		switch {
		case parts[1] == "custom":
			descriptor.Kind = "custom-role"
		case strings.Contains(roleName, "-capability:"):
			descriptor.Kind = "capability"
		}
	}

	// A custom role names its own scope after the prefix: d8:custom:<scope>:...
	if parts[1] == "custom" {
		parts = parts[1:]
	}

	switch {
	case parts[1] == "system" && len(parts) > 2:
		setIfEmpty(&descriptor.Scope, "system")
		setLevel(descriptor, parts[2])
	case parts[1] == "subsystem" && len(parts) > 3:
		setIfEmpty(&descriptor.Scope, "subsystem")
		setIfEmpty(&descriptor.Subsystem, parts[2])
		setLevel(descriptor, parts[3])
	case parts[1] == "namespace" && len(parts) > 2:
		setIfEmpty(&descriptor.Scope, "namespace")
		setLevel(descriptor, parts[2])
	case parts[1] == "project" && len(parts) > 2:
		setIfEmpty(&descriptor.Scope, "project")
		setLevel(descriptor, parts[2])
	default:
		// Roles of the deprecated naming scheme: d8:manage:<subsystem|all>:<level>
		// and d8:use:role:<level>. They still exist as compatibility aliases, so
		// reporting their scope and level keeps the UI meaningful during migration.
		fillFromLegacyName(descriptor, parts)
	}
}

func fillFromLegacyName(descriptor *v1alpha1.RoleDescriptor, parts []string) {
	switch {
	case parts[1] == "manage" && len(parts) > 3 && parts[2] == "all":
		setIfEmpty(&descriptor.Scope, "system")
		setLevel(descriptor, parts[3])
	case parts[1] == "manage" && len(parts) > 3:
		setIfEmpty(&descriptor.Scope, "subsystem")
		setIfEmpty(&descriptor.Subsystem, parts[2])
		setLevel(descriptor, parts[3])
	case parts[1] == "use" && len(parts) > 3 && parts[2] == "role":
		setIfEmpty(&descriptor.Scope, "namespace")
		setLevel(descriptor, parts[3])
	}
}

func setIfEmpty(target *string, value string) {
	if *target == "" {
		*target = value
	}
}

func setLevel(descriptor *v1alpha1.RoleDescriptor, level string) {
	if descriptor.Level != "" {
		return
	}
	if _, ok := accessLevels[level]; ok {
		descriptor.Level = level
	}
}

// IsSuperadminRole reports whether the role grants the superadmin level of a
// scope whose protection the system-resource admission webhook bypasses for
// superadmins.
func IsSuperadminRole(descriptor v1alpha1.RoleDescriptor) bool {
	if descriptor.Level != "superadmin" {
		return false
	}

	switch descriptor.Scope {
	case "namespace", "project", "system", "subsystem":
		return true
	default:
		return false
	}
}

// clusterAdminRoles are the roles the system-resource webhook treats as a cluster
// administrator: Kubernetes' own cluster-admin and the previous role model's
// user-authz:super-admin, which a ClusterAuthorizationRule with accessLevel
// SuperAdmin binds to. They carry no rbac.deckhouse.io labels, so the descriptor
// says nothing about them and they have to be recognised by name.
var clusterAdminRoles = map[string]struct{}{
	"cluster-admin":          {},
	"user-authz:super-admin": {},
}

// IsClusterAdminRole reports whether holding this role bypasses the
// system-resource protection the same way a superadmin does. Keep in sync with
// CLUSTER_ADMIN_ROLES in modules/140-user-authz/webhooks/validating/system_resources.py:
// a report that claims a restriction the webhook does not apply is worse than no
// report at all.
func IsClusterAdminRole(roleName string) bool {
	_, ok := clusterAdminRoles[roleName]

	return ok
}
