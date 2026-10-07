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

package rolebinding

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
)

const (
	// serviceAccountUserPrefix starts the user name the API server authenticates a ServiceAccount as,
	// and serviceAccountGroupPrefix the group of all ServiceAccounts of a namespace.
	serviceAccountUserPrefix  = "system:serviceaccount:"
	serviceAccountGroupPrefix = "system:serviceaccounts:"
)

// ServiceAccountNamespace returns the namespace of the ServiceAccounts a subject names, and false
// when it names none. Besides a ServiceAccount subject, the User
// "system:serviceaccount:<namespace>:<name>" is the same ServiceAccount and the Group
// "system:serviceaccounts:<namespace>" is every ServiceAccount of the namespace, and RBAC grants
// them the same rights. A ServiceAccount subject without a namespace names the ServiceAccount of
// each namespace its RoleBinding is in, so the namespace returned for it is empty.
func ServiceAccountNamespace(subject rbacv1.Subject) (string, bool) {
	switch subject.Kind {
	case rbacv1.ServiceAccountKind:
		return subject.Namespace, true
	case rbacv1.UserKind:
		rest, ok := strings.CutPrefix(subject.Name, serviceAccountUserPrefix)
		if !ok {
			return "", false
		}
		// The API server never authenticates a malformed name, so such a user is no ServiceAccount.
		namespace, name, ok := strings.Cut(rest, ":")
		if !ok || namespace == "" || name == "" || strings.Contains(name, ":") {
			return "", false
		}
		return namespace, true
	case rbacv1.GroupKind:
		namespace, ok := strings.CutPrefix(subject.Name, serviceAccountGroupPrefix)
		if !ok || namespace == "" || strings.Contains(namespace, ":") {
			return "", false
		}
		return namespace, true
	default:
		return "", false
	}
}

// ServiceAccountNamespaces returns the namespaces other than except whose ServiceAccounts the
// subjects name, sorted and without repeats.
func ServiceAccountNamespaces(subjects []rbacv1.Subject, except string) []string {
	namespaces := make([]string, 0, len(subjects))
	for _, s := range subjects {
		if namespace, ok := ServiceAccountNamespace(s); ok && namespace != "" && namespace != except {
			namespaces = append(namespaces, namespace)
		}
	}
	slices.Sort(namespaces)
	return slices.Compact(namespaces)
}

// IsAdministratorsBinding reports whether a ProjectRoleBinding with this name and value of the
// managed-by label is the one the controller writes from the administrators of its project. The
// ProjectRoleBinding webhook refuses the label to everyone but the controller and Deckhouse.
func IsAdministratorsBinding(name, managedBy string) bool {
	return name == v1alpha3.ProjectAdministratorsBinding && managedBy == v1alpha3.ManagedByController
}

// AdministratorSubject is the subject that names an administrator of a project in the administrators
// binding.
func AdministratorSubject(administrator v1alpha3.Administrator) rbacv1.Subject {
	return rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: administrator.Kind, Name: administrator.Name}
}

// SplitProjectSubjects splits the subjects of a ProjectRoleBinding of the project into the ones its
// RoleBindings carry and the ones they leave out, in this order. A subject that names ServiceAccounts
// (see ServiceAccountNamespace) is carried only while their namespace is a namespace of the project,
// which is the main namespace or a namespace that exists, is not being deleted and carries the
// project label. The webhook checks a ServiceAccount subject when it is added, but its namespace can
// leave the project later, with a deleted ProjectNamespace for one, and the name can then go to a
// separate project whose ServiceAccounts must not get the rights of the binding. Other subjects are
// always carried. The namespaces are read through reader, the manager cache in the controller.
//
// A subject that names one of administrators is carried whatever namespace it names. The caller
// passes the administrators of the project for the binding the controller writes from them (see
// IsAdministratorsBinding), and nil for every other binding. Only a writer of the Project sets the
// administrators, and the previous release granted them the rights through an AuthorizationRule
// whatever namespace they named.
func SplitProjectSubjects(
	ctx context.Context,
	reader client.Reader,
	project string,
	subjects []rbacv1.Subject,
	administrators []v1alpha3.Administrator,
) ([]rbacv1.Subject, []rbacv1.Subject, error) {
	kept := make([]rbacv1.Subject, 0, len(subjects))
	var leftOut []rbacv1.Subject
	members := map[string]bool{}
	for _, s := range subjects {
		namespace, ok := ServiceAccountNamespace(s)
		if !ok || namespace == "" || namespace == project || namesAdministrator(s, administrators) {
			kept = append(kept, s)
			continue
		}
		member, known := members[namespace]
		if !known {
			var err error
			if member, err = namespaceInProject(ctx, reader, namespace, project); err != nil {
				return nil, nil, err
			}
			members[namespace] = member
		}
		if member {
			kept = append(kept, s)
		} else {
			leftOut = append(leftOut, s)
		}
	}
	return kept, leftOut, nil
}

