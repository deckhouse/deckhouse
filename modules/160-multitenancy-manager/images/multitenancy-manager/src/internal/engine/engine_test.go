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

package engine

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
)

func factory() jsonpath.Factory { return jsonpath.NewWithCache() }

func TestRuleMatches(t *testing.T) {
	cases := []struct {
		name             string
		rule             v1alpha1.UsageRule
		group, ver, plur string
		want             bool
	}{
		{"core exact", v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}}, "", "v1", "services", true},
		{"wrong group", v1alpha1.UsageRule{APIGroups: []string{"networking.k8s.io"}, APIVersions: []string{"v1"}, Resources: []string{"ingresses"}}, "", "v1", "ingresses", false},
		{"group wildcard", v1alpha1.UsageRule{APIGroups: []string{"*"}, APIVersions: []string{"v1"}, Resources: []string{"ingresses"}}, "networking.k8s.io", "v1", "ingresses", true},
		{"version wildcard", v1alpha1.UsageRule{APIGroups: []string{"networking.k8s.io"}, APIVersions: []string{"*"}, Resources: []string{"ingresses"}}, "networking.k8s.io", "v1beta1", "ingresses", true},
		{"multi-group match", v1alpha1.UsageRule{APIGroups: []string{"networking.k8s.io", "extensions"}, APIVersions: []string{"*"}, Resources: []string{"ingresses"}}, "extensions", "v1beta1", "ingresses", true},
		{"wrong resource", v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}}, "", "v1", "pods", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RuleMatches(c.rule, c.group, c.ver, c.plur); got != c.want {
				t.Fatalf("RuleMatches=%v want %v", got, c.want)
			}
		})
	}
}

func TestSelectFieldPath(t *testing.T) {
	ref := v1alpha1.UsageReference{
		FieldPath: "$.spec.ingressClassName",
		Paths: []v1alpha1.PathOverride{
			{APIVersions: []string{"v1beta1"}, FieldPath: "$.metadata.annotations['kubernetes.io/ingress.class']"},
		},
	}
	if got := SelectFieldPath(ref, "networking.k8s.io", "v1"); got != "$.spec.ingressClassName" {
		t.Fatalf("v1 path = %q", got)
	}
	if got := SelectFieldPath(ref, "networking.k8s.io", "v1beta1"); got != "$.metadata.annotations['kubernetes.io/ingress.class']" {
		t.Fatalf("v1beta1 path = %q", got)
	}
}

func TestEvalMatch(t *testing.T) {
	obj := map[string]any{"spec": map[string]any{"type": "LoadBalancer"}}
	pred := &v1alpha1.MatchPredicate{FieldPath: "$.spec.type", Equals: "LoadBalancer"}
	ok, err := EvalMatch(factory(), pred, obj)
	if err != nil || !ok {
		t.Fatalf("expected match, ok=%v err=%v", ok, err)
	}
	pred.Equals = "ClusterIP"
	ok, _ = EvalMatch(factory(), pred, obj)
	if ok {
		t.Fatal("expected no match for ClusterIP")
	}
	// nil predicate always matches.
	if ok, _ := EvalMatch(factory(), nil, obj); !ok {
		t.Fatal("nil predicate must match")
	}
	// in[] predicate.
	predIn := &v1alpha1.MatchPredicate{FieldPath: "$.spec.type", In: []string{"NodePort", "LoadBalancer"}}
	if ok, _ := EvalMatch(factory(), predIn, obj); !ok {
		t.Fatal("expected in[] match")
	}
}

func reg(defAvail v1alpha1.AvailabilityDefault, excluded *v1alpha1.ResourceFilter) *v1alpha1.ClusterGrantableResource {
	return &v1alpha1.ClusterGrantableResource{
		Spec: v1alpha1.ClusterGrantableResourceSpec{DefaultAvailability: defAvail, Excluded: excluded},
	}
}

func TestAvailabilityPrecedence(t *testing.T) {
	// excluded beats allowed.
	r := reg(v1alpha1.AvailabilityAll, &v1alpha1.ResourceFilter{Names: []string{"sys"}})
	grants := []v1alpha1.GrantResource{{Allowed: []string{"sys", "standard"}}}
	if ok, _ := Available(r, grants, "sys", nil); ok {
		t.Fatal("excluded must deny even when allowed")
	}
	if ok, _ := Available(r, grants, "standard", nil); !ok {
		t.Fatal("standard must be allowed")
	}

	// denied beats allowed.
	r2 := reg(v1alpha1.AvailabilityAll, nil)
	g2 := []v1alpha1.GrantResource{{Allowed: []string{"x"}, Denied: []string{"x"}}}
	if ok, _ := Available(r2, g2, "x", nil); ok {
		t.Fatal("denied must beat allowed")
	}

	// defaultAvailability None with no grant decision → deny.
	rNone := reg(v1alpha1.AvailabilityNone, nil)
	if ok, _ := Available(rNone, nil, "anything", nil); ok {
		t.Fatal("None default must deny ungranted")
	}
	// defaultAvailability All with no grant → allow.
	rAll := reg(v1alpha1.AvailabilityAll, nil)
	if ok, _ := Available(rAll, nil, "anything", nil); !ok {
		t.Fatal("All default must allow ungranted")
	}
	// empty default treated as All.
	rEmpty := reg("", nil)
	if ok, _ := Available(rEmpty, nil, "anything", nil); !ok {
		t.Fatal("empty default must behave as All")
	}

	// grant availabilityDefault None overrides registration All.
	rAll2 := reg(v1alpha1.AvailabilityAll, nil)
	gNone := []v1alpha1.GrantResource{{AvailabilityDefault: v1alpha1.AvailabilityNone}}
	if ok, _ := Available(rAll2, gNone, "z", nil); ok {
		t.Fatal("grant availabilityDefault None must override registration All")
	}
}

