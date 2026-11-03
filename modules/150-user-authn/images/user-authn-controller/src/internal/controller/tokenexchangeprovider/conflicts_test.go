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
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
)

func TestOverlap(t *testing.T) {
	tests := []struct {
		a, b     string
		expected bool
	}{
		{"ci:", "ci:", true},
		{"ci:", "ci:a:", true},
		{"tep-a:", "tep-ab:", false},
		{"ci:", "gitlab:", false},
	}
	for _, tt := range tests {
		if got := overlap(tt.a, tt.b); got != tt.expected {
			t.Errorf("overlap(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.expected)
		}
	}
}

func namedProvider(name, uid string, created time.Time, spec map[string]any) *unstructured.Unstructured {
	u := provider(spec, Finalizer)
	u.SetName(name)
	u.SetUID(types.UID(uid))
	u.SetCreationTimestamp(metav1.NewTime(created))
	return u
}

// Two providers that give the same names: the earlier one keeps them, the later one gets no
// connector until the earlier one is gone.
func TestPrefixConflict(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	older := namedProvider("older", "aaaaaaaa-0000-0000-0000-000000000000", now.Add(-time.Hour), peeringSpec())
	newer := namedProvider("newer", "bbbbbbbb-0000-0000-0000-000000000000", now, peeringSpec())
	r, c := newTestReconciler(t, older, newer)

	for _, name := range []string{"older", "newer"} {
		if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
			t.Fatalf("Reconcile %s: %v", name, err)
		}
	}
	if _, ok := getConnector(t, c, "tep-older-aaaaaaaa"); !ok {
		t.Error("the earlier provider has no connector")
	}
	if _, ok := getConnector(t, c, "tep-newer-bbbbbbbb"); ok {
		t.Error("the later provider got a connector for names the earlier one gives")
	}
	got := controller.Object(controller.TokenExchangeProviderGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Name: "newer"}, got); err != nil {
		t.Fatal(err)
	}
	if cond := meta.FindStatusCondition(readConditions(got), conditionReady); cond == nil || cond.Reason != ReasonPrefixConflict {
		t.Errorf("Ready of the later provider = %+v", cond)
	}

	if err := c.Delete(t.Context(), older); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "older"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "newer"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := getConnector(t, c, "tep-newer-bbbbbbbb"); !ok {
		t.Error("the later provider got no connector after the earlier one was deleted")
	}
}

// A provider that lost its names for a while gets a connector of another ID back: the revocation of
// the first one refuses every token issued through it until it expires.
func TestConflictResolvedGetsANewConnectorID(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	ciSpec := func(prefix string) map[string]any {
		spec := peeringSpec()
		spec["identity"] = map[string]any{"username": map[string]any{"claim": "sub", "prefix": prefix}}
		return spec
	}
	older := namedProvider("older", "aaaaaaaa-0000-0000-0000-000000000000", now.Add(-time.Hour), ciSpec("ci-a:"))
	newer := namedProvider("newer", "bbbbbbbb-0000-0000-0000-000000000000", now, ciSpec("ci-b:"))
	r, c := newTestReconciler(t, older, newer)
	reconcileName := func(name string) {
		t.Helper()
		if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
			t.Fatalf("Reconcile %s: %v", name, err)
		}
	}
	setPrefix := func(prefix string) {
		t.Helper()
		obj := controller.Object(controller.TokenExchangeProviderGVK)
		if err := c.Get(t.Context(), types.NamespacedName{Name: "older"}, obj); err != nil {
			t.Fatal(err)
		}
		obj.Object["spec"] = ciSpec(prefix)
		if err := c.Update(t.Context(), obj); err != nil {
			t.Fatal(err)
		}
	}
	reconcileName("older")
	reconcileName("newer")
	if _, ok := getConnector(t, c, "tep-newer-bbbbbbbb"); !ok {
		t.Fatal("newer has no connector")
	}

	setPrefix("ci-b:")
	reconcileName("newer")
	if _, ok := getConnector(t, c, "tep-newer-bbbbbbbb"); ok {
		t.Fatal("newer kept its connector in the conflict")
	}

	setPrefix("ci-a:")
	reconcileName("newer")
	got := controller.Object(controller.TokenExchangeProviderGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Name: "newer"}, got); err != nil {
		t.Fatal(err)
	}
	id, _, _ := unstructured.NestedString(got.Object, "status", "connectorID")
	if id == "tep-newer-bbbbbbbb" || len(id) != len("tep-newer-bbbbbbbb") {
		t.Fatalf("connectorID after the conflict = %q, want a new one of the same length", id)
	}
	if _, ok := getConnector(t, c, id); !ok {
		t.Errorf("no connector %s", id)
	}
	if cond := meta.FindStatusCondition(readConditions(got), conditionReady); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Errorf("Ready = %+v", cond)
	}

	// The ID stays while its connector is not revoked.
	reconcileName("newer")
	if err := c.Get(t.Context(), types.NamespacedName{Name: "newer"}, got); err != nil {
		t.Fatal(err)
	}
	if again, _, _ := unstructured.NestedString(got.Object, "status", "connectorID"); again != id {
		t.Errorf("connectorID moved from %q to %q", id, again)
	}
}

