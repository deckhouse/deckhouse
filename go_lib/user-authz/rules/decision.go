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

package rules

import "k8s.io/apimachinery/pkg/labels"

const (
	// NoNamespaceAccessReason is deliberately ambiguous: a user who may not act in a namespace must
	// not be able to tell an existing namespace from a missing one. Distinct answers turned the
	// authorizer into a namespace-existence oracle for anyone limited by a rule.
	NoNamespaceAccessReason = "either you have no access to the namespace or the namespace does not exist"
	// NamespaceLimitedAccessReason is the answer to a cluster-scoped request for a namespaced
	// resource by a subject whose namespaces are limited: such a request would list every namespace.
	NamespaceLimitedAccessReason = "making cluster-scoped requests for namespaced resources is not allowed"
)

// NamespaceLabels returns the labels of a namespace for the selector check. An error stands for a
// namespace that does not exist as well as for a cache problem; the caller reports neither to the
// user (see NoNamespaceAccessReason) and treats the selector as not matching.
type NamespaceLabels func(namespace string) (labels.Set, error)

// NamespaceAllowed reports whether the entry opens the namespace: it must match a limitNamespaces
// pattern (or the entry must have no filters), it must not be a system namespace unless allowed,
// and failing both a namespaceSelector may still open it. This is the whole multi-tenancy scope of
// a subject; whether CAR-independent RBAC grants the request anyway is the caller's next question.
//
// The caller must first check HasAnyFilters: an entry without filters has no opinion at all, and
// this function would report every non-system namespace as allowed.
func NamespaceAllowed(entry *Entry, namespace string, nsLabels NamespaceLabels) (allowed bool, err error) {
	allowed = entry.NamespaceFiltersAbsent
	if !allowed {
		for _, m := range entry.LimitNamespaces {
			if m.Matches(namespace) {
				allowed = true
				break
			}
		}
	}

	if allowed && !entry.AllowAccessToSystemNamespaces && IsSystemNamespace(namespace) {
		allowed = false
	}

	// A namespaceSelector opens the namespaces it matches whatever the patterns and the system gate
	// said, system namespaces included.
	if !allowed && len(entry.NamespaceSelectors) > 0 && nsLabels != nil {
		set, lookupErr := nsLabels(namespace)
		if lookupErr != nil {
			return false, lookupErr
		}
		for _, selector := range entry.NamespaceSelectors {
			if selector.Labels != nil && selector.Labels.Matches(set) {
				return true, nil
			}
		}
	}

	return allowed, nil
}

// ResourceScope is what a consumer knows about the resource of a cluster-scoped request.
type ResourceScope struct {
	// Known is set when discovery has an answer for the group/resource.
	Known bool
	// Namespaced is the answer when Known.
	Namespaced bool
	// CoreGroupPopulated is set when discovery has a snapshot of the core group, so an unknown
	// core resource is one that does not exist rather than one not looked up yet.
	CoreGroupPopulated bool
	// Core is set for a resource of the core group (empty apiGroup).
	Core bool
}

// ClusterScopedDenied reports whether a cluster-scoped request for the resource (a list or watch
// across all namespaces, or a request for a cluster-scoped object) must be denied for an entry with
// filters. A cluster-scoped resource is never limited by namespaces. A namespaced or unknown
// resource is: an unknown resource is treated as namespaced so that a discovery miss cannot open a
// cluster-wide list. The one exception is a core resource absent from a populated core snapshot:
// it does not exist, and RBAC is left to answer, which it does with a denial of its own.
//
// The caller must first check HasAnyFilters, and afterwards may still let CAR-independent RBAC
// override the denial.
func ClusterScopedDenied(scope ResourceScope) bool {
	if scope.Known && !scope.Namespaced {
		return false
	}
	if !scope.Known && scope.Core && scope.CoreGroupPopulated {
		return false
	}
	return true
}
