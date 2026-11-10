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

package groupprefix

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

var start = time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

// fakeRevoker records the connectors whose tokens were deleted.
type fakeRevoker struct {
	connectors []string
	// before are the createdBefore arguments of the calls, in order.
	before []time.Time
	err    error
}

func (f *fakeRevoker) DeleteConnectorTokens(_ context.Context, connectorID string, createdBefore time.Time) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.connectors = append(f.connectors, connectorID)
	f.before = append(f.before, createdBefore)
	return 1, nil
}

// clock is a time that only moves when a test says so.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{controller.DexProviderGVK.GroupVersion(), {Version: "v1"}})
	mapper.Add(controller.DexProviderGVK, meta.RESTScopeRoot)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
	return fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(mapper).WithObjects(objs...).Build()
}

func provider(name, prefix string) *unstructured.Unstructured {
	spec := map[string]any{"type": "LDAP", "displayName": name}
	if prefix != "" {
		spec["groupPrefix"] = prefix
	}
	u := &unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"name": name}, "spec": spec}}
	u.SetGroupVersionKind(controller.DexProviderGVK)
	return u
}

type harness struct {
	t       *testing.T
	c       client.Client
	revoker *fakeRevoker
	clock   *clock
	r       *Reconciler
}

func newHarness(t *testing.T, objs ...client.Object) *harness {
	t.Helper()
	c := newClient(t, objs...)
	h := &harness{t: t, c: c, revoker: &fakeRevoker{}, clock: &clock{now: start}}
	h.r = New(c, c, h.revoker, logr.Discard(), h.clock.Now, 10*time.Minute, 3*time.Minute)
	return h
}

func (h *harness) reconcile() reconcile.Result {
	h.t.Helper()
	res, err := h.r.Reconcile(h.t.Context(), reconcile.Request{})
	if err != nil {
		h.t.Fatalf("Reconcile: %v", err)
	}
	return res
}

func (h *harness) setPrefix(name, prefix string) {
	h.t.Helper()
	obj := controller.Object(controller.DexProviderGVK)
	if err := h.c.Get(h.t.Context(), client.ObjectKey{Name: name}, obj); err != nil {
		h.t.Fatalf("get %s: %v", name, err)
	}
	if prefix == "" {
		unstructured.RemoveNestedField(obj.Object, "spec", "groupPrefix")
	} else if err := unstructured.SetNestedField(obj.Object, prefix, "spec", "groupPrefix"); err != nil {
		h.t.Fatal(err)
	}
	if err := h.c.Update(h.t.Context(), obj); err != nil {
		h.t.Fatalf("update %s: %v", name, err)
	}
}

// swept returns the connectors swept so far, and forgets them.
func (h *harness) swept() []string {
	got := slices.Clone(h.revoker.connectors)
	h.revoker.connectors = nil
	return got
}

func TestReconcile_ProvidersWithoutPrefixAreLeftAlone(t *testing.T) {
	t.Parallel()

	h := newHarness(t, provider("a", ""), provider("b", ""))
	res := h.reconcile()
	if got := h.swept(); len(got) != 0 {
		t.Fatalf("swept %v", got)
	}
	if res.RequeueAfter != 0 {
		t.Fatalf("requeue after %v", res.RequeueAfter)
	}
	h.reconcile()
	if got := h.swept(); len(got) != 0 {
		t.Fatalf("swept %v on the second pass", got)
	}
}

func TestReconcile_ChangeOfThePrefixDeletesTheTokens(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		before    string
		after     string
		wantSwept bool
	}{
		{name: "a prefix set", before: "", after: "corp:", wantSwept: true},
		{name: "a prefix changed", before: "corp:", after: "corp2:", wantSwept: true},
		{name: "a prefix removed", before: "corp:", after: "", wantSwept: true},
		{name: "no change", before: "corp:", after: "corp:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t, provider("a", tc.before), provider("b", "other:"))
			h.reconcile()
			h.swept()

			h.setPrefix("a", tc.after)
			h.reconcile()
			got := h.swept()
			if tc.wantSwept && !slices.Equal(got, []string{"a"}) {
				t.Fatalf("swept %v, want only a", got)
			}
			if !tc.wantSwept && len(got) != 0 {
				t.Fatalf("swept %v, want nothing", got)
			}
		})
	}
}

