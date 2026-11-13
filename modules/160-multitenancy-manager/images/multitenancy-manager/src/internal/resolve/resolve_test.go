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
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
)

func testMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "storage.k8s.io", Version: "v1"},
		{Group: "rbac.authorization.k8s.io", Version: "v1"},
		{Group: "", Version: "v1"},
	})
	m.Add(schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"}, meta.RESTScopeRoot)
	m.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, meta.RESTScopeRoot)
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}, meta.RESTScopeNamespace)
	return m
}

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, storagev1.AddToScheme, rbacv1.AddToScheme, v1alpha1.AddToScheme, grantsv1alpha2.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

// valueReg is a value-backed registration (no granted objects; names are literal values).
func valueReg(defAvail v1alpha1.AvailabilityDefault, excluded []v1alpha1.ResourceFilter) *v1alpha1.GrantableClusterResourceDefinition {
	return &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "loadbalancerclasses"},
		Spec:       v1alpha1.GrantableClusterResourceDefinitionSpec{DefaultAvailability: defAvail, Excluded: excluded},
	}
}

func decide(t *testing.T, reg *v1alpha1.GrantableClusterResourceDefinition, entries []v1alpha1.GrantResource, name string, objs ...client.Object) bool {
	t.Helper()
	cl := newClient(t, objs...)
	resolved, err := Resolve(context.Background(), cl, testMapper(), reg, entries)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return resolved.Decide(name)
}

func TestDecidePrecedence(t *testing.T) {
	// excluded beats allowed.
	r := valueReg(v1alpha1.AvailabilityAll, []v1alpha1.ResourceFilter{{Names: []string{"sys"}}})
	g := []v1alpha1.GrantResource{{Allowed: []string{"sys", "standard"}}}
	if decide(t, r, g, "sys") {
		t.Fatal("excluded must deny even when allowed")
	}
	if !decide(t, r, g, "standard") {
		t.Fatal("standard must be allowed")
	}

	// denied beats allowed.
	r2 := valueReg(v1alpha1.AvailabilityAll, nil)
	g2 := []v1alpha1.GrantResource{{Allowed: []string{"x"}, Denied: []string{"x"}}}
	if decide(t, r2, g2, "x") {
		t.Fatal("denied must beat allowed")
	}

	// registration defaultAvailability None denies ungranted; All (and empty) allow.
	if decide(t, valueReg(v1alpha1.AvailabilityNone, nil), nil, "anything") {
		t.Fatal("None default must deny ungranted")
	}
	if !decide(t, valueReg(v1alpha1.AvailabilityAll, nil), nil, "anything") {
		t.Fatal("All default must allow ungranted")
	}
	if !decide(t, valueReg("", nil), nil, "anything") {
		t.Fatal("empty default must behave as All")
	}

	// grant availabilityDefault None overrides registration All.
	gNone := []v1alpha1.GrantResource{{AvailabilityDefault: v1alpha1.AvailabilityNone}}
	if decide(t, valueReg(v1alpha1.AvailabilityAll, nil), gNone, "z") {
		t.Fatal("grant availabilityDefault None must override registration All")
	}
}

func TestDecideAllowListInfersNoneBaseline(t *testing.T) {
	// The common case: registration is All, but a grant entry carries an allow-list and no explicit
	// availabilityDefault. The allow-list must restrict the resource (baseline None for everything
	// else) — this is what the alert hook previously failed to mirror.
	r := valueReg(v1alpha1.AvailabilityAll, nil)
	g := []v1alpha1.GrantResource{{Allowed: []string{"local"}}}
	if !decide(t, r, g, "local") {
		t.Fatal("allow-listed value must be allowed")
	}
	if decide(t, r, g, "replicated") {
		t.Fatal("an allow-list must restrict the resource even when the registration default is All")
	}
}

// clusterRole is an object-backed granted object (ClusterRole) carrying the given labels.
func clusterRole(name string, lbls map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: lbls}}
}

func roleReg(excluded []v1alpha1.ResourceFilter) *v1alpha1.GrantableClusterResourceDefinition {
	return &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "clusterroles"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
			DefaultAvailability: v1alpha1.AvailabilityAll,
			Excluded:            excluded,
		},
	}
}

