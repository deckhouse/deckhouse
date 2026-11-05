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

package scope

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// ReasonFiltered is the reason of a read WithAuthorizer allowed.
const ReasonFiltered = "scope: read denied by RBAC in kube-apiserver, served filtered by the trusted namespace set"

// WithAuthorizer wraps the delegated authorizer of an extension server.
//
// kube-apiserver sends ModeFiltered for a read RBAC denied there, so the
// extension's own delegated SubjectAccessReview of it would deny too. The
// wrapper allows such a read without asking a, when all of these hold:
//
//   - the context carries a trusted Decision with ModeFiltered;
//   - it is a resource request without a subresource;
//   - the verb is list, get or watch;
//   - the group and resource are among resources (the ones the APIService
//     opts in with scope.deckhouse.io/resources).
//
// It does not check the namespace of the request against the set:
// kube-apiserver forwards a list or watch in a namespace outside the set with
// the empty set and expects an empty result, not 403. The storage restricts
// the data with Allowed, FilterList, WatchFilter and CheckGet; skipping the
// review is safe only because it does.
//
// Everything else, ModeNarrowed included, is decided by a. With no resources
// the wrapper never allows anything itself.
func WithAuthorizer(a authorizer.Authorizer, resources ...schema.GroupResource) authorizer.Authorizer {
	opted := make(map[schema.GroupResource]struct{}, len(resources))
	for _, gr := range resources {
		opted[gr] = struct{}{}
	}
	return authorizer.AuthorizerFunc(func(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
		if allowsFiltered(ctx, attrs, opted) {
			return authorizer.DecisionAllow, ReasonFiltered, nil
		}
		return a.Authorize(ctx, attrs)
	})
}

func allowsFiltered(ctx context.Context, attrs authorizer.Attributes, opted map[schema.GroupResource]struct{}) bool {
	if len(opted) == 0 || attrs == nil {
		return false
	}
	d, ok := FromContext(ctx)
	if !ok || d.Mode != ModeFiltered {
		return false
	}
	if !attrs.IsResourceRequest() || attrs.GetSubresource() != "" {
		return false
	}
	switch attrs.GetVerb() {
	case "list", "get", "watch":
	default:
		return false
	}
	_, ok = opted[schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}]
	return ok
}