func TestReconcile_DoesNotRepeatForTheSamePrefix(t *testing.T) {
	t.Parallel()

	h := newHarness(t, provider("a", ""))
	h.reconcile()
	h.setPrefix("a", "corp:")
	h.reconcile()
	h.swept()

	h.reconcile()
	if got := h.swept(); len(got) != 0 {
		t.Fatalf("swept %v again before the delay", got)
	}
}

// A provider created with a prefix has no tokens, and one the controller learns of with a prefix may have: either
// way it is swept once, which is harmless for the first.
func TestReconcile_ProviderSeenFirstWithAPrefix(t *testing.T) {
	t.Parallel()

	h := newHarness(t, provider("a", "corp:"))
	h.reconcile()
	if got := h.swept(); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("swept %v, want a", got)
	}
}

// Dex takes a new configuration some time after the object changed, and a token issued in between has the old prefix.
func TestReconcile_SweepsOnceMoreAfterTheDelay(t *testing.T) {
	t.Parallel()

	h := newHarness(t, provider("a", ""))
	h.reconcile()
	h.setPrefix("a", "corp:")
	res := h.reconcile()
	h.swept()
	if res.RequeueAfter != 10*time.Minute {
		t.Fatalf("requeue after %v, want 10m", res.RequeueAfter)
	}

	h.clock.now = start.Add(9 * time.Minute)
	res = h.reconcile()
	if got := h.swept(); len(got) != 0 {
		t.Fatalf("swept %v before the delay", got)
	}
	if res.RequeueAfter != time.Minute {
		t.Fatalf("requeue after %v, want 1m", res.RequeueAfter)
	}

	h.clock.now = start.Add(10 * time.Minute)
	res = h.reconcile()
	if got := h.swept(); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("swept %v after the delay, want a", got)
	}
	// The first pass deleted every token of the connector, the second only those issued before the change and the
	// margin: a user who signed in after that keeps the token with the new prefix.
	if len(h.revoker.before) != 2 || !h.revoker.before[0].IsZero() || !h.revoker.before[1].Equal(start.Add(3*time.Minute)) {
		t.Fatalf("createdBefore of the passes: %v, want [zero, %v]", h.revoker.before, start.Add(3*time.Minute))
	}
	if res.RequeueAfter != 0 {
		t.Fatalf("requeue after %v, want none", res.RequeueAfter)
	}

	h.clock.now = start.Add(time.Hour)
	h.reconcile()
	if got := h.swept(); len(got) != 0 {
		t.Fatalf("swept %v a third time", got)
	}
}

// New with a zero margin and delay uses the defaults: the second pass waits 30 minutes and deletes the tokens
// issued before the change and that margin.
func TestReconcile_DefaultSecondPassWaits30Minutes(t *testing.T) {
	t.Parallel()

	h := newHarness(t, provider("a", ""))
	h.r = New(h.c, h.c, h.revoker, logr.Discard(), h.clock.Now, 0, 0)
	h.reconcile()
	h.setPrefix("a", "corp:")
	res := h.reconcile()
	h.swept()
	if res.RequeueAfter != 30*time.Minute {
		t.Fatalf("requeue after %v, want 30m", res.RequeueAfter)
	}

	h.clock.now = start.Add(30*time.Minute - time.Second)
	if _, err := h.r.Reconcile(t.Context(), reconcile.Request{}); err != nil {
		t.Fatal(err)
	}
	if got := h.swept(); len(got) != 0 {
		t.Fatalf("swept %v before the margin", got)
	}

	h.clock.now = start.Add(30 * time.Minute)
	h.reconcile()
	if got := h.swept(); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("swept %v after 30m, want a", got)
	}
	if len(h.revoker.before) != 2 || !h.revoker.before[0].IsZero() || !h.revoker.before[1].Equal(start.Add(30*time.Minute)) {
		t.Fatalf("createdBefore of the passes: %v, want [zero, %v]", h.revoker.before, start.Add(30*time.Minute))
	}
}

// A failed deletion leaves what the last pass saw, so the next pass tries again.
func TestReconcile_RetriesAfterAFailure(t *testing.T) {
	t.Parallel()

	h := newHarness(t, provider("a", ""), provider("b", ""))
	h.reconcile()
	h.setPrefix("a", "corp:")
	h.setPrefix("b", "idp:")

	h.revoker.err = errors.New("the API server is away")
	if _, err := h.r.Reconcile(t.Context(), reconcile.Request{}); err == nil {
		t.Fatal("the failure was not reported")
	}

	h.revoker.err = nil
	h.reconcile()
	got := h.swept()
	slices.Sort(got)
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("swept %v, want a and b", got)
	}
}