// A provider whose spec turned invalid keeps its connector, but that connector still gives way when
// an earlier provider takes its names.
func TestInvalidSpecStillGivesWay(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	ciSpec := func(prefix string) map[string]any {
		spec := peeringSpec()
		spec["identity"] = map[string]any{"username": map[string]any{"claim": "sub", "prefix": prefix}}
		return spec
	}
	older := namedProvider("older", "aaaaaaaa-0000-0000-0000-000000000000", now.Add(-time.Hour), ciSpec("ci-a:"))
	newer := namedProvider("newer", "bbbbbbbb-0000-0000-0000-000000000000", now, ciSpec("ci-b:"))
	r, c := newTestReconciler(t, older, newer)
	reconcileName := func(name string) error {
		_, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
		return err
	}
	update := func(name string, spec map[string]any) {
		t.Helper()
		obj := controller.Object(controller.TokenExchangeProviderGVK)
		if err := c.Get(t.Context(), types.NamespacedName{Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		obj.Object["spec"] = spec
		if err := c.Update(t.Context(), obj); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"older", "newer"} {
		if err := reconcileName(name); err != nil {
			t.Fatal(err)
		}
	}

	// newer turns invalid: it keeps its connector with ci-b:.
	invalid := ciSpec("ci-b:")
	invalid["keys"] = map[string]any{"jwks": map[string]any{"keys": []any{map[string]any{"kty": "RSA", "n": "AQAB", "e": "AQAB", "d": "AQAB"}}}}
	update("newer", invalid)
	if err := reconcileName("newer"); err == nil {
		t.Fatal("an invalid spec reconciled without an error")
	}
	if _, ok := getConnector(t, c, "tep-newer-bbbbbbbb"); !ok {
		t.Fatal("newer lost its connector to an invalid spec")
	}

	// older takes ci-b:; newer gives way although its spec is invalid.
	update("older", ciSpec("ci-b:"))
	if err := reconcileName("older"); err != nil {
		t.Fatal(err)
	}
	if err := reconcileName("newer"); err != nil {
		t.Fatal(err)
	}
	if _, ok := getConnector(t, c, "tep-newer-bbbbbbbb"); ok {
		t.Error("newer kept a connector with the names of an earlier provider")
	}
	got := controller.Object(controller.TokenExchangeProviderGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Name: "newer"}, got); err != nil {
		t.Fatal(err)
	}
	if cond := meta.FindStatusCondition(readConditions(got), conditionReady); cond == nil || cond.Reason != ReasonPrefixConflict {
		t.Errorf("Ready = %+v", cond)
	}
}

// The mirror case: an earlier provider whose spec turned invalid keeps its connector, and a later
// one cannot take the names of that connector.
func TestLaterProviderCannotTakeTheNamesOfAnInvalidOne(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	ciSpec := func(prefix string) map[string]any {
		spec := peeringSpec()
		spec["identity"] = map[string]any{"username": map[string]any{"claim": "sub", "prefix": prefix}}
		return spec
	}
	older := namedProvider("older", "aaaaaaaa-0000-0000-0000-000000000000", now.Add(-time.Hour), ciSpec("ci-a:"))
	newer := namedProvider("newer", "bbbbbbbb-0000-0000-0000-000000000000", now, ciSpec("ci-b:"))
	r, c := newTestReconciler(t, older, newer)
	reconcileName := func(name string) error {
		_, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
		return err
	}
	update := func(name string, spec map[string]any) {
		t.Helper()
		obj := controller.Object(controller.TokenExchangeProviderGVK)
		if err := c.Get(t.Context(), types.NamespacedName{Name: name}, obj); err != nil {
			t.Fatal(err)
		}
		obj.Object["spec"] = spec
		if err := c.Update(t.Context(), obj); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"older", "newer"} {
		if err := reconcileName(name); err != nil {
			t.Fatal(err)
		}
	}

	// older turns invalid with another prefix: its connector keeps ci-a:.
	invalid := ciSpec("ci-x:")
	invalid["keys"] = map[string]any{"jwks": map[string]any{"keys": []any{map[string]any{"kty": "RSA", "n": "AQAB", "e": "AQAB", "d": "AQAB"}}}}
	update("older", invalid)
	if err := reconcileName("older"); err == nil {
		t.Fatal("an invalid spec reconciled without an error")
	}
	if _, ok := getConnector(t, c, "tep-older-aaaaaaaa"); !ok {
		t.Fatal("older lost its connector")
	}

	// newer moves to ci-a:, the names the connector of older still gives.
	update("newer", ciSpec("ci-a:"))
	if err := reconcileName("newer"); err != nil {
		t.Fatal(err)
	}
	if _, ok := getConnector(t, c, "tep-newer-bbbbbbbb"); ok {
		t.Error("newer kept a connector with the names of the connector of older")
	}
}

// A connector ID written into the status that is not one of the provider's is not taken: the
// provider would overwrite the connector of another one.
func TestForeignConnectorIDInStatus(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	ciSpec := func(prefix string) map[string]any {
		spec := peeringSpec()
		spec["identity"] = map[string]any{"username": map[string]any{"claim": "sub", "prefix": prefix}}
		return spec
	}
	victim := namedProvider("a", "aaaaaaaa-0000-0000-0000-000000000000", now.Add(-time.Hour), ciSpec("ci-a:"))
	evil := namedProvider("evil", "eeeeeeee-0000-0000-0000-000000000000", now, ciSpec("evil:"))
	evil.Object["status"] = map[string]any{"connectorID": "tep-a-aaaaaaaa"}
	r, c := newTestReconciler(t, victim, evil)
	for _, name := range []string{"a", "evil"} {
		if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
			t.Fatal(err)
		}
	}
	connector, ok := getConnector(t, c, "tep-a-aaaaaaaa")
	if !ok {
		t.Fatal("the connector of a is gone")
	}
	if connector.GetLabels()[ProviderLabel] != "a" {
		t.Errorf("the connector of a was taken over: labels %v", connector.GetLabels())
	}
	if _, ok := getConnector(t, c, "tep-evil-eeeeeeee"); !ok {
		t.Error("evil did not get a connector of its own")
	}
}

// Distinct prefixes, a shared group claim: groups with the same prefix conflict too.
func TestGroupPrefixConflict(t *testing.T) {
	a := Spec{Identity: Identity{Username: ClaimMapping{Prefix: "a:"}, Groups: []GroupMapping{{Prefix: "ci:"}}}}
	b := Spec{Identity: Identity{Username: ClaimMapping{Prefix: "b:"}, Groups: []GroupMapping{{Prefix: "ci:"}}}}
	if where := conflict(namesOf(a), namesOf(b)); where != `group prefix "ci:"` {
		t.Errorf("conflict = %q", where)
	}
	b.Identity.Groups[0].Prefix = "b:"
	if where := conflict(namesOf(a), namesOf(b)); where != "" {
		t.Errorf("conflict = %q, want none", where)
	}
}
