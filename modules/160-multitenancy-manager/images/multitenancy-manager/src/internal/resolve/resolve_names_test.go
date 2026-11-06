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

package resolve

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
)

// TestResolveNamesMatchesResolve checks that, for every name it was asked about, ResolveNames decides
// as Resolve does, agrees on membership in the available set and on the effective default, over a
// matrix of excluded filters, entries (literal and selector, allow and deny, baselines, defaults),
// registration defaults and names with and without an object. Names without an object are the trap:
// a NotIn or DoesNotExist selector matches an empty label set, and Resolve never matches selectors
// against a name it did not list.
func TestResolveNamesMatchesResolve(t *testing.T) {
	defaultKey := "example.com/is-default"
	objs := []client.Object{
		clusterRole("gold", map[string]string{"tier": "gold"}),
		clusterRole("silver", map[string]string{"tier": "silver"}),
		clusterRole("plain", nil),
		clusterRole("system", map[string]string{"tier": "gold", "sys": "true"}),
	}
	marked := clusterRole("marked", map[string]string{"tier": "silver"})
	marked.Annotations = map[string]string{defaultKey: "true"}
	objs = append(objs, marked)
	names := []string{"gold", "silver", "plain", "system", "marked", "ghost", "literal"}

	sel := func(key string, op metav1.LabelSelectorOperator, values ...string) *metav1.LabelSelector {
		return &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: key, Operator: op, Values: values}}}
	}
	excludedSets := map[string][]v1alpha1.ResourceFilter{
		"none":         nil,
		"names":        {{Names: []string{"silver", "ghost"}}},
		"sys-exists":   {{MatchExpressions: sel("sys", metav1.LabelSelectorOpExists).MatchExpressions}},
		"tier-notin":   {{MatchExpressions: sel("tier", metav1.LabelSelectorOpNotIn, "gold").MatchExpressions}},
		"no-tier-name": {{MatchExpressions: sel("tier", metav1.LabelSelectorOpDoesNotExist).MatchExpressions}, {Names: []string{"literal"}}},
	}
	entrySets := map[string][]v1alpha1.GrantResource{
		"none":            nil,
		"allow-names":     {{Allowed: []string{"gold", "ghost", "literal"}}},
		"allow-selector":  {{AllowedSelector: sel("tier", metav1.LabelSelectorOpIn, "gold")}},
		"deny-no-tier":    {{AvailabilityDefault: v1alpha1.AvailabilityAll, DeniedSelector: sel("tier", metav1.LabelSelectorOpDoesNotExist)}},
		"deny-notin":      {{DeniedSelector: sel("tier", metav1.LabelSelectorOpNotIn, "silver")}},
		"all-deny-names":  {{AvailabilityDefault: v1alpha1.AvailabilityAll, Denied: []string{"plain", "literal"}}},
		"none-baseline":   {{AvailabilityDefault: v1alpha1.AvailabilityNone, Allowed: []string{"silver"}}},
		"default-live":    {{Default: "gold"}},
		"default-ghost":   {{Default: "ghost", Allowed: []string{"ghost"}}},
		"default-denied":  {{Default: "plain", Denied: []string{"plain"}}},
		"default-literal": {{Default: "literal", Allowed: []string{"literal"}}},
		"two-entries": {
			{AllowedSelector: sel("tier", metav1.LabelSelectorOpExists), Default: "system"},
			{Denied: []string{"gold"}, DeniedSelector: sel("sys", metav1.LabelSelectorOpDoesNotExist)},
		},
	}
	ctx := context.Background()
	cl := newClient(t, objs...)
	for exName, excluded := range excludedSets {
		for _, availability := range []v1alpha1.AvailabilityDefault{v1alpha1.AvailabilityAll, v1alpha1.AvailabilityNone} {
			for _, defaultFrom := range []bool{false, true} {
				reg := roleReg(excluded)
				reg.Spec.DefaultAvailability = availability
				if defaultFrom {
					reg.Spec.DefaultFrom = &v1alpha1.DefaultFrom{AnnotationKey: defaultKey}
				}
				for enName, entries := range entrySets {
					full, err := Resolve(ctx, cl, testMapper(), reg, entries)
					if err != nil {
						t.Fatal(err)
					}
					for _, name := range names {
						t.Run(fmt.Sprintf("%s/%s/defaultFrom=%v/%s/%s", exName, availability, defaultFrom, enName, name), func(t *testing.T) {
							narrow, err := ResolveNames(ctx, cl, testMapper(), reg, entries, []string{name})
							if err != nil {
								t.Fatal(err)
							}
							if got, want := narrow.Decide(name), full.Decide(name); got != want {
								t.Errorf("Decide = %v, Resolve says %v", got, want)
							}
							if got, want := narrow.IsAvailable(name), full.IsAvailable(name); got != want {
								t.Errorf("IsAvailable = %v, Resolve says %v", got, want)
							}
							if got, want := narrow.Default(), full.Default(); got != want {
								t.Errorf("Default = %q, Resolve says %q", got, want)
							}
							decided, err := DecideNames(ctx, cl, testMapper(), reg, entries, []string{name})
							if err != nil {
								t.Fatal(err)
							}
							if got, want := decided.Decide(name), full.Decide(name); got != want {
								t.Errorf("DecideNames: Decide = %v, Resolve says %v", got, want)
							}
							if got, want := decided.IsAvailable(name), full.IsAvailable(name); got != want {
								t.Errorf("DecideNames: IsAvailable = %v, Resolve says %v", got, want)
							}
						})
					}
				}
			}
		}
	}
}

