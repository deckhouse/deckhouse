/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"fmt"
	"strings"

	"permission-browser-apiserver/pkg/authorizer/multitenancy"
)

// applyMultitenancy accounts for ClusterAuthorizationRule restrictions.
//
// The rule of thumb mirrors both the authorization webhook and
// NamespaceResolver: a ClusterAuthorizationRule narrows only the access it
// itself grants. Grants that exist independently of it - RoleBindings and
// ClusterRoleBindings that were not rendered from a rule - are never hidden,
// otherwise the report would claim less than the subject can really do.
//
// In this report CAR-derived access can only appear in the cluster-wide scope,
// because namespace scopes are built exclusively from RoleBindings. So the
// filter has nothing to remove; what it must do is say that the cluster-wide
// rows only apply inside the namespaces the rule allows. Silently dropping
// those rows would be worse: the subject really does have that access, just not
// everywhere.
func (r *SubjectAccessResolver) applyMultitenancy(identity subjectIdentity, report *reportBuilder) []string {
	if r.mtEngine == nil || !report.cluster.anyCAR {
		return nil
	}

	accessType, filter := r.mtEngine.GetNamespaceAccessType(identity.userInfo())
	if accessType != multitenancy.FilteredAccess || filter == nil {
		return nil
	}

	scope := "part of the cluster-wide access"
	if report.cluster.carOnly {
		scope = "the cluster-wide access"
	}

	return []string{fmt.Sprintf(
		"%s comes from a ClusterAuthorizationRule and applies only in the namespaces it allows (%s)",
		scope, describeNamespaceFilter(filter),
	)}
}

// describeNamespaceFilter renders the namespace limits of a rule in a form a
// human can act on.
func describeNamespaceFilter(filter *multitenancy.DirectoryEntry) string {
	var parts []string

	if !filter.NamespaceFiltersAbsent && len(filter.LimitNamespaces) > 0 {
		patterns := make([]string, 0, len(filter.LimitNamespaces))
		for _, pattern := range filter.LimitNamespaces {
			patterns = append(patterns, pattern.String())
		}
		parts = append(parts, "limitNamespaces: "+strings.Join(patterns, ", "))
	}

	if len(filter.NamespaceSelectors) > 0 {
		parts = append(parts, "namespaceSelector is set")
	}

	if !filter.AllowAccessToSystemNamespaces {
		parts = append(parts, "system namespaces are excluded")
	}

	if len(parts) == 0 {
		return "see the ClusterAuthorizationRule"
	}

	return strings.Join(parts, "; ")
}