func TestAvailabilitySelectors(t *testing.T) {
	r := reg(v1alpha1.AvailabilityNone, nil)
	grants := []v1alpha1.GrantResource{{
		AllowedSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"shared": "true"}},
	}}
	if ok, _ := Available(r, grants, "ssd", labels.Set{"shared": "true"}); !ok {
		t.Fatal("allowedSelector must allow labelled object")
	}
	if ok, _ := Available(r, grants, "ssd", labels.Set{"shared": "false"}); ok {
		t.Fatal("allowedSelector must not allow non-labelled object")
	}
	// value-backed (nil labels) ignores selectors → falls to None.
	if ok, _ := Available(r, grants, "ssd", nil); ok {
		t.Fatal("value-backed must ignore selector and deny under None")
	}
}

func TestMeasures(t *testing.T) {
	r := &v1alpha1.ClusterGrantableResource{Spec: v1alpha1.ClusterGrantableResourceSpec{
		UsageReferences: []v1alpha1.UsageReference{{
			Rule:       v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}},
			Countable:  true,
			Quantities: []v1alpha1.QuantityMeasure{{Name: "requests.storage", FieldPath: "$.spec.resources.requests.storage"}},
		}},
	}}
	ms := Measures(r)
	if len(ms) != 2 {
		t.Fatalf("expected 2 measures, got %v", ms)
	}
	var hasCount, hasQty bool
	for _, m := range ms {
		if m.Key == "persistentvolumeclaims" && m.Count {
			hasCount = true
		}
		if m.Key == "requests.storage" && !m.Count {
			hasQty = true
		}
	}
	if !hasCount || !hasQty {
		t.Fatalf("missing measures: %+v", ms)
	}
}

func TestContributions(t *testing.T) {
	r := &v1alpha1.ClusterGrantableResource{Spec: v1alpha1.ClusterGrantableResourceSpec{
		UsageReferences: []v1alpha1.UsageReference{{
			Rule:       v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}},
			FieldPath:  "$.spec.storageClassName",
			Countable:  true,
			Quantities: []v1alpha1.QuantityMeasure{{Name: "requests.storage", FieldPath: "$.spec.resources.requests.storage"}},
		}},
	}}
	obj := map[string]any{"spec": map[string]any{
		"storageClassName": "fast",
		"resources":        map[string]any{"requests": map[string]any{"storage": "10Gi"}},
	}}
	cs, err := Contributions(factory(), r, obj, "", "v1", "persistentvolumeclaims")
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 1 || cs[0].Name != "fast" {
		t.Fatalf("unexpected contributions: %+v", cs)
	}
	cnt := cs[0].Increments["persistentvolumeclaims"]
	if cnt.Value() != 1 {
		t.Fatalf("count = %v", cnt.Value())
	}
	st := cs[0].Increments["requests.storage"]
	if st.String() != "10Gi" {
		t.Fatalf("storage = %v", st.String())
	}

	// match guard false → no contribution.
	r.Spec.UsageReferences[0].Match = &v1alpha1.MatchPredicate{FieldPath: "$.spec.type", Equals: "LoadBalancer"}
	cs2, _ := Contributions(factory(), r, obj, "", "v1", "persistentvolumeclaims")
	if len(cs2) != 0 {
		t.Fatalf("guard false must yield no contributions, got %+v", cs2)
	}
}

func TestLimitFor(t *testing.T) {
	objects := map[string]map[string]resource.Quantity{
		"*":    {"requests.storage": resource.MustParse("100Gi")},
		"fast": {"requests.storage": resource.MustParse("20Gi")},
	}
	// named tighter than star → named wins.
	lim, ok := LimitFor(objects, "fast", "requests.storage")
	if !ok || lim.String() != "20Gi" {
		t.Fatalf("fast limit = %v ok=%v", lim.String(), ok)
	}
	// unknown name → star applies.
	lim2, ok := LimitFor(objects, "slow", "requests.storage")
	if !ok || lim2.String() != "100Gi" {
		t.Fatalf("slow limit = %v ok=%v", lim2.String(), ok)
	}
	// no entry → not found.
	if _, ok := LimitFor(objects, "fast", "services"); ok {
		t.Fatal("services should be unlimited (not found)")
	}
	// unlimited (-1) honored.
	objects["external"] = map[string]resource.Quantity{"services": resource.MustParse("-1")}
	lim3, ok := LimitFor(objects, "external", "services")
	if !ok || !IsUnlimited(lim3) {
		t.Fatalf("external services should be unlimited, got %v", lim3.String())
	}
}
