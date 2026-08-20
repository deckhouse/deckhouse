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

package providercheck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
)

func TestReconcile_PendingThenSucceeded(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	httpFake := newFakeHTTP()
	httpFake.set(dexDiscoveryURL, http.StatusOK, `{"issuer":"https://dex.d8-user-authn"}`)
	httpFake.set("https://api.github.com/meta", http.StatusOK, `{}`)

	provider := githubProvider("github", 3)
	check := newCheck("github", "github")
	r := newTestReconciler(t, now, httpFake, &fakeLDAP{}, provider, check)

	res, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "github"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if res.RequeueAfter != recheckInterval {
		t.Fatalf("RequeueAfter = %s, want %s", res.RequeueAfter, recheckInterval)
	}

	got := getCheck(t, r, "github")
	if got.Status.Phase != DexProviderCheckPhaseSucceeded {
		t.Fatalf("phase = %q, want Succeeded", got.Status.Phase)
	}
	if got.Status.ObservedDexProviderGeneration != 3 {
		t.Fatalf("observed generation = %d, want 3", got.Status.ObservedDexProviderGeneration)
	}
	if got.Status.CompletedAt == nil || !got.Status.CompletedAt.Time.Equal(now) {
		t.Fatalf("completedAt = %v, want %s", got.Status.CompletedAt, now)
	}
	assertStep(t, got, "providerExists", stepSucceeded)
	assertStep(t, got, "providerEnabled", stepSucceeded)
	assertStep(t, got, "dexReady", stepSucceeded)
	assertStep(t, got, "githubAPI", stepSucceeded)
	assertStep(t, got, "githubCredentials", stepSkipped)

	calls := httpFake.callCount()
	res, err = r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "github"}})
	if err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if res.RequeueAfter != recheckInterval {
		t.Fatalf("second RequeueAfter = %s, want %s", res.RequeueAfter, recheckInterval)
	}
	if httpFake.callCount() != calls {
		t.Fatalf("second reconcile re-ran probes: calls %d -> %d", calls, httpFake.callCount())
	}
}

func TestReconcile_FailedProbe(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	httpFake := newFakeHTTP()
	httpFake.setErr(dexDiscoveryURL, errors.New("connection refused"))
	httpFake.set("https://api.github.com/meta", http.StatusOK, `{}`)

	r := newTestReconciler(t, now, httpFake, &fakeLDAP{}, githubProvider("github", 1), newCheck("github", "github"))
	res, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "github"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if res.RequeueAfter != recheckInterval {
		t.Fatalf("RequeueAfter = %s, want %s", res.RequeueAfter, recheckInterval)
	}

	got := getCheck(t, r, "github")
	if got.Status.Phase != DexProviderCheckPhaseFailed {
		t.Fatalf("phase = %q, want Failed", got.Status.Phase)
	}
	assertStep(t, got, "dexReady", stepFailed)
	if !strings.Contains(got.Status.Message, "Dex discovery is not reachable") {
		t.Fatalf("message = %q", got.Status.Message)
	}
}

func TestReconcile_SkipSteps(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	httpFake := newFakeHTTP()
	httpFake.set(dexDiscoveryURL, http.StatusOK, `{"issuer":"https://dex.d8-user-authn"}`)

	provider := ldapProvider("ldap", 1, map[string]any{
		"host":          "ldap.example.com",
		"insecureNoSSL": true,
	})
	r := newTestReconciler(t, now, httpFake, &fakeLDAP{}, provider, newCheck("ldap", "ldap"))
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "ldap"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	got := getCheck(t, r, "ldap")
	if got.Status.Phase != DexProviderCheckPhaseSucceeded {
		t.Fatalf("phase = %q, want Succeeded; checks=%v", got.Status.Phase, got.Status.Checks)
	}
	assertStep(t, got, "ldapCertificate", stepSkipped)
	assertStep(t, got, "ldapBind", stepSkipped)
	assertStep(t, got, "ldapKerberosKeytab", stepSkipped)
	assertStep(t, got, "ldapCABundle", stepSkipped)
}

