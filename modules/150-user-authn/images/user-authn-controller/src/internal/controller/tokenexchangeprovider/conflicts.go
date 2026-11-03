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

package tokenexchangeprovider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// names are the name spaces a provider gives its subjects: the prefix of the user name the API
// server takes (the email, which is the user name unless identity.email is set) and the prefixes
// of the groups.
type names struct {
	user   string
	groups []string
}

func namesOf(spec Spec) names {
	n := names{user: spec.Identity.Username.Prefix}
	if spec.Identity.Email != nil {
		n.user = spec.Identity.Email.Prefix
	}
	for _, g := range spec.Identity.Groups {
		n.groups = append(n.groups, g.Prefix)
	}
	return n
}

// overlap reports whether two prefixes can produce the same name: equal ones, or one that the
// other starts with. The schema requires every prefix and its colon.
func overlap(a, b string) bool {
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

// conflict describes where two providers can name a subject alike, or returns "".
func conflict(a, b names) string {
	if overlap(a.user, b.user) {
		return fmt.Sprintf("user name prefix %q", a.user)
	}
	for _, ga := range a.groups {
		for _, gb := range b.groups {
			if overlap(ga, gb) {
				return fmt.Sprintf("group prefix %q", ga)
			}
		}
	}
	return ""
}

// earlier reports whether the provider a was there before b: the older one keeps its names.
func earlier(a, b *unstructured.Unstructured) bool {
	ta, tb := a.GetCreationTimestamp(), b.GetCreationTimestamp()
	if !ta.Equal(&tb) {
		return ta.Before(&tb)
	}
	return a.GetName() < b.GetName()
}

// conflictingProvider returns the earlier provider whose names overlap any of own and where, or
// empty strings. Two issuers that give the same names would let one act as the users of the
// other, so the later provider gets no connector.
func (r *Reconciler) conflictingProvider(ctx context.Context, obj *unstructured.Unstructured, own []names) (string, string, error) {
	list := controller.List(controller.TokenExchangeProviderGVK)
	if err := r.client.List(ctx, list); err != nil {
		return "", "", fmt.Errorf("list TokenExchangeProviders: %w", err)
	}
	for i := range list.Items {
		other := &list.Items[i]
		if other.GetName() == obj.GetName() || !earlier(other, obj) {
			continue
		}
		// The earlier provider holds the names of its spec and of the connector Dex serves for it:
		// a spec that turned invalid leaves its previous connector in place.
		theirs, err := r.connectorNames(ctx, other.GetName())
		if err != nil {
			return "", "", err
		}
		if otherSpec, err := Decode(other); err == nil {
			theirs = append(theirs, namesOf(otherSpec))
		}
		for _, n := range own {
			for _, t := range theirs {
				if where := conflict(n, t); where != "" {
					return other.GetName(), where, nil
				}
			}
		}
	}
	return "", "", nil
}

// connectorNames returns the names the connectors of the provider give now, read from their
// configs in the Dex storage. A config that does not decode gives none.
func (r *Reconciler) connectorNames(ctx context.Context, name string) ([]names, error) {
	list := controller.List(controller.ConnectorGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingLabels{ProviderLabel: name}); err != nil {
		return nil, fmt.Errorf("list connectors of %s: %w", name, err)
	}
	var result []names
	for i := range list.Items {
		encoded, _ := list.Items[i].Object["config"].(string)
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		var config connectorConfig
		if err := json.Unmarshal(raw, &config); err != nil {
			continue
		}
		result = append(result, namesOf(Spec{Identity: config.Identity}))
	}
	return result, nil
}

// mapAllProviders enqueues every provider: one that appears, changes its names or goes away may
// start or end a conflict with any other.
func (r *Reconciler) mapAllProviders(ctx context.Context, _ client.Object) []reconcile.Request {
	list := controller.List(controller.TokenExchangeProviderGVK)
	if err := r.client.List(ctx, list); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: list.Items[i].GetName()}})
	}
	return requests
}