func TestDecideExcludedUnion(t *testing.T) {
	// clusterroles pattern: available by default, but keep only the delegatable roles —
	// (kind=use) OR (module=user-authz with no kind) — by unioning two excluded filters.
	excluded := []v1alpha1.ResourceFilter{
		{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "rbac.deckhouse.io/kind", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"use"}},
			{Key: "module", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"user-authz"}},
		}},
		{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "rbac.deckhouse.io/kind", Operator: metav1.LabelSelectorOpIn, Values: []string{"manage"}},
		}},
	}
	reg := roleReg(excluded)
	cases := []struct {
		name string
		lbls map[string]string
		want bool
	}{
		{"use-role", map[string]string{"rbac.deckhouse.io/kind": "use", "module": "user-authz"}, true},
		{"use-cap-other-module", map[string]string{"rbac.deckhouse.io/kind": "use", "module": "deckhouse"}, true},
		{"userauthz-access-level", map[string]string{"module": "user-authz"}, true},
		{"manage-role", map[string]string{"rbac.deckhouse.io/kind": "manage", "module": "user-authz"}, false},
		{"generic-role", map[string]string{}, false},
		{"cluster-admin-like", map[string]string{"kubernetes.io/bootstrapping": "rbac-defaults"}, false},
	}
	objs := make([]client.Object, 0, len(cases))
	for _, c := range cases {
		objs = append(objs, clusterRole(c.name, c.lbls))
	}
	cl := newClient(t, objs...)
	resolved, err := Resolve(context.Background(), cl, testMapper(), reg, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for _, c := range cases {
		if got := resolved.Decide(c.name); got != c.want {
			t.Fatalf("%s: got available=%v want %v", c.name, got, c.want)
		}
	}
}

// The selector the module ships for clusterroles (templates/cluster-objects-controller/
// grantable-resources.yaml) keeps the delegatable roles and nothing else. The value matters: the
// admission webhook that guards the label accepts "true" and judges nothing else, so a role
// labelled "false" was never validated as delegatable and must not reach a project either. An
// exclusion by absence alone would have let exactly that role through.
func TestDecideDelegatableMarkerNeedsTheValue(t *testing.T) {
	reg := roleReg([]v1alpha1.ResourceFilter{
		{MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "rbac.deckhouse.io/delegatable", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"true"}},
		}},
	})

	cases := []struct {
		name string
		lbls map[string]string
		want bool
	}{
		{"delegatable-true", map[string]string{"rbac.deckhouse.io/delegatable": "true"}, true},
		{"delegatable-false", map[string]string{"rbac.deckhouse.io/delegatable": "false"}, false},
		{"delegatable-garbage", map[string]string{"rbac.deckhouse.io/delegatable": "yes"}, false},
		{"no-marker", map[string]string{}, false},
	}

	objs := make([]client.Object, 0, len(cases))
	for _, c := range cases {
		objs = append(objs, clusterRole(c.name, c.lbls))
	}

	resolved, err := Resolve(context.Background(), newClient(t, objs...), testMapper(), reg, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for _, c := range cases {
		if got := resolved.Decide(c.name); got != c.want {
			t.Fatalf("%s: got available=%v want %v", c.name, got, c.want)
		}
	}
}

func TestDecideAllowedSelector(t *testing.T) {
	reg := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityNone,
		},
	}
	g := []v1alpha1.GrantResource{{
		AllowedSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"shared": "true"}},
	}}
	shared := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "ssd", Labels: map[string]string{"shared": "true"}}, Provisioner: "x"}
	private := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "hdd", Labels: map[string]string{"shared": "false"}}, Provisioner: "x"}
	if !decide(t, reg, g, "ssd", shared, private) {
		t.Fatal("allowedSelector must allow the labelled object")
	}
	if decide(t, reg, g, "hdd", shared, private) {
		t.Fatal("allowedSelector must not allow the non-labelled object under None baseline")
	}
}

// TestResolveRefusesANamespacedGrantedResource: a namespaced kind is not resolved, even with a
// defaultFrom annotation that would read the objects on its own path; a cluster-scoped one still is.
func TestResolveRefusesANamespacedGrantedResource(t *testing.T) {
	reg := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "secrets"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "", Kind: "Secret"},
			DefaultAvailability: v1alpha1.AvailabilityAll,
			DefaultFrom:         &v1alpha1.DefaultFrom{AnnotationKey: "example.com/default"},
		},
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "admin", Namespace: "kube-system", Annotations: map[string]string{"example.com/default": "true"}}}
	cl := newClient(t, secret)
	if _, err := Resolve(context.Background(), cl, testMapper(), reg, nil); err == nil || !strings.Contains(err.Error(), "namespaced") {
		t.Fatalf("a namespaced grantedResource must not resolve, err=%v", err)
	}
	if _, err := defaultFromAnnotation(context.Background(), cl, testMapper(), reg); err == nil || !strings.Contains(err.Error(), "namespaced") {
		t.Fatalf("defaultFrom must not read a namespaced grantedResource, err=%v", err)
	}
	reg.Spec.GrantedResource = &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"}
	if _, err := Resolve(context.Background(), cl, testMapper(), reg, nil); err != nil {
		t.Fatalf("a cluster-scoped grantedResource must resolve: %v", err)
	}
}