func TestReconcile_AcknowledgedWarnings(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	httpFake := newFakeHTTP()
	httpFake.set(dexDiscoveryURL, http.StatusOK, `{"issuer":"https://dex.d8-user-authn"}`)

	tests := []struct {
		name   string
		annots map[string]string
		want   string
	}{
		{name: "unacknowledged stays warning", want: stepWarning},
		{
			name:   "named step acknowledged",
			annots: map[string]string{acknowledgedWarningsAnnotation: "ldapCertificate"},
			want:   stepSucceeded,
		},
		{
			name:   "wildcard acknowledges all",
			annots: map[string]string{acknowledgedWarningsAnnotation: "*"},
			want:   stepSucceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			provider := ldapProvider("ldap", 1, map[string]any{
				"host":               "ldap.example.com",
				"insecureSkipVerify": true,
			})
			if tt.annots != nil {
				provider.SetAnnotations(tt.annots)
			}
			httpLocal := newFakeHTTP()
			httpLocal.set(dexDiscoveryURL, http.StatusOK, `{"issuer":"https://dex.d8-user-authn"}`)
			r := newTestReconciler(t, now, httpLocal, &fakeLDAP{hasTLS: true}, provider, newCheck("ldap", "ldap"))
			if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "ldap"}}); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			got := getCheck(t, r, "ldap")
			assertStep(t, got, "ldapCertificate", tt.want)
			if tt.want == stepSucceeded {
				step := stepByName(got, "ldapCertificate")
				if !strings.Contains(step.Message, "acknowledged via annotation") {
					t.Fatalf("acknowledged message missing: %q", step.Message)
				}
			}
		})
	}
}

func TestReconcile_DeletesOrphanAndDuplicate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	httpFake := newFakeHTTP()

	t.Run("orphan when provider is missing", func(t *testing.T) {
		t.Parallel()
		r := newTestReconciler(t, now, httpFake, &fakeLDAP{}, newCheck("gone", "gone"))
		if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "gone"}}); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		err := r.client.Get(t.Context(), types.NamespacedName{Name: "gone"}, &DexProviderCheck{})
		if !apierrors.IsNotFound(err) {
			t.Fatalf("want NotFound, got %v", err)
		}
	})

	t.Run("duplicate non-canonical name", func(t *testing.T) {
		t.Parallel()
		r := newTestReconciler(t, now, httpFake, &fakeLDAP{}, githubProvider("github", 1), newCheck("old-random", "github"))
		if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "old-random"}}); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		err := r.client.Get(t.Context(), types.NamespacedName{Name: "old-random"}, &DexProviderCheck{})
		if !apierrors.IsNotFound(err) {
			t.Fatalf("want NotFound, got %v", err)
		}
	})
}

func TestReconcile_HonorsCanceledContext(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, time.Now(), newFakeHTTP(), &fakeLDAP{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "x"}})
	if err == nil {
		t.Fatal("Reconcile with canceled context: want error")
	}
}

func TestReconcile_CancelsInFlightProbe(t *testing.T) {
	t.Parallel()

	httpFake := newFakeHTTP()
	httpFake.delay = time.Hour
	httpFake.set(dexDiscoveryURL, http.StatusOK, `{"issuer":"https://dex.d8-user-authn"}`)

	r := newTestReconciler(t, time.Now(), httpFake, &fakeLDAP{}, githubProvider("github", 1), newCheck("github", "github"))
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "github"}})
	if err == nil {
		t.Fatal("Reconcile with canceled probe: want error")
	}

	got := getCheck(t, r, "github")
	if got.Status.Phase != DexProviderCheckPhasePending {
		t.Fatalf("phase after cancel = %q, want Pending", got.Status.Phase)
	}
}

