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

package useraccount

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

func TestReconcile_LocalPasswordProjectsAttemptsLockEmail(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	until := now.Add(2 * time.Hour).UTC().Format(time.RFC3339)
	pw := passwordUnstructured("pw-jane", map[string]any{
		"email":                          "jane@example.com",
		"username":                       "jane",
		"userID":                         "jane",
		"hash":                           "secret-hash",
		"previousHashes":                 []any{"old-hash"},
		"incorrectPasswordLoginAttempts": int64(3),
		"lockedUntil":                    until,
	})
	user := userUnstructured("jane", "jane@example.com", []string{"admins"}, "")

	r := newTestReconciler(t, now, pw, user)
	name := naming.LocalName("jane@example.com")
	res, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if res.RequeueAfter != 2*time.Hour {
		t.Errorf("RequeueAfter = %v, want 2h so the account unlocks when lockedUntil expires", res.RequeueAfter)
	}

	var ua v1alpha1.UserAccount
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua); err != nil {
		t.Fatalf("get useraccount: %v", err)
	}
	if ua.Status.Email != "jane@example.com" {
		t.Errorf("email = %q", ua.Status.Email)
	}
	if ua.Status.IncorrectLoginAttempts != 3 {
		t.Errorf("attempts = %d, want 3 (from Password)", ua.Status.IncorrectLoginAttempts)
	}
	if !ua.Status.Locked {
		t.Error("locked = false, want true")
	}
	if ua.Labels[v1alpha1.LabelLocked] != "true" {
		t.Errorf("locked label = %q", ua.Labels[v1alpha1.LabelLocked])
	}
	if ua.Status.UserRef != "jane" {
		t.Errorf("userRef = %q", ua.Status.UserRef)
	}

	raw, err := json.Marshal(ua)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	assertNoSecretKeys(t, raw)
}

func TestReconcile_ExpiredLockedUntilUnlocked(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour).UTC().Format(time.RFC3339)

	tests := []struct {
		name       string
		annots     map[string]any
		wantLocked bool
	}{
		{name: "expired lock", wantLocked: false},
		{
			name:       "expired lock with admin annotation",
			annots:     map[string]any{lockedByAdministratorAnnot: ""},
			wantLocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			meta := map[string]any{"name": "pw-exp", "namespace": naming.DexNamespace}
			if tt.annots != nil {
				meta["annotations"] = tt.annots
			}
			pw := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion":  controller.PasswordGVK.GroupVersion().String(),
				"kind":        controller.PasswordGVK.Kind,
				"metadata":    meta,
				"email":       "exp@example.com",
				"username":    "exp",
				"lockedUntil": expired,
			}}
			r := newTestReconciler(t, now, pw)
			name := naming.LocalName("exp@example.com")
			res, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
			if err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			if res.RequeueAfter != 0 {
				t.Errorf("RequeueAfter = %v, want 0 after lock expiry", res.RequeueAfter)
			}
			var ua v1alpha1.UserAccount
			if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua); err != nil {
				t.Fatalf("get: %v", err)
			}
			if ua.Status.Locked != tt.wantLocked {
				t.Errorf("locked = %v, want %v", ua.Status.Locked, tt.wantLocked)
			}
			wantLabel := "false"
			if tt.wantLocked {
				wantLabel = "true"
			}
			if ua.Labels[v1alpha1.LabelLocked] != wantLabel {
				t.Errorf("locked label = %q, want %q", ua.Labels[v1alpha1.LabelLocked], wantLabel)
			}
		})
	}
}

func TestReconcile_ExternalOnlyLDAPOrCrowd(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		providerType string
		wantCreate   bool
	}{
		{name: "LDAP provider", providerType: "LDAP", wantCreate: true},
		{name: "Crowd provider", providerType: "Crowd", wantCreate: true},
		{name: "OIDC provider ignored", providerType: "OIDC", wantCreate: false},
		{name: "Github provider ignored", providerType: "Github", wantCreate: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			connID := "conn-" + tt.providerType
			sess := sessionUnstructured("sess-"+tt.providerType, map[string]any{
				"email":                          "alice@example.com",
				"userID":                         "alice",
				"connID":                         connID,
				"connectorData":                  "secret",
				"refresh":                        map[string]any{"t": map[string]any{"ID": "1"}},
				"totp":                           "secret-totp",
				"incorrectPasswordLoginAttempts": int64(2),
			})
			provider := providerUnstructured(connID, tt.providerType)
			r := newTestReconciler(t, now, sess, provider)
			name := naming.ExternalName(connID, "alice")
			if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			var ua v1alpha1.UserAccount
			err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua)
			if tt.wantCreate {
				if err != nil {
					t.Fatalf("expected useraccount: %v", err)
				}
				if ua.Status.IncorrectLoginAttempts != 2 {
					t.Errorf("attempts = %d, want 2 (from OfflineSessions)", ua.Status.IncorrectLoginAttempts)
				}
				if ua.Status.ProviderType != tt.providerType {
					t.Errorf("providerType = %q", ua.Status.ProviderType)
				}
				raw, marshalErr := json.Marshal(ua)
				if marshalErr != nil {
					t.Fatalf("marshal: %v", marshalErr)
				}
				assertNoSecretKeys(t, raw)
				return
			}
			if !apierrors.IsNotFound(err) {
				t.Fatalf("Get error = %v, want NotFound", err)
			}
		})
	}
}

