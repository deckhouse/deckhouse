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

// Package rolebinding holds helpers shared by the ProjectRoleBinding and ClusterProjectRoleBinding
// reconcilers and webhooks.
package rolebinding

import (
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"

	"controller/apis/deckhouse.io/v1alpha3"
)

const (
	// prbServicePrefix/cprbServicePrefix prefix the service RoleBindings fanned out by the
	// PRB/CPRB reconcilers.
	prbServicePrefix  = "d8:prb:"
	cprbServicePrefix = "d8:cprb:"
)

// PRBServiceName returns the name of the service RoleBinding fanned out for a ProjectRoleBinding.
func PRBServiceName(name string) string {
	return prbServicePrefix + name
}

// CPRBServiceName returns the name of the service RoleBinding fanned out for a
// ClusterProjectRoleBinding.
func CPRBServiceName(name string) string {
	return cprbServicePrefix + name
}

// ProjectNamespaceNames returns the namespaces of the project. It always includes the main
// namespace (the project name) even before the status is populated.
func ProjectNamespaceNames(project *v1alpha3.Project) []string {
	if len(project.Status.Namespaces) == 0 {
		return []string{project.Name}
	}
	names := make([]string, 0, len(project.Status.Namespaces))
	hasMain := false
	for _, ns := range project.Status.Namespaces {
		names = append(names, ns.Name)
		if ns.Name == project.Name {
			hasMain = true
		}
	}
	if !hasMain {
		names = append(names, project.Name)
	}
	return names
}

// CopySubjects deep-copies a subjects slice.
func CopySubjects(in []rbacv1.Subject) []rbacv1.Subject {
	if in == nil {
		return nil
	}
	out := make([]rbacv1.Subject, len(in))
	copy(out, in)
	return out
}

// AllowedRolePrefixes lists the ClusterRole name prefixes that may be granted via PRB/CPRB.
var AllowedRolePrefixes = []string{
	"d8:project:",
	"d8:namespace:",
	"d8:project-capability:",
	"d8:namespace-capability:",
	"d8:custom:",
}

// IsRoleAllowed reports whether a ClusterRole name may be granted via a (Cluster)ProjectRoleBinding.
func IsRoleAllowed(name string) bool {
	for _, prefix := range AllowedRolePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