// TestIsConfigurationError: the Resolve errors of a namespaced and of an unserved grantedResource are
// configuration errors; an API error and any other error are not.
func TestIsConfigurationError(t *testing.T) {
	resolveErr := func(apiGroup, kind string) error {
		reg := &v1alpha1.GrantableClusterResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "d"},
			Spec:       v1alpha1.GrantableClusterResourceDefinitionSpec{GrantedResource: &v1alpha1.GrantedResource{APIGroup: apiGroup, Kind: kind}},
		}
		_, err := Resolve(context.Background(), newClient(t), testMapper(), reg, nil)
		if err == nil {
			t.Fatalf("%s/%s must not resolve", apiGroup, kind)
		}
		return err
	}
	for name, tc := range map[string]struct {
		err  error
		want bool
	}{
		"namespaced":   {resolveErr("", "Secret"), true},
		"unknown kind": {resolveErr("example.com", "Unknown"), true},
		"not served":   {fmt.Errorf("list granted resource: %w", ErrGrantedResourceNotServed), true},
		"not found":    {fmt.Errorf("get: %w", k8serrors.NewNotFound(schema.GroupResource{Resource: "storageclasses"}, "x")), false},
		"api error":    {fmt.Errorf("list granted resource: %w", k8serrors.NewServiceUnavailable("down")), false},
		"other":        {errors.New("boom"), false},
		"nil":          {nil, false},
	} {
		if got := IsConfigurationError(tc.err); got != tc.want {
			t.Errorf("%s: IsConfigurationError(%v) = %v, want %v", name, tc.err, got, tc.want)
		}
	}
}

// resettableMapper counts the resets of a mapper.
type resettableMapper struct {
	meta.RESTMapper
	resets int
}

func (m *resettableMapper) Reset() { m.resets++ }

// TestResolveTreatsAListNotFoundAsNotServed: a mapped kind whose list answers 404 (its CRD deleted
// while the mapper still maps it) is a configuration error, and the stale mapping is dropped.
func TestResolveTreatsAListNotFoundAsNotServed(t *testing.T) {
	reg := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource: &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
		},
	}
	scheme := runtime.NewScheme()
	if err := storagev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return k8serrors.NewNotFound(schema.GroupResource{Group: "storage.k8s.io", Resource: "storageclasses"}, "")
		},
	}).Build()
	mapper := &resettableMapper{RESTMapper: testMapper()}
	_, err := Resolve(context.Background(), cl, mapper, reg, nil)
	if !errors.Is(err, ErrGrantedResourceNotServed) || !IsConfigurationError(err) {
		t.Fatalf("a list 404 must be a configuration error, err=%v", err)
	}
	if mapper.resets != 1 {
		t.Fatalf("the mapper must be reset once, got %d", mapper.resets)
	}
}

// reservedReg is an object-backed registration (ClusterRole) with production-high reserved by name and
// every object labelled reserved=true reserved by selector.
func reservedReg(defAvail v1alpha1.AvailabilityDefault, excluded []v1alpha1.ResourceFilter) *v1alpha1.GrantableClusterResourceDefinition {
	reg := roleReg(excluded)
	reg.Spec.DefaultAvailability = defAvail
	reg.Spec.Reserved = []v1alpha1.ResourceFilter{
		{Names: []string{"production-high"}},
		{MatchLabels: map[string]string{"reserved": "true"}},
	}
	return reg
}

func reservedObjects() []client.Object {
	return []client.Object{
		clusterRole("develop", nil),
		clusterRole("staging", map[string]string{"reserved": "false"}),
		clusterRole("production-high", nil),
		clusterRole("deployment-machinery", map[string]string{"reserved": "true"}),
	}
}

// availableSet resolves the registration and returns the names Decide allows among the live objects,
// checking that the catalog (Available) lists exactly the same names.
func availableSet(t *testing.T, reg *v1alpha1.GrantableClusterResourceDefinition, entries []v1alpha1.GrantResource, objs []client.Object) []string {
	t.Helper()
	resolved, err := Resolve(context.Background(), newClient(t, objs...), testMapper(), reg, entries)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	var decided []string
	for _, o := range objs {
		if resolved.Decide(o.GetName()) {
			decided = append(decided, o.GetName())
		}
	}
	slices.Sort(decided)
	catalog := []string{}
	for _, a := range resolved.Available() {
		catalog = append(catalog, a.Name)
	}
	if !slices.Equal(catalog, append([]string{}, decided...)) {
		t.Fatalf("catalog %v differs from the decided names %v", catalog, decided)
	}
	return decided
}

