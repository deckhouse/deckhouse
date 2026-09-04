/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package scopefilter mirrors, for reporting purposes, the part of the
// kube-apiserver that answers a read with an ACL-filtered list instead of a
// denial. It decides nothing about real access: the apiserver has already made
// that decision, and this package only keeps the permission browser from
// telling a user they cannot read something the API will happily serve them.
package scopefilter

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// filteredReadReason explains an allow that RBAC did not grant. It names the
// mechanism so an operator reading a BulkSubjectAccessReview response can tell
// this apart from an ordinary RBAC allow.
const filteredReadReason = "the namespace ACL answers this read with the accessible subset instead of a denial"

// namespaceIdentityResources are the cluster-scoped resources whose visibility
// the apiserver derives from the namespace ACL rather than from RBAC: a Project
// is visible whenever any of its namespaces is. Reads of them are never
// answered with a 403 -- the storage layer returns the accessible subset, and a
// GET of an invisible object answers NotFound.
//
// Keep in sync with scopefilter.namespaceIdentityResources in the apiserver
// patch (modules/000-common/images/kubernetes/patches/*/*-namespace-list-acl-filtering.patch);
// the two describe the same allowlist, one enforcing it and one reporting it.
var namespaceIdentityResources = map[schema.GroupResource]struct{}{
	{Group: "deckhouse.io", Resource: "projects"}: {},
}

// ResourceRegistry reports whether the cluster serves a resource at all.
type ResourceRegistry interface {
	HasResource(group, resource string) bool
}

// IdentityReadAuthorizer reports reads of a namespace-ACL-filtered resource as
// allowed even when the wrapped authorizer does not, because that is what the
// API does. Withholding the cluster-wide RBAC grant is what lets the filter
// engage at all (see modules/140-user-authz/templates/cluster-roles.yaml), so
// without this the permission browser would answer "denied" for every tenant
// and the console would hide a section the user can in fact open.
//
// The allow is deliberately unconditional on how much the user may see: an
// empty list is a successful read, and reporting it as denied would be the same
// existence oracle the filtering is there to close. It is not unconditional on
// the resource existing: an identity resource lives in a CRD that a cluster
// without multitenancy-manager does not have, and there the apiserver keeps the
// plain denial too, because its own bypass is gated on the filter being
// registered for the resource.
type IdentityReadAuthorizer struct {
	inner    authorizer.Authorizer
	registry ResourceRegistry
}

var _ authorizer.Authorizer = (*IdentityReadAuthorizer)(nil)

// NewIdentityReadAuthorizer wraps inner. A nil inner is not supported. A nil
// registry disables the override entirely: without knowing which resources the
// cluster serves there is no ground to answer anything but plain RBAC.
func NewIdentityReadAuthorizer(inner authorizer.Authorizer, registry ResourceRegistry) *IdentityReadAuthorizer {
	return &IdentityReadAuthorizer{inner: inner, registry: registry}
}

// Authorize implements authorizer.Authorizer.
func (a *IdentityReadAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	decision, reason, err := a.inner.Authorize(ctx, attrs)
	if err != nil || decision == authorizer.DecisionAllow {
		return decision, reason, err
	}

	if !isNamespaceIdentityRead(attrs) {
		return decision, reason, nil
	}
	if a.registry == nil || !a.registry.HasResource(attrs.GetAPIGroup(), attrs.GetResource()) {
		return decision, reason, nil
	}

	return authorizer.DecisionAllow, filteredReadReason, nil
}

// isNamespaceIdentityRead reports whether the request is one the apiserver
// hands to the storage filter: a top-level read of a cluster-scoped identity
// resource. Mutating verbs and subresources keep the plain denial there, so
// they must keep it here too.
func isNamespaceIdentityRead(attrs authorizer.Attributes) bool {
	if attrs == nil || !attrs.IsResourceRequest() {
		return false
	}
	if attrs.GetSubresource() != "" || attrs.GetNamespace() != "" {
		return false
	}

	switch attrs.GetVerb() {
	case "get", "list", "watch":
	default:
		return false
	}

	gr := schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}
	_, ok := namespaceIdentityResources[gr]

	return ok
}