func TestReconcile_OfflineSessionsLocalConnIDIgnored(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	sess := sessionUnstructured("sess-local", map[string]any{
		"email":  "local@example.com",
		"userID": "local-user",
		"connID": "local",
	})
	provider := providerUnstructured("local", "LDAP")
	r := newTestReconciler(t, now, sess, provider)
	name := naming.ExternalName("local", "local-user")
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	var ua v1alpha1.UserAccount
	err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Get error = %v, want NotFound", err)
	}
}

func TestReconcile_DeletePasswordDeletesUserAccount(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	pw := passwordUnstructured("pw-del", map[string]any{
		"email":    "gone@example.com",
		"username": "gone",
	})
	r := newTestReconciler(t, now, pw)
	name := naming.LocalName("gone@example.com")
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
	if _, err := r.Reconcile(t.Context(), req); err != nil {
		t.Fatalf("first Reconcile: %v", err)
	}
	if err := r.client.Delete(t.Context(), pw); err != nil {
		t.Fatalf("delete password: %v", err)
	}
	if _, err := r.Reconcile(t.Context(), req); err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	var ua v1alpha1.UserAccount
	err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Get error = %v, want NotFound after password delete", err)
	}
}

func TestReconcile_SkipIfEqualDoesNotUpdate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	pw := passwordUnstructured("pw-skip", map[string]any{
		"email":                          "skip@example.com",
		"username":                       "skip",
		"incorrectPasswordLoginAttempts": int64(1),
	})

	var statusUpdates atomic.Int32
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(newTestMapper()).
		WithStatusSubresource(&v1alpha1.UserAccount{}).
		WithObjects(pw).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(ctx context.Context, cl client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				if subResourceName == "status" {
					statusUpdates.Add(1)
				}
				return cl.SubResource(subResourceName).Update(ctx, obj, opts...)
			},
		}).
		Build()

	r := New(c, logr.Discard(), func() time.Time { return now })
	name := naming.LocalName("skip@example.com")
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
	if _, err := r.Reconcile(t.Context(), req); err != nil {
		t.Fatalf("first Reconcile: %v", err)
	}
	if statusUpdates.Load() != 1 {
		t.Fatalf("status updates after create = %d, want 1", statusUpdates.Load())
	}

	var ua v1alpha1.UserAccount
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua); err != nil {
		t.Fatalf("get: %v", err)
	}
	rv := ua.ResourceVersion

	if _, err := r.Reconcile(t.Context(), req); err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if statusUpdates.Load() != 1 {
		t.Fatalf("status updates after skip-if-equal = %d, want 1", statusUpdates.Load())
	}
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua); err != nil {
		t.Fatalf("get after second: %v", err)
	}
	if ua.ResourceVersion != rv {
		t.Errorf("resourceVersion changed from %q to %q", rv, ua.ResourceVersion)
	}
}

func TestReconcile_HonorsCanceledContext(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, time.Now())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "x"}})
	if err == nil {
		t.Fatal("Reconcile with canceled context: want error")
	}
}

func newTestReconciler(t *testing.T, now time.Time, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(newTestMapper()).
		WithStatusSubresource(&v1alpha1.UserAccount{}).
		WithObjects(objs...).
		Build()
	return New(c, logr.Discard(), func() time.Time { return now })
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return scheme
}

func newTestMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "dex.coreos.com", Version: "v1"},
		{Group: "deckhouse.io", Version: "v1"},
		v1alpha1.SchemeGroupVersion,
	})
	m.Add(controller.PasswordGVK, meta.RESTScopeNamespace)
	m.Add(controller.OfflineSessionsGVK, meta.RESTScopeNamespace)
	m.Add(controller.DexProviderGVK, meta.RESTScopeRoot)
	m.Add(controller.UserGVK, meta.RESTScopeRoot)
	m.Add(v1alpha1.SchemeGroupVersion.WithKind("UserAccount"), meta.RESTScopeRoot)
	return m
}

func passwordUnstructured(name string, fields map[string]any) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": controller.PasswordGVK.GroupVersion().String(),
		"kind":       controller.PasswordGVK.Kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": naming.DexNamespace,
		},
	}
	for k, v := range fields {
		obj[k] = v
	}
	return &unstructured.Unstructured{Object: obj}
}

func sessionUnstructured(name string, fields map[string]any) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": controller.OfflineSessionsGVK.GroupVersion().String(),
		"kind":       controller.OfflineSessionsGVK.Kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": naming.DexNamespace,
		},
	}
	for k, v := range fields {
		obj[k] = v
	}
	return &unstructured.Unstructured{Object: obj}
}

func providerUnstructured(name, providerType string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.DexProviderGVK.GroupVersion().String(),
		"kind":       controller.DexProviderGVK.Kind,
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"type":   providerType,
			"ldap":   map[string]any{"bindPW": "secret-bind"},
			"github": map[string]any{"clientSecret": "secret-gh"},
		},
	}}
}

func userUnstructured(name, email string, groups []string, expireAt string) *unstructured.Unstructured {
	status := map[string]any{}
	if groups != nil {
		items := make([]any, 0, len(groups))
		for _, g := range groups {
			items = append(items, g)
		}
		status["groups"] = items
	}
	if expireAt != "" {
		status["expireAt"] = expireAt
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.UserGVK.GroupVersion().String(),
		"kind":       controller.UserGVK.Kind,
		"metadata":   map[string]any{"name": name, "uid": "uid-" + name},
		"spec": map[string]any{
			"email":    email,
			"password": "secret-user-password-hash",
			"userID":   name,
		},
		"status": status,
	}}
}