func TestDecideReserved(t *testing.T) {
	allowed := func(names ...string) v1alpha1.GrantResource { return v1alpha1.GrantResource{Allowed: names} }
	all := v1alpha1.AvailabilityAll
	cases := []struct {
		name     string
		defAvail v1alpha1.AvailabilityDefault
		excluded []v1alpha1.ResourceFilter
		entries  []v1alpha1.GrantResource
		want     []string
	}{
		{
			name: "no policy: reserved closed, the others open",
			want: []string{"develop", "staging"},
		},
		{
			name:    "allowed opens the reserved name and narrows the rest as any allow-list",
			entries: []v1alpha1.GrantResource{allowed("production-high")},
			want:    []string{"production-high"},
		},
		{
			name:    "allowed with availabilityDefault All adds the reserved name to the open ones",
			entries: []v1alpha1.GrantResource{{Allowed: []string{"production-high"}, AvailabilityDefault: all}},
			want:    []string{"develop", "production-high", "staging"},
		},
		{
			name:    "availabilityDefault All alone does not open reserved",
			entries: []v1alpha1.GrantResource{{AvailabilityDefault: all}},
			want:    []string{"develop", "staging"},
		},
		{
			name:    "availabilityDefault All in another entry does not open reserved",
			entries: []v1alpha1.GrantResource{allowed("develop"), {AvailabilityDefault: all}},
			want:    []string{"develop", "staging"},
		},
		{
			name: "allowedSelector opens the reserved objects it matches",
			entries: []v1alpha1.GrantResource{{
				AllowedSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"reserved": "true"}},
				AvailabilityDefault: all,
			}},
			want: []string{"deployment-machinery", "develop", "staging"},
		},
		{
			name:    "a template allow-list and a second policy allowing a reserved name add up",
			entries: []v1alpha1.GrantResource{allowed("develop", "staging"), allowed("production-high")},
			want:    []string{"develop", "production-high", "staging"},
		},
		{
			name:    "denied wins over an explicit allow of a reserved name",
			entries: []v1alpha1.GrantResource{{Allowed: []string{"production-high"}, AvailabilityDefault: all}, {Denied: []string{"production-high"}}},
			want:    []string{"develop", "staging"},
		},
		{
			name:     "excluded wins over an explicit allow of a reserved name",
			excluded: []v1alpha1.ResourceFilter{{Names: []string{"production-high"}}},
			entries:  []v1alpha1.GrantResource{{Allowed: []string{"production-high"}, AvailabilityDefault: all}},
			want:     []string{"develop", "staging"},
		},
		{
			name:     "registration None: All opens the others, reserved only by allowed",
			defAvail: v1alpha1.AvailabilityNone,
			entries:  []v1alpha1.GrantResource{{Allowed: []string{"production-high"}, AvailabilityDefault: all}},
			want:     []string{"develop", "production-high", "staging"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defAvail := c.defAvail
			if defAvail == "" {
				defAvail = v1alpha1.AvailabilityAll
			}
			got := availableSet(t, reservedReg(defAvail, c.excluded), c.entries, reservedObjects())
			if !slices.Equal(got, c.want) {
				t.Fatalf("available = %v, want %v", got, c.want)
			}
		})
	}
}

