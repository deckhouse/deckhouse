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
	fps := []v1alpha1.FieldPath{
		{Path: "$.spec.ingressClassName"}, // unscoped fallback
		{APIVersions: []string{"v1beta1"}, Path: "$.metadata.annotations['kubernetes.io/ingress.class']"},
	}
	if fp, ok := SelectFieldPath(fps, "networking.k8s.io", "v1"); !ok || fp.Path != "$.spec.ingressClassName" {
		t.Fatalf("v1 path = %q ok=%v", fp.Path, ok)
	}
	// scoped entry wins for v1beta1.
	if fp, ok := SelectFieldPath(fps, "networking.k8s.io", "v1beta1"); !ok || fp.Path != "$.metadata.annotations['kubernetes.io/ingress.class']" {
		t.Fatalf("v1beta1 path = %q ok=%v", fp.Path, ok)
	}
	// no matching entry → ok=false.
	if _, ok := SelectFieldPath([]v1alpha1.FieldPath{{APIGroups: []string{"x"}, Path: "$.a"}}, "y", "v1"); ok {
		t.Fatal("expected no match")
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
