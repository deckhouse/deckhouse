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
	"slices"
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
	if fp, ok := SelectFieldPath(fps, "networking.k8s.io", "v1", "ingresses"); !ok || fp.Path != "$.spec.ingressClassName" {
		t.Fatalf("v1 path = %q ok=%v", fp.Path, ok)
	}
	// scoped entry wins for v1beta1.
	if fp, ok := SelectFieldPath(fps, "networking.k8s.io", "v1beta1", "ingresses"); !ok || fp.Path != "$.metadata.annotations['kubernetes.io/ingress.class']" {
		t.Fatalf("v1beta1 path = %q ok=%v", fp.Path, ok)
	}
	// no matching entry → ok=false.
	if _, ok := SelectFieldPath([]v1alpha1.FieldPath{{APIGroups: []string{"x"}, Path: "$.a"}}, "y", "v1", "zs"); ok {
		t.Fatal("expected no match")
	}
}

// TestSelectFieldPathResourceScope covers paths that collide within one group and version, which only
// the resource scope can separate.
func TestSelectFieldPathResourceScope(t *testing.T) {
	core := []v1alpha1.FieldPath{
		{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}, Path: "$.spec.priorityClassName"},
		{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"replicationcontrollers"}, Path: "$.spec.template.spec.priorityClassName"},
	}
	batch := []v1alpha1.FieldPath{
		{APIGroups: []string{"batch"}, APIVersions: []string{"v1"}, Resources: []string{"jobs"}, Path: "$.spec.template.spec.priorityClassName"},
		{APIGroups: []string{"batch"}, APIVersions: []string{"v1"}, Resources: []string{"cronjobs"}, Path: "$.spec.jobTemplate.spec.template.spec.priorityClassName"},
	}
	cases := []struct {
		name                   string
		fps                    []v1alpha1.FieldPath
		group, version, plural string
		want                   string
	}{
		{"core pods", core, "", "v1", "pods", "$.spec.priorityClassName"},
		{"core replicationcontrollers", core, "", "v1", "replicationcontrollers", "$.spec.template.spec.priorityClassName"},
		{"batch jobs", batch, "batch", "v1", "jobs", "$.spec.template.spec.priorityClassName"},
		{"batch cronjobs", batch, "batch", "v1", "cronjobs", "$.spec.jobTemplate.spec.template.spec.priorityClassName"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fp, ok := SelectFieldPath(c.fps, c.group, c.version, c.plural)
			if !ok || fp.Path != c.want {
				t.Fatalf("path = %q ok=%v, want %q", fp.Path, ok, c.want)
			}
		})
	}
	// A resource outside every scoped entry has no fallback here → ok=false.
	if _, ok := SelectFieldPath(core, "", "v1", "services"); ok {
		t.Fatal("expected no match for services")
	}
}

