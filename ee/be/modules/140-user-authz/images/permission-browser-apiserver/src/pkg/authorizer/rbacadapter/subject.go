/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package rbacadapter

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
)

// SubjectMatches reports whether a single binding subject refers to the given
// identity.
//
// defaultNamespace is used for ServiceAccount subjects that omit the namespace:
// inside a RoleBinding such a subject means "the ServiceAccount of this
// binding's namespace". Pass an empty string for ClusterRoleBindings, where
// there is no namespace to default to.
func SubjectMatches(subject rbacv1.Subject, userName string, userGroups []string, defaultNamespace string) bool {
	switch subject.Kind {
	case rbacv1.UserKind:
		return subject.Name == userName
	case rbacv1.GroupKind:
		for _, group := range userGroups {
			if subject.Name == group {
				return true
			}
		}
	case rbacv1.ServiceAccountKind:
		namespace := subject.Namespace
		if namespace == "" {
			namespace = defaultNamespace
		}
		return fmt.Sprintf("system:serviceaccount:%s:%s", namespace, subject.Name) == userName
	}

	return false
}

// MatchingSubject returns the first binding subject that refers to the given
// identity. Callers that need to explain *why* a binding applies (matched
// directly by name or through one of the identity's groups) use the returned
// subject; callers that only need a yes/no answer can ignore it.
func MatchingSubject(subjects []rbacv1.Subject, userName string, userGroups []string, defaultNamespace string) (rbacv1.Subject, bool) {
	for _, subject := range subjects {
		if SubjectMatches(subject, userName, userGroups, defaultNamespace) {
			return subject, true
		}
	}

	return rbacv1.Subject{}, false
}