// A reserved value of a value-backed registration (no objects) is reserved by its literal name.
func TestDecideReservedValueBacked(t *testing.T) {
	reg := valueReg(v1alpha1.AvailabilityAll, nil)
	reg.Spec.Reserved = []v1alpha1.ResourceFilter{{Names: []string{"internal"}}}
	if decide(t, reg, nil, "internal") {
		t.Fatal("a reserved value must be closed without a policy")
	}
	if !decide(t, reg, nil, "public") {
		t.Fatal("a value that is not reserved must stay open")
	}
	if decide(t, reg, []v1alpha1.GrantResource{{AvailabilityDefault: v1alpha1.AvailabilityAll}}, "internal") {
		t.Fatal("availabilityDefault All must not open a reserved value")
	}
	resolved, err := Resolve(context.Background(), newClient(t), testMapper(), reg, []v1alpha1.GrantResource{{Allowed: []string{"internal"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Decide("internal") || !resolved.IsAvailable("internal") {
		t.Fatal("an explicitly allowed reserved value must be available")
	}
}

// A default naming a reserved object counts only when the project is granted the object: an entry
// that sets default alone, or a defaultFrom annotation on a reserved object, leaves no default.
func TestDefaultReserved(t *testing.T) {
	const key = "example.com/is-default"
	objs := reservedObjects()
	objs[2].SetAnnotations(map[string]string{key: "true"}) // production-high
	cases := []struct {
		name        string
		defaultFrom bool
		entries     []v1alpha1.GrantResource
		want        string
	}{
		{name: "default alone", entries: []v1alpha1.GrantResource{{Default: "production-high"}}, want: ""},
		{name: "default with availabilityDefault All", entries: []v1alpha1.GrantResource{{Default: "production-high", AvailabilityDefault: v1alpha1.AvailabilityAll}}, want: ""},
		{name: "default with allowed", entries: []v1alpha1.GrantResource{{Default: "production-high", Allowed: []string{"production-high"}, AvailabilityDefault: v1alpha1.AvailabilityAll}}, want: "production-high"},
		{name: "defaultFrom without a grant", defaultFrom: true, want: ""},
		{name: "defaultFrom with allowed", defaultFrom: true, entries: []v1alpha1.GrantResource{{Allowed: []string{"production-high"}, AvailabilityDefault: v1alpha1.AvailabilityAll}}, want: "production-high"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reg := reservedReg(v1alpha1.AvailabilityAll, nil)
			if c.defaultFrom {
				reg.Spec.DefaultFrom = &v1alpha1.DefaultFrom{AnnotationKey: key}
			}
			resolved, err := Resolve(context.Background(), newClient(t, objs...), testMapper(), reg, c.entries)
			if err != nil {
				t.Fatal(err)
			}
			if got := resolved.Default(); got != c.want {
				t.Fatalf("Default = %q, want %q", got, c.want)
			}
		})
	}
}

// An invalid reserved selector fails the resolve and is named as reserved, not excluded.
func TestResolveInvalidReservedSelector(t *testing.T) {
	reg := roleReg(nil)
	reg.Spec.Reserved = []v1alpha1.ResourceFilter{{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "k", Operator: "Bogus"}}}}
	_, err := Resolve(context.Background(), newClient(t), testMapper(), reg, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid reserved selector") {
		t.Fatalf("want an invalid reserved selector error, got %v", err)
	}
}

// The table in docs/USAGE.md, "Granting a reserved resource": keep the two in sync.
func TestUsageReservedTable(t *testing.T) {
	reg := roleReg([]v1alpha1.ResourceFilter{{Names: []string{"system"}}})
	reg.Spec.DefaultAvailability = v1alpha1.AvailabilityAll
	reg.Spec.Reserved = []v1alpha1.ResourceFilter{{Names: []string{"premium", "internal"}}}
	objs := []client.Object{
		clusterRole("standard", nil),
		clusterRole("premium", map[string]string{"tier": "critical"}),
		clusterRole("internal", nil),
		clusterRole("system", nil),
	}
	all := v1alpha1.AvailabilityAll
	cases := []struct {
		name    string
		entries []v1alpha1.GrantResource
		want    []string
	}{
		{name: "no policy", want: []string{"standard"}},
		{name: "availabilityDefault All", entries: []v1alpha1.GrantResource{{AvailabilityDefault: all}}, want: []string{"standard"}},
		{name: "allowed premium", entries: []v1alpha1.GrantResource{{Allowed: []string{"premium"}}}, want: []string{"premium"}},
		{name: "allowed premium and availabilityDefault All", entries: []v1alpha1.GrantResource{{Allowed: []string{"premium"}, AvailabilityDefault: all}}, want: []string{"premium", "standard"}},
		{
			name:    "allowedSelector matching premium",
			entries: []v1alpha1.GrantResource{{AllowedSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "critical"}}}},
			want:    []string{"premium"},
		},
		{name: "template allows standard, another policy allows premium", entries: []v1alpha1.GrantResource{{Allowed: []string{"standard"}}, {Allowed: []string{"premium"}}}, want: []string{"premium", "standard"}},
		{name: "allowed premium, another policy denies premium", entries: []v1alpha1.GrantResource{{Allowed: []string{"premium"}}, {Denied: []string{"premium"}}}, want: nil},
		{name: "allowed system", entries: []v1alpha1.GrantResource{{Allowed: []string{"system"}}}, want: nil},
		{name: "allowed system and availabilityDefault All", entries: []v1alpha1.GrantResource{{Allowed: []string{"system"}, AvailabilityDefault: all}}, want: []string{"standard"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := availableSet(t, reg, c.entries, objs); !slices.Equal(got, c.want) {
				t.Fatalf("available = %v, want %v", got, c.want)
			}
		})
	}
}