// TestSelectFieldPathRanking pins the specificity order: resources > apiGroups > apiVersions, with the
// unscoped entry as the last resort and the lowest index breaking ties.
func TestSelectFieldPathRanking(t *testing.T) {
	// Every entry matches core/v1 pods; they differ only in how specific their scope is.
	fps := []v1alpha1.FieldPath{
		{Path: "$.unscoped"},
		{APIVersions: []string{"v1"}, Path: "$.byVersion"},
		{APIGroups: []string{""}, Path: "$.byGroup"},
		{Resources: []string{"pods"}, Path: "$.byResource"},
	}
	if fp, ok := SelectFieldPath(fps, "", "v1", "pods"); !ok || fp.Path != "$.byResource" {
		t.Fatalf("resource scope must win: path = %q ok=%v", fp.Path, ok)
	}
	if fp, ok := SelectFieldPath(fps[:3], "", "v1", "pods"); !ok || fp.Path != "$.byGroup" {
		t.Fatalf("group scope must beat version scope: path = %q ok=%v", fp.Path, ok)
	}
	if fp, ok := SelectFieldPath(fps[:2], "", "v1", "pods"); !ok || fp.Path != "$.byVersion" {
		t.Fatalf("version scope must beat the unscoped entry: path = %q ok=%v", fp.Path, ok)
	}
	// Resources alone outranks apiGroups+apiVersions together (4 > 2+1).
	both := []v1alpha1.FieldPath{
		{APIGroups: []string{""}, APIVersions: []string{"v1"}, Path: "$.byGroupAndVersion"},
		{Resources: []string{"pods"}, Path: "$.byResource"},
	}
	if fp, ok := SelectFieldPath(both, "", "v1", "pods"); !ok || fp.Path != "$.byResource" {
		t.Fatalf("resource scope must outrank group+version: path = %q ok=%v", fp.Path, ok)
	}
	// Unscoped fallback still applies when no scoped entry matches the request.
	fallback := []v1alpha1.FieldPath{
		{Resources: []string{"pods"}, Path: "$.byResource"},
		{Path: "$.unscoped"},
	}
	if fp, ok := SelectFieldPath(fallback, "", "v1", "services"); !ok || fp.Path != "$.unscoped" {
		t.Fatalf("unscoped fallback: path = %q ok=%v", fp.Path, ok)
	}
	// Equal weights → the earliest entry wins.
	tie := []v1alpha1.FieldPath{
		{Resources: []string{"pods"}, Path: "$.first"},
		{Resources: []string{"pods", "services"}, Path: "$.second"},
	}
	if fp, ok := SelectFieldPath(tie, "", "v1", "pods"); !ok || fp.Path != "$.first" {
		t.Fatalf("tie must go to the first entry: path = %q ok=%v", fp.Path, ok)
	}
}