// namesAdministrator reports whether the subject names one of the administrators the way the
// administrators binding does.
func namesAdministrator(subject rbacv1.Subject, administrators []v1alpha3.Administrator) bool {
	return slices.ContainsFunc(administrators, func(administrator v1alpha3.Administrator) bool {
		return AdministratorSubject(administrator) == subject
	})
}

// namespaceInProject reports whether the namespace exists, is not being deleted and carries the label
// of the project.
func namespaceInProject(ctx context.Context, reader client.Reader, name, project string) (bool, error) {
	ns := new(corev1.Namespace)
	if err := reader.Get(ctx, client.ObjectKey{Name: name}, ns); err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get the %q namespace: %w", name, err)
	}
	return ns.DeletionTimestamp.IsZero() && ns.Labels[v1alpha3.ResourceLabelProject] == project, nil
}

// ServiceAccountGrantConflict returns why a namespace named name cannot become a namespace of the
// project yet, or an empty string when it can. A ClusterProjectRoleBinding, or a ProjectRoleBinding of
// another project, can still name the ServiceAccounts of a namespace that is gone, and the ones a new
// namespace under that name brings would get its rights. The fan-out of a ProjectRoleBinding leaves
// them out once the controller gets to it, and a ClusterProjectRoleBinding has no project to check
// them against, so the name is refused until the subjects are removed. The reason does not name the
// binding, which the requester may not be allowed to see.
//
// The ProjectRoleBindings are read in the projects whose name and a dash start name, as Claiming
// reads the ProjectNamespaces. The webhook admits a ServiceAccount subject of such a namespace only,
// and reading every ProjectRoleBinding of the cluster on each create would cost too much. A User or
// Group subject of another name is left to the fan-out. The administrators binding the controller
// writes is the exception: it may name the ServiceAccounts of any namespace and the fan-out carries
// them, so a name that does not extend the name of their project is not refused for them here.
func ServiceAccountGrantConflict(ctx context.Context, reader client.Reader, name, project string) (string, error) {
	const removeFirst = "still grants rights to ServiceAccounts of a namespace with this name. These subjects have to be removed from that binding first"

	cprbs := new(v1alpha3.ClusterProjectRoleBindingList)
	if err := reader.List(ctx, cprbs); err != nil {
		return "", fmt.Errorf("list the ClusterProjectRoleBindings: %w", err)
	}
	for i := range cprbs.Items {
		if namesServiceAccountsOf(cprbs.Items[i].Spec.Subjects, name) {
			return "a ClusterProjectRoleBinding " + removeFirst, nil
		}
	}

	for i := 1; i < len(name)-1; i++ {
		if name[i] != '-' || name[:i] == project {
			continue
		}
		prbs := new(v1alpha3.ProjectRoleBindingList)
		if err := reader.List(ctx, prbs, client.InNamespace(name[:i])); err != nil {
			return "", fmt.Errorf("list the ProjectRoleBindings of the %q project: %w", name[:i], err)
		}
		for j := range prbs.Items {
			if namesServiceAccountsOf(prbs.Items[j].Spec.Subjects, name) {
				return "a ProjectRoleBinding of another project " + removeFirst, nil
			}
		}
	}
	return "", nil
}

// namesServiceAccountsOf reports whether one of the subjects names ServiceAccounts of the namespace.
func namesServiceAccountsOf(subjects []rbacv1.Subject, namespace string) bool {
	return slices.ContainsFunc(subjects, func(s rbacv1.Subject) bool {
		ns, ok := ServiceAccountNamespace(s)
		return ok && ns == namespace
	})
}