func TestReconcile_DisabledProviderFails(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	provider := githubProvider("github", 42)
	_ = unstructured.SetNestedField(provider.Object, false, "spec", "enabled")
	r := newTestReconciler(t, now, newFakeHTTP(), &fakeLDAP{}, provider, newCheck("github", "github"))
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "github"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	got := getCheck(t, r, "github")
	if got.Status.Phase != DexProviderCheckPhaseFailed {
		t.Fatalf("phase = %q, want Failed", got.Status.Phase)
	}
	if got.Status.ObservedDexProviderGeneration != 42 {
		t.Fatalf("observed generation = %d, want 42", got.Status.ObservedDexProviderGeneration)
	}
	assertStep(t, got, "providerEnabled", stepFailed)
}

func TestReconcile_StatusOmitsSecrets(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	httpFake := newFakeHTTP()
	httpFake.set(dexDiscoveryURL, http.StatusOK, `{"issuer":"https://dex.d8-user-authn"}`)
	httpFake.set("https://github.com/login/oauth/access_token", http.StatusUnauthorized, `{"error":"invalid_client"}`)
	httpFake.set("https://api.github.com/meta", http.StatusOK, `{}`)

	provider := githubProvider("github", 1)
	_ = unstructured.SetNestedField(provider.Object, "super-secret-xyz", "spec", "github", "clientSecret")
	r := newTestReconciler(t, now, httpFake, &fakeLDAP{}, provider, newCheck("github", "github"))
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "github"}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	got := getCheck(t, r, "github")
	raw, err := json.Marshal(got.Status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if strings.Contains(string(raw), "super-secret-xyz") {
		t.Fatalf("status leaked client secret: %s", raw)
	}
}

func newTestReconciler(t *testing.T, now time.Time, httpFake HTTPFactory, ldapFake LDAPDialer, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(newTestMapper()).
		WithStatusSubresource(&DexProviderCheck{}).
		WithObjects(objs...).
		Build()
	return New(c, c, logr.Discard(), httpFake, ldapFake, func() time.Time { return now })
}

func newTestMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "deckhouse.io", Version: "v1"},
		{Group: "", Version: "v1"},
	})
	m.Add(controller.DexProviderGVK, meta.RESTScopeRoot)
	m.Add(dexProviderCheckGV.WithKind(dexProviderCheckKind), meta.RESTScopeRoot)
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}, meta.RESTScopeNamespace)
	return m
}

func newCheck(name, providerName string) *DexProviderCheck {
	return &DexProviderCheck{
		TypeMeta: metav1.TypeMeta{
			APIVersion: dexProviderAPIVersion,
			Kind:       dexProviderCheckKind,
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       DexProviderCheckSpec{ProviderName: providerName},
	}
}

func githubProvider(name string, generation int64) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": dexProviderAPIVersion,
		"kind":       dexProviderKind,
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"type": "Github",
			"github": map[string]any{
				"clientID": "client",
			},
		},
	}}
	u.SetGroupVersionKind(controller.DexProviderGVK)
	u.SetGeneration(generation)
	return u
}

func ldapProvider(name string, generation int64, ldap map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": dexProviderAPIVersion,
		"kind":       dexProviderKind,
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"type": "LDAP",
			"ldap": ldap,
		},
	}}
	u.SetGroupVersionKind(controller.DexProviderGVK)
	u.SetGeneration(generation)
	return u
}

func getCheck(t *testing.T, r *Reconciler, name string) *DexProviderCheck {
	t.Helper()
	got := &DexProviderCheck{}
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, got); err != nil {
		t.Fatalf("get check: %v", err)
	}
	return got
}

func stepByName(check *DexProviderCheck, name string) DexProviderCheckStepStatus {
	for _, step := range check.Status.Checks {
		if step.Name == name {
			return step
		}
	}
	return DexProviderCheckStepStatus{}
}

func assertStep(t *testing.T, check *DexProviderCheck, name, wantStatus string) {
	t.Helper()
	step := stepByName(check, name)
	if step.Name == "" {
		t.Fatalf("missing step %q in %#v", name, check.Status.Checks)
	}
	if step.Status != wantStatus {
		t.Fatalf("step %q status = %q, want %q (message=%q)", name, step.Status, wantStatus, step.Message)
	}
}