// TestFieldsMemoReusesUnchangedObjects checks that the memo projects an object again only when the
// object or the catalogFields change, forgets the objects that are gone, and returns what a fresh
// projection returns.
func TestFieldsMemoReusesUnchangedObjects(t *testing.T) {
	reg := roleReg(nil)
	reg.UID = types.UID("uid-1")
	reg.Generation = 1
	reg.Spec.CatalogFields = []v1alpha1.CatalogField{{Name: "tier", Path: "$.metadata.labels.tier"}}
	factory := jsonpath.NewWithCache()

	object := func(name, rv, tier string) map[string]any {
		u := &unstructured.Unstructured{}
		u.SetName(name)
		u.SetResourceVersion(rv)
		u.SetLabels(map[string]string{"tier": tier})
		return u.Object
	}
	resolved := func(objs ...map[string]any) *Resolved {
		r := newResolved(reg, nil)
		r.complete = true
		r.liveObjects = map[string]map[string]any{}
		for _, o := range objs {
			name := o["metadata"].(map[string]any)["name"].(string)
			r.liveNames = append(r.liveNames, name)
			r.liveObjects[name] = o
		}
		return r
	}
	tierOf := func(t *testing.T, r *Resolved, memo *FieldsMemo, name string) string {
		t.Helper()
		out, _ := r.AvailableWithFieldsMemo(factory, memo)
		for i := range out {
			if out[i].Name == name {
				return string(out[i].Fields["tier"].Raw)
			}
		}
		t.Fatalf("%s is not in the catalog", name)
		return ""
	}

	memo := &FieldsMemo{}
	steps := []struct {
		what string
		r    func() *Resolved
		want string
	}{
		{"first projection", func() *Resolved { return resolved(object("a", "1", "gold")) }, `"gold"`},
		// A value changed under the same resourceVersion (the API server never does that) shows the
		// projection came from the memo.
		{"same resourceVersion is reused", func() *Resolved { return resolved(object("a", "1", "silver")) }, `"gold"`},
		{"new resourceVersion is projected", func() *Resolved { return resolved(object("a", "2", "silver")) }, `"silver"`},
		{"new definition generation is projected", func() *Resolved {
			reg.Generation = 2
			return resolved(object("a", "2", "bronze"))
		}, `"bronze"`},
		{"other object only, a is forgotten", func() *Resolved { return resolved(object("b", "7", "gold")) }, ""},
		{"forgotten object is projected again", func() *Resolved { return resolved(object("a", "2", "steel")) }, `"steel"`},
	}
	for _, step := range steps {
		r := step.r()
		if step.want == "" {
			r.AvailableWithFieldsMemo(factory, memo)
			continue
		}
		if got := tierOf(t, r, memo, "a"); got != step.want {
			t.Fatalf("%s: tier = %s, want %s", step.what, got, step.want)
		}
	}
}

// TestResolveNamesTellsAMissingObjectFromAKindNotServed: a 404 Status naming the object is a missing
// object, decided by the literal lists; the plain 404 of a resource the API server does not serve (its
// CRD deleted while the mapper still maps it) is a configuration error and resets the mapper, as on the
// list of Resolve.
func TestResolveNamesTellsAMissingObjectFromAKindNotServed(t *testing.T) {
	reg := roleReg(nil)
	gr := schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: "clusterroles"}
	for _, resolveFn := range []func(context.Context, client.Reader, meta.RESTMapper, *v1alpha1.GrantableClusterResourceDefinition, []v1alpha1.GrantResource, []string) (*Resolved, error){ResolveNames, DecideNames} {
		missing := newClient(t)
		mapper := &resettableMapper{RESTMapper: testMapper()}
		r, err := resolveFn(context.Background(), missing, mapper, reg, []v1alpha1.GrantResource{{Allowed: []string{"ghost"}}}, []string{"ghost"})
		if err != nil {
			t.Fatalf("a missing object: %v", err)
		}
		if !r.Decide("ghost") || mapper.resets != 0 {
			t.Fatalf("a missing object is decided by the literal lists, without a mapper reset")
		}

		notServed := interceptor.NewClient(newClient(t).(client.WithWatch), interceptor.Funcs{
			Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
				// What client-go makes of the plain "404 page not found" the API server answers for a
				// resource it does not serve: the requested name is filled in, the body is not a Status.
				return k8serrors.NewGenericServerResponse(http.StatusNotFound, "GET", gr, "ghost", "404 page not found", 0, true)
			},
		})
		_, err = resolveFn(context.Background(), notServed, mapper, reg, nil, []string{"ghost"})
		if !errors.Is(err, ErrGrantedResourceNotServed) || !IsConfigurationError(err) {
			t.Fatalf("a 404 naming no object must be a configuration error, err=%v", err)
		}
		if mapper.resets != 1 {
			t.Fatalf("the mapper must be reset once, got %d", mapper.resets)
		}
	}
}
