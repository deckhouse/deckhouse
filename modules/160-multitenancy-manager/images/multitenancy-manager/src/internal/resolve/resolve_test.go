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
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/api/v1alpha1"
)

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		storagev1.AddToScheme, rbacv1.AddToScheme, v1alpha1.AddToScheme,
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
	resolved, err := Resolve(context.Background(), cl, reg, entries)
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
			GrantedResource:     &v1alpha1.GrantedResource{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole"},
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
	resolved, err := Resolve(context.Background(), cl, reg, nil)
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
			GrantedResource:     &v1alpha1.GrantedResource{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"},
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