// What one provider's failure does not stop: the other provider's change is recorded and not repeated.
func TestReconcile_AFailureOfOneProviderDoesNotRepeatTheOther(t *testing.T) {
	t.Parallel()

	h := newHarness(t, provider("a", ""), provider("b", ""))
	h.reconcile()
	h.setPrefix("a", "corp:")
	h.setPrefix("b", "idp:")

	failing := &failFor{failing: "a", fakeRevoker: h.revoker}
	h.r.revoker = failing
	if _, err := h.r.Reconcile(t.Context(), reconcile.Request{}); err == nil {
		t.Fatal("the failure was not reported")
	}
	failing.failing = ""
	h.reconcile()
	got := h.swept()
	if !slices.Equal(got, []string{"b", "a"}) {
		t.Fatalf("swept %v, want b (first pass) and a (second)", got)
	}
}

type failFor struct {
	*fakeRevoker
	failing string
}

func (f *failFor) DeleteConnectorTokens(ctx context.Context, connectorID string, createdBefore time.Time) (int, error) {
	if connectorID == f.failing {
		return 0, errors.New("the API server is away")
	}
	return f.fakeRevoker.DeleteConnectorTokens(ctx, connectorID, createdBefore)
}

func TestReconcile_KeepsTheStateInAConfigMap(t *testing.T) {
	t.Parallel()

	h := newHarness(t, provider("a", "corp:"), provider("b", ""))
	h.reconcile()

	cm := &corev1.ConfigMap{}
	if err := h.c.Get(t.Context(), client.ObjectKey{Namespace: naming.DexNamespace, Name: StateConfigMap}, cm); err != nil {
		t.Fatalf("get the state: %v", err)
	}
	if len(cm.Data) != 2 {
		t.Fatalf("the state has %d entries, want 2: %v", len(cm.Data), cm.Data)
	}

	// A new controller reads the state and does not sweep again.
	second := New(h.c, h.c, h.revoker, logr.Discard(), h.clock.Now, 10*time.Minute, 3*time.Minute)
	h.swept()
	if _, err := second.Reconcile(t.Context(), reconcile.Request{}); err != nil {
		t.Fatal(err)
	}
	if got := h.swept(); len(got) != 0 {
		t.Fatalf("a restart swept %v", got)
	}
}

func TestReconcile_ForgetsDeletedProviders(t *testing.T) {
	t.Parallel()

	h := newHarness(t, provider("a", "corp:"), provider("b", "idp:"))
	h.reconcile()
	if err := h.c.Delete(t.Context(), provider("a", "")); err != nil {
		t.Fatal(err)
	}
	h.reconcile()

	cm := &corev1.ConfigMap{}
	if err := h.c.Get(t.Context(), client.ObjectKey{Namespace: naming.DexNamespace, Name: StateConfigMap}, cm); err != nil {
		t.Fatal(err)
	}
	if _, ok := cm.Data["a"]; ok || len(cm.Data) != 1 {
		t.Fatalf("the state of a deleted provider is kept: %v", cm.Data)
	}
}

func TestReconcile_UnreadableEntryIsRewritten(t *testing.T) {
	t.Parallel()

	state := &corev1.ConfigMap{}
	state.Namespace, state.Name = naming.DexNamespace, StateConfigMap
	state.Data = map[string]string{"a": "not json"}
	h := newHarness(t, provider("a", ""), state)
	h.reconcile()
	// The provider has no prefix, and the unreadable entry reads as none: nothing to delete, and the entry is rewritten.
	if got := h.swept(); len(got) != 0 {
		t.Fatalf("swept %v", got)
	}
	cm := &corev1.ConfigMap{}
	if err := h.c.Get(t.Context(), client.ObjectKey{Namespace: naming.DexNamespace, Name: StateConfigMap}, cm); err != nil {
		t.Fatal(err)
	}
	if cm.Data["a"] == "not json" {
		t.Fatalf("the entry was not rewritten: %v", cm.Data)
	}
}

// Every DexProvider maps to one request: a pass covers all of them, so the changes of several are one pass.
func TestPassRequest(t *testing.T) {
	t.Parallel()

	a, b := passRequest(t.Context(), provider("a", "")), passRequest(t.Context(), provider("b", "corp:"))
	if len(a) != 1 || len(b) != 1 || a[0] != b[0] {
		t.Fatalf("requests %v and %v, want the same single request", a, b)
	}
}
