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
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type countingAuthorizer struct {
	calls    int
	decision authorizer.Decision
}

func (c *countingAuthorizer) Authorize(context.Context, authorizer.Attributes) (authorizer.Decision, string, error) {
	c.calls++
	return c.decision, "delegate", nil
}

func TestWithAuthorizer(t *testing.T) {
	gizmos := schema.GroupResource{Group: "scopetoy.deckhouse.io", Resource: "gizmos"}
	filtered := WithDecision(context.Background(), Decision{Mode: ModeFiltered, Namespaces: sets.New("a")})
	filteredEmpty := WithDecision(context.Background(), Decision{Mode: ModeFiltered})
	narrowed := WithDecision(context.Background(), Decision{Mode: ModeNarrowed, Namespaces: sets.New("a")})
	none := context.Background()

	read := func(verb, ns, sub string) authorizer.AttributesRecord {
		return authorizer.AttributesRecord{
			User: &user.DefaultInfo{Name: "alice"}, Verb: verb, Namespace: ns,
			APIGroup: gizmos.Group, APIVersion: "v1", Resource: gizmos.Resource, Subresource: sub,
			ResourceRequest: true,
		}
	}
	other := read("list", "", "")
	other.Resource = "widgets"
	otherGroup := read("list", "", "")
	otherGroup.APIGroup = "other.io"
	nonResource := authorizer.AttributesRecord{User: &user.DefaultInfo{Name: "alice"}, Verb: "get", Path: "/apis/scopetoy.deckhouse.io/v1"}

	cases := []struct {
		name      string
		ctx       context.Context
		attrs     authorizer.Attributes
		resources []schema.GroupResource
		want      authorizer.Decision // with a denying delegate
		delegated bool
	}{
		{"filtered cluster-wide list", filtered, read("list", "", ""), []schema.GroupResource{gizmos}, authorizer.DecisionAllow, false},
		{"filtered watch", filtered, read("watch", "", ""), []schema.GroupResource{gizmos}, authorizer.DecisionAllow, false},
		{"filtered get in the set", filtered, read("get", "a", ""), []schema.GroupResource{gizmos}, authorizer.DecisionAllow, false},
		{"filtered list outside the set is allowed, storage returns empty", filtered, read("list", "b", ""), []schema.GroupResource{gizmos}, authorizer.DecisionAllow, false},
		{"filtered empty set", filteredEmpty, read("list", "", ""), []schema.GroupResource{gizmos}, authorizer.DecisionAllow, false},
		{"filtered create", filtered, read("create", "a", ""), []schema.GroupResource{gizmos}, authorizer.DecisionDeny, true},
		{"filtered delete", filtered, read("delete", "a", ""), []schema.GroupResource{gizmos}, authorizer.DecisionDeny, true},
		{"filtered subresource", filtered, read("get", "a", "status"), []schema.GroupResource{gizmos}, authorizer.DecisionDeny, true},
		{"filtered resource not opted in", filtered, other, []schema.GroupResource{gizmos}, authorizer.DecisionDeny, true},
		{"filtered group not opted in", filtered, otherGroup, []schema.GroupResource{gizmos}, authorizer.DecisionDeny, true},
		{"filtered non-resource request", filtered, nonResource, []schema.GroupResource{gizmos}, authorizer.DecisionDeny, true},
		{"filtered, no resources configured", filtered, read("list", "", ""), nil, authorizer.DecisionDeny, true},
		{"narrowed delegates", narrowed, read("list", "", ""), []schema.GroupResource{gizmos}, authorizer.DecisionDeny, true},
		{"no decision delegates", none, read("list", "", ""), []schema.GroupResource{gizmos}, authorizer.DecisionDeny, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			delegate := &countingAuthorizer{decision: authorizer.DecisionDeny}
			got, _, err := WithAuthorizer(delegate, tc.resources...).Authorize(tc.ctx, tc.attrs)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("decision %v, want %v", got, tc.want)
			}
			if (delegate.calls > 0) != tc.delegated {
				t.Errorf("delegate calls %d, want delegated=%v", delegate.calls, tc.delegated)
			}
		})
	}

	t.Run("narrowed keeps an allowing delegate", func(t *testing.T) {
		delegate := &countingAuthorizer{decision: authorizer.DecisionAllow}
		got, reason, _ := WithAuthorizer(delegate, gizmos).Authorize(narrowed, read("list", "", ""))
		if got != authorizer.DecisionAllow || reason != "delegate" {
			t.Fatalf("%v %q", got, reason)
		}
	})
}