// TestSelectFieldPathScopeEdges pins how wildcards, multi-item and empty lists weigh in selection.
func TestSelectFieldPathScopeEdges(t *testing.T) {
	cases := []struct {
		name                   string
		fps                    []v1alpha1.FieldPath
		group, version, plural string
		want                   string
	}{
		{
			"resources wildcard does not beat explicit group+version",
			[]v1alpha1.FieldPath{
				{Resources: []string{"*"}, Path: "$.wildcard"},
				{APIGroups: []string{"batch"}, APIVersions: []string{"v1"}, Path: "$.explicit"},
			},
			"batch", "v1", "jobs", "$.explicit",
		},
		{
			"resources wildcard does not tie with an explicit resource",
			[]v1alpha1.FieldPath{
				{Resources: []string{"*"}, Path: "$.wildcard"},
				{Resources: []string{"pods"}, Path: "$.explicit"},
			},
			"", "v1", "pods", "$.explicit",
		},
		{
			"apiGroups wildcard ties with unscoped, first wins",
			[]v1alpha1.FieldPath{
				{APIGroups: []string{"*"}, Path: "$.wildcard"},
				{Path: "$.unscoped"},
			},
			"batch", "v1", "jobs", "$.wildcard",
		},
		{
			"unscoped ties with apiGroups wildcard, first wins",
			[]v1alpha1.FieldPath{
				{Path: "$.unscoped"},
				{APIGroups: []string{"*"}, Path: "$.wildcard"},
			},
			"batch", "v1", "jobs", "$.unscoped",
		},
		{
			"two-resource list matches the first resource",
			[]v1alpha1.FieldPath{
				{Path: "$.unscoped"},
				{Resources: []string{"jobs", "cronjobs"}, Path: "$.scoped"},
			},
			"batch", "v1", "jobs", "$.scoped",
		},
		{
			"two-resource list matches the second resource",
			[]v1alpha1.FieldPath{
				{Path: "$.unscoped"},
				{Resources: []string{"jobs", "cronjobs"}, Path: "$.scoped"},
			},
			"batch", "v1", "cronjobs", "$.scoped",
		},
		{
			"group matches but resource does not, unscoped fallback applies",
			[]v1alpha1.FieldPath{
				{APIGroups: []string{"batch"}, Resources: []string{"cronjobs"}, Path: "$.scoped"},
				{Path: "$.unscoped"},
			},
			"batch", "v1", "jobs", "$.unscoped",
		},
		{
			"empty non-nil resources behaves as unrestricted",
			[]v1alpha1.FieldPath{
				{Resources: []string{}, Path: "$.empty"},
				{Path: "$.unscoped"},
			},
			"", "v1", "pods", "$.empty",
		},
		{
			"empty non-nil resources loses to an explicit version",
			[]v1alpha1.FieldPath{
				{Resources: []string{}, Path: "$.empty"},
				{APIVersions: []string{"v1"}, Path: "$.byVersion"},
			},
			"", "v1", "pods", "$.byVersion",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fp, ok := SelectFieldPath(c.fps, c.group, c.version, c.plural)
			if !ok || fp.Path != c.want {
				t.Fatalf("path = %q ok=%v, want %q", fp.Path, ok, c.want)
			}
		})
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

func TestParsePathSegments(t *testing.T) {
	cases := []struct {
		path string
		want []string // nil = ok=false
	}{
		// Not a single member chain, or not RFC 9535 at all.
		{"$", nil},
		{"$.", nil},
		{"$.a.", nil},
		{"$..a", nil},
		{"$.a[*]", nil},
		{"$.a[0]", nil},
		{"$.a[?(@.x)]", nil},
		{"$.*", nil},
		{"$.a['b", nil},
		{`$.a["b']`, nil},
		{"a.b", nil},
		{"$['a','b']", nil},
		{"$['']", nil},   // RFC accepts it, but it would patch an empty root key
		{"$.a['']", nil}, // same, one level down
		// Shorthand names follow the RFC 9535 name-first/name-char rules.
		{"$.a-b", nil},
		{"$.a b", nil},
		{"$.1abc", nil},
		{`$['a\q']`, nil}, // an escape RFC 9535 does not define
		// Member chains.
		{"$['a']", []string{"a"}},
		{`$["a"]`, []string{"a"}},
		{"$.a['b.c']", []string{"a", "b.c"}},
		{"$.spec.storageClassName", []string{"spec", "storageClassName"}},
		{"$.metadata.annotations['cert-manager.io/cluster-issuer']", []string{"metadata", "annotations", "cert-manager.io/cluster-issuer"}},
		{"$._a1.b_", []string{"_a1", "b_"}},
		// Escapes in quotes are decoded the way the evaluator decodes them, not kept literally.
		{`$['a\u0041']`, []string{"aA"}},
		{`$['a\/b']`, []string{"a/b"}},
		{`$['a\\b']`, []string{`a\b`}},
		{`$['it\'s']`, []string{"it's"}},
	}
	f := factory()
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got, ok := ParsePathSegments(f, c.path)
			if ok != (c.want != nil) || !slices.Equal(got, c.want) {
				t.Fatalf("ParsePathSegments(%q) = %q, %v; want %q", c.path, got, ok, c.want)
			}
			if !ok {
				return
			}
			// Property: an accepted path is one the RFC 9535 evaluator accepts too, and it selects the
			// very field the segments name — the one /defaults would patch.
			parsed, err := f.Path(c.path)
			if err != nil {
				t.Fatalf("accepted %q, but the RFC 9535 parser refuses it: %v", c.path, err)
			}
			var obj any = "target"
			for i := len(got) - 1; i >= 0; i-- {
				obj = map[string]any{got[i]: obj}
			}
			if nodes := parsed.Select(obj); len(nodes) != 1 || nodes[0] != "target" {
				t.Fatalf("%q with segments %q selects %v, want the field the segments name", c.path, got, nodes)
			}
		})
	}
}

func TestDefaultingActive(t *testing.T) {
	cases := map[v1alpha1.DefaultingMode]bool{
		"":                           false,
		v1alpha1.DefaultingNone:      false,
		v1alpha1.DefaultingFillEmpty: true,
		v1alpha1.DefaultingCoerce:    true,
	}
	for mode, want := range cases {
		if got := DefaultingActive(v1alpha1.FieldPath{Path: "$.a", Defaulting: mode}); got != want {
			t.Errorf("DefaultingActive(%q) = %v, want %v", mode, got, want)
		}
	}
}
