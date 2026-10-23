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
	"slices"
	"sync/atomic"
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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
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

func TestReconcile_ExternalForEveryProviderType(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		providerType string
		wantCreate   bool
	}{
		{name: "LDAP provider", providerType: "LDAP", wantCreate: true},
		{name: "Crowd provider", providerType: "Crowd", wantCreate: true},
		{name: "OIDC provider", providerType: "OIDC", wantCreate: true},
		{name: "Github provider", providerType: "Github", wantCreate: true},
		{name: "SAML provider", providerType: "SAML", wantCreate: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			connID := "conn-" + tt.providerType
			sess := sessionUnstructured(naming.OfflineTokenName("alice", connID), map[string]any{
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
	sess := sessionUnstructured(naming.OfflineTokenName("local-user", "local"), map[string]any{
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

func TestReconcile_ExternalDoesNotListPasswords(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	connID := "corp-ldap"
	sessName := naming.OfflineTokenName("alice", connID)
	sess := sessionUnstructured(sessName, map[string]any{
		"email":  "alice@example.com",
		"userID": "alice",
		"connID": connID,
	})
	provider := providerUnstructured(connID, "LDAP")

	var passwordLists atomic.Int32
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(newTestMapper()).
		WithStatusSubresource(&v1alpha1.UserAccount{}).
		WithIndex(controller.Object(controller.AuthSessionGVK), controller.AuthSessionPairField, controller.AuthSessionPair).
		WithObjects(sess, provider).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if u, ok := list.(*unstructured.UnstructuredList); ok && u.GetKind() == controller.PasswordGVK.Kind+"List" {
					passwordLists.Add(1)
				}
				return cl.List(ctx, list, opts...)
			},
		}).
		Build()

	r := New(c, logr.Discard(), func() time.Time { return now })
	name := naming.ExternalName(connID, "alice")
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if passwordLists.Load() != 0 {
		t.Fatalf("password lists = %d, want 0 for an external account name", passwordLists.Load())
	}
	var ua v1alpha1.UserAccount
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua); err != nil {
		t.Fatalf("expected useraccount: %v", err)
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
		WithIndex(controller.Object(controller.PasswordGVK), controller.PasswordPairField, controller.PasswordPair).
		WithIndex(controller.Object(controller.AuthSessionGVK), controller.AuthSessionPairField, controller.AuthSessionPair).
		WithIndex(controller.Object(controller.RefreshTokenGVK), controller.RefreshTokenPairField, controller.RefreshTokenPair).
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
	m.Add(controller.UserIdentityGVK, meta.RESTScopeNamespace)
	m.Add(controller.AuthSessionGVK, meta.RESTScopeNamespace)
	m.Add(controller.RefreshTokenGVK, meta.RESTScopeNamespace)
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

// refreshTokenUnstructured is a Dex RefreshToken of the pair (userID, connID) named id, with its
// secret.
func refreshTokenUnstructured(id, userID, connID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":  controller.RefreshTokenGVK.GroupVersion().String(),
		"kind":        controller.RefreshTokenGVK.Kind,
		"metadata":    map[string]any{"name": id, "namespace": naming.DexNamespace},
		"claims":      map[string]any{"userID": userID},
		"connectorID": connID,
		"token":       "secret-refresh-token",
	}}
}

// authSessionUnstructured is a Dex AuthSession of the pair (userID, connID) with its cookie secret,
// which the projection must never show.
func authSessionUnstructured(id, userID, connID string, fields map[string]any) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion":  controller.AuthSessionGVK.GroupVersion().String(),
		"kind":        controller.AuthSessionGVK.Kind,
		"metadata":    map[string]any{"name": id, "namespace": naming.DexNamespace},
		"userID":      userID,
		"connectorID": connID,
		"secret":      "secret-session-cookie",
		"nonce":       "secret-nonce",
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

func TestReconcile_ExternalFromUserIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	adminLock := now.Add(48 * time.Hour)
	autoLock := now.Add(2 * time.Hour)
	expired := now.Add(-time.Hour)

	lastLogin := now.Add(-30 * time.Minute)
	migrated := func(id *useridentity.Identity) {
		if err := id.SetRecord(useridentity.Record{Time: now.Format(time.RFC3339)}); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name            string
		session         map[string]any
		identity        func(*useridentity.Identity)
		expectEmail     string
		expectLocked    bool
		expectUntil     *time.Time
		expectByAdmin   bool
		expectAttempts  int64
		expectLastLogin *time.Time
		expectTwoFactor *v1alpha1.UserAccountTwoFactor
	}{
		{
			name: "identity only, as Dex v2.46 keeps an OIDC user",
			identity: func(id *useridentity.Identity) {
				id.SetClaims(useridentity.Claims{UserID: "sub-1", Email: "bob@example.com"})
				_ = id.SetBlockedUntil(adminLock)
				id.SetAdminLock(adminLock)
			},
			expectEmail: "bob@example.com", expectLocked: true, expectUntil: &adminLock, expectByAdmin: true,
			expectTwoFactor: &v1alpha1.UserAccountTwoFactor{},
		},
		{
			name: "dex zero blockedUntil is not a lock",
			identity: func(id *useridentity.Identity) {
				id.SetClaims(useridentity.Claims{UserID: "sub-1", Email: "bob@example.com"})
				id.Object().Object["blockedUntil"] = "0001-01-01T00:00:00Z"
			},
			expectEmail: "bob@example.com", expectLocked: false,
		},
		{
			name: "before the copy the later lock of the two places wins",
			session: map[string]any{
				"email": "bob@old.example.com", "lockedUntil": autoLock.Format(time.RFC3339), "incorrectPasswordLoginAttempts": int64(2),
			},
			identity: func(id *useridentity.Identity) {
				id.SetClaims(useridentity.Claims{UserID: "sub-1", Email: "bob@example.com"})
				_ = id.SetBlockedUntil(adminLock)
				id.SetAdminLock(adminLock)
			},
			expectEmail: "bob@example.com", expectLocked: true, expectUntil: &adminLock, expectByAdmin: true, expectAttempts: 2,
		},
		{
			name: "after the copy only UserIdentity counts",
			session: map[string]any{
				"email": "bob@example.com", "lockedUntil": autoLock.Format(time.RFC3339), "incorrectPasswordLoginAttempts": int64(4),
			},
			identity: func(id *useridentity.Identity) {
				id.SetClaims(useridentity.Claims{UserID: "sub-1", Email: "bob@example.com"})
				id.Object().Object["incorrectPasswordLoginAttempts"] = int64(1)
				_ = id.SetLastLogin(lastLogin)
				migrated(id)
			},
			expectEmail: "bob@example.com", expectLocked: false, expectAttempts: 1, expectLastLogin: &lastLogin,
		},
		{
			name:    "old-place lock before the copy",
			session: map[string]any{"email": "bob@example.com", "lockedUntil": autoLock.Format(time.RFC3339)},
			identity: func(id *useridentity.Identity) {
				id.SetClaims(useridentity.Claims{UserID: "sub-1", Email: "bob@example.com"})
				_ = id.SetBlockedUntil(expired)
				id.SetAdminLock(expired)
			},
			expectEmail: "bob@example.com", expectLocked: true, expectUntil: &autoLock, expectByAdmin: false,
		},
		{
			name:        "session only, before the copy",
			session:     map[string]any{"email": "bob@example.com"},
			expectEmail: "bob@example.com", expectLocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			connID := "corp-oidc"
			pair := naming.OfflineTokenName("sub-1", connID)
			objs := []client.Object{providerUnstructured(connID, "OIDC")}
			if tt.session != nil {
				fields := map[string]any{"userID": "sub-1", "connID": connID}
				for k, v := range tt.session {
					fields[k] = v
				}
				objs = append(objs, sessionUnstructured(pair, fields))
			}
			if tt.identity != nil {
				id := useridentity.New("sub-1", connID)
				tt.identity(id)
				objs = append(objs, id.Object())
			}
			r := newTestReconciler(t, now, objs...)
			name := naming.ExternalName(connID, "sub-1")

			if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}

			var ua v1alpha1.UserAccount
			if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua); err != nil {
				t.Fatalf("get useraccount: %v", err)
			}
			s := ua.Status
			if s.Email != tt.expectEmail || s.Locked != tt.expectLocked || s.LockedByAdministrator != tt.expectByAdmin {
				t.Errorf("email = %q, locked = %v, byAdmin = %v; want %q, %v, %v",
					s.Email, s.Locked, s.LockedByAdministrator, tt.expectEmail, tt.expectLocked, tt.expectByAdmin)
			}
			if s.IncorrectLoginAttempts != tt.expectAttempts {
				t.Errorf("attempts = %d, want %d", s.IncorrectLoginAttempts, tt.expectAttempts)
			}
			if tt.expectUntil != nil && (s.LockedUntil == nil || !s.LockedUntil.Equal(&metav1.Time{Time: *tt.expectUntil})) {
				t.Errorf("lockedUntil = %v, want %v", s.LockedUntil, tt.expectUntil)
			}
			if s.ProviderType != "OIDC" || s.Kind != v1alpha1.KindExternal {
				t.Errorf("providerType = %q, kind = %q", s.ProviderType, s.Kind)
			}
			if tt.expectLastLogin != nil && (s.LastLogin == nil || !s.LastLogin.Equal(&metav1.Time{Time: *tt.expectLastLogin})) {
				t.Errorf("lastLogin = %v, want %v", s.LastLogin, tt.expectLastLogin)
			}
			if tt.expectTwoFactor != nil && (s.TwoFactor == nil || *s.TwoFactor != *tt.expectTwoFactor) {
				t.Errorf("twoFactor = %+v, want %+v", s.TwoFactor, tt.expectTwoFactor)
			}
			if tt.identity == nil && s.TwoFactor != nil {
				t.Errorf("twoFactor = %+v, want none without a UserIdentity", s.TwoFactor)
			}
		})
	}
}

func TestReconcile_LocalLockFromUserIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	lock := now.Add(time.Hour)
	pw := passwordUnstructured("pw-jane", map[string]any{"email": "jane@example.com", "username": "jane", "userID": "jane"})
	id := useridentity.New("jane", naming.LocalConnectorID)
	if err := id.SetBlockedUntil(lock); err != nil {
		t.Fatal(err)
	}
	id.SetAdminLock(lock)
	r := newTestReconciler(t, now, pw, id.Object())
	name := naming.LocalName("jane@example.com")

	res, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var ua v1alpha1.UserAccount
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua); err != nil {
		t.Fatalf("get useraccount: %v", err)
	}
	if !ua.Status.Locked || !ua.Status.LockedByAdministrator {
		t.Errorf("locked = %v, byAdmin = %v; want the UserIdentity lock", ua.Status.Locked, ua.Status.LockedByAdministrator)
	}
	if res.RequeueAfter != time.Hour {
		t.Errorf("RequeueAfter = %v, want 1h", res.RequeueAfter)
	}
}

func TestReconcile_LocalFromMigratedUserIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	lastLogin := now.Add(-time.Hour)
	oldLock := now.Add(time.Hour)

	tests := []struct {
		name            string
		migrated        bool
		totpConfirmed   bool
		expectLocked    bool
		expectTwoFactor v1alpha1.UserAccountTwoFactor
	}{
		{
			name:            "before the copy the Password lock is in force",
			migrated:        false,
			expectLocked:    true,
			expectTwoFactor: v1alpha1.UserAccountTwoFactor{Enrolled: true},
		},
		{
			name:            "after the copy the Password lock is history",
			migrated:        true,
			totpConfirmed:   true,
			expectLocked:    false,
			expectTwoFactor: v1alpha1.UserAccountTwoFactor{Enrolled: true, Confirmed: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pw := passwordUnstructured("pw-jane", map[string]any{
				"email": "jane@example.com", "username": "jane", "userID": "jane",
				"lockedUntil": oldLock.Format(time.RFC3339), "incorrectPasswordLoginAttempts": int64(3),
			})
			id := useridentity.New("jane", naming.LocalConnectorID)
			if err := id.SetLastLogin(lastLogin); err != nil {
				t.Fatal(err)
			}
			// What the cache keeps of a TOTP secret: flags only.
			id.Object().Object["mfaSecrets"] = map[string]any{
				"totp": map[string]any{"authenticatorID": "totp", "type": "TOTP", "confirmed": tt.totpConfirmed},
			}
			if tt.migrated {
				if err := id.SetRecord(useridentity.Record{Time: now.Format(time.RFC3339)}); err != nil {
					t.Fatal(err)
				}
			}
			r := newTestReconciler(t, now, pw, id.Object())
			name := naming.LocalName("jane@example.com")

			if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}

			var ua v1alpha1.UserAccount
			if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua); err != nil {
				t.Fatalf("get useraccount: %v", err)
			}
			s := ua.Status
			if s.Locked != tt.expectLocked {
				t.Errorf("locked = %v, want %v", s.Locked, tt.expectLocked)
			}
			if s.IncorrectLoginAttempts != 3 {
				t.Errorf("attempts = %d, want 3: local users count on Password", s.IncorrectLoginAttempts)
			}
			if s.LastLogin == nil || !s.LastLogin.Equal(&metav1.Time{Time: lastLogin}) {
				t.Errorf("lastLogin = %v, want %v", s.LastLogin, lastLogin)
			}
			if s.TwoFactor == nil || *s.TwoFactor != tt.expectTwoFactor {
				t.Errorf("twoFactor = %+v, want %+v", s.TwoFactor, tt.expectTwoFactor)
			}
		})
	}
}

func TestReconcile_LocalWithoutUserIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	pw := passwordUnstructured("pw-jane", map[string]any{"email": "jane@example.com", "username": "jane", "userID": "jane"})
	r := newTestReconciler(t, now, pw)
	name := naming.LocalName("jane@example.com")

	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var ua v1alpha1.UserAccount
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua); err != nil {
		t.Fatalf("get useraccount: %v", err)
	}
	if ua.Status.LastLogin != nil || ua.Status.TwoFactor != nil {
		t.Errorf("lastLogin = %v, twoFactor = %+v; want none before the first login", ua.Status.LastLogin, ua.Status.TwoFactor)
	}
}

func TestReconcile_LocalListsDevices(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 16, 12, 30, 0, 0, time.UTC)
	ts := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339Nano) }
	pair := naming.OfflineTokenName("jane", naming.LocalConnectorID)
	pw := passwordUnstructured("pw-jane", map[string]any{"email": "jane@example.com", "username": "jane", "userID": "jane"})
	offline := sessionUnstructured(pair, map[string]any{
		"userID": "jane",
		"connID": naming.LocalConnectorID,
		"grants": map[string]any{
			"g-console": map[string]any{
				"ID": "g-console", "ClientID": "console", "SessionID": "sid-1",
				"CreatedAt": ts(-2 * time.Hour), "LastUsed": ts(-time.Hour),
			},
			"g-kube": map[string]any{
				"ID": "g-kube", "ClientID": "kubeconfig-generator", "IPAddress": "198.51.100.2", "UserAgent": "kubectl",
				"CreatedAt": ts(-30 * time.Minute), "LastUsed": "0001-01-01T00:00:00Z",
			},
			// A reference whose refresh token is gone: no device.
			"g-gone": map[string]any{
				"ID": "g-gone", "ClientID": "grafana", "CreatedAt": ts(-10 * time.Minute), "LastUsed": ts(-time.Minute),
			},
		},
		"refresh": map[string]any{
			"console":              map[string]any{"ID": "g-console", "ClientID": "console"},
			"kubeconfig-generator": map[string]any{"ID": "g-kube", "ClientID": "kubeconfig-generator"},
		},
	})
	tokenConsole := refreshTokenUnstructured("g-console", "jane", naming.LocalConnectorID)
	tokenKube := refreshTokenUnstructured("g-kube", "jane", naming.LocalConnectorID)
	live := authSessionUnstructured("sid-1", "jane", naming.LocalConnectorID, map[string]any{
		"clientStates": map[string]any{"console": map[string]any{}},
		"ipAddress":    "203.0.113.7",
		"userAgent":    "Firefox",
		"createdAt":    ts(-3 * time.Hour),
		"lastActivity": ts(-2 * time.Hour),
		"idleExpiry":   ts(time.Hour),
	})
	ended := authSessionUnstructured("sid-ended", "jane", naming.LocalConnectorID, map[string]any{
		"createdAt":  ts(-48 * time.Hour),
		"idleExpiry": ts(-time.Minute),
	})
	// A session of another pair whose name the index would also return on a hash collision.
	other := authSessionUnstructured("sid-other", "someone", naming.LocalConnectorID, nil)

	r := newTestReconciler(t, now, pw, offline, tokenConsole, tokenKube, live, ended, other)
	name := naming.LocalName("jane@example.com")
	if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	var ua v1alpha1.UserAccount
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, &ua); err != nil {
		t.Fatalf("get useraccount: %v", err)
	}
	hour := func(h int) *metav1.Time { return metaTimeOrNil(time.Date(2026, 10, 16, h, 0, 0, 0, time.UTC)) }
	want := []v1alpha1.UserAccountSession{
		{
			ID: "g-kube", Kind: v1alpha1.SessionKindGrant, Clients: []string{"kubeconfig-generator"},
			IPAddress: "198.51.100.2", UserAgent: "kubectl",
			SignedInAt: metaTimeOrNil(now.Add(-30 * time.Minute)), LastActivity: hour(12),
		},
		{
			ID: "sid-1", Kind: v1alpha1.SessionKindBrowser, Clients: []string{"console"},
			IPAddress: "203.0.113.7", UserAgent: "Firefox",
			SignedInAt: metaTimeOrNil(now.Add(-3 * time.Hour)), LastActivity: hour(11),
		},
	}
	if !slices.EqualFunc(ua.Status.Sessions, want, sessionEqual) {
		got, _ := json.MarshalIndent(ua.Status.Sessions, "", "  ")
		t.Errorf("sessions =\n%s", got)
	}
	if ua.Status.SessionsTruncated {
		t.Error("sessionsTruncated = true, want false")
	}
}

func TestMapUserIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	pw := passwordUnstructured("pw-jane", map[string]any{"email": "jane@example.com", "username": "jane", "userID": "jane"})
	r := newTestReconciler(t, now, pw)

	local := r.mapUserIdentity(t.Context(), useridentity.New("jane", naming.LocalConnectorID).Object())
	if len(local) != 1 || local[0].Name != naming.LocalName("jane@example.com") {
		t.Errorf("local = %v, want the account of the Password", local)
	}
	external := r.mapUserIdentity(t.Context(), useridentity.New("sub-1", "corp-oidc").Object())
	if len(external) != 1 || external[0].Name != naming.ExternalName("corp-oidc", "sub-1") {
		t.Errorf("external = %v", external)
	}
	if reqs := r.mapUserIdentity(t.Context(), useridentity.New("ghost", naming.LocalConnectorID).Object()); len(reqs) != 0 {
		t.Errorf("local identity without a Password = %v", reqs)
	}
}

func TestMapDevicesOfAPair(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	pw := passwordUnstructured("pw-jane", map[string]any{"email": "jane@example.com", "username": "jane", "userID": "jane"})
	r := newTestReconciler(t, now, pw)
	local := naming.LocalName("jane@example.com")

	tests := []struct {
		name     string
		mapFunc  func(context.Context, client.Object) []reconcile.Request
		obj      client.Object
		expected []string
	}{
		{
			name:     "browser session of a local user",
			mapFunc:  r.mapAuthSession,
			obj:      authSessionUnstructured("sid-1", "jane", naming.LocalConnectorID, nil),
			expected: []string{local},
		},
		{
			name:     "browser session of an external user",
			mapFunc:  r.mapAuthSession,
			obj:      authSessionUnstructured("sid-2", "sub-1", "corp-oidc", nil),
			expected: []string{naming.ExternalName("corp-oidc", "sub-1")},
		},
		{
			name:     "browser session without its pair",
			mapFunc:  r.mapAuthSession,
			obj:      authSessionUnstructured("sid-3", "", "", nil),
			expected: nil,
		},
		{
			name:    "grants of a local user",
			mapFunc: r.mapOfflineSessions,
			obj: sessionUnstructured(naming.OfflineTokenName("jane", naming.LocalConnectorID), map[string]any{
				"userID": "jane", "connID": naming.LocalConnectorID,
			}),
			expected: []string{local},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, req := range tt.mapFunc(t.Context(), tt.obj) {
				got = append(got, req.Name)
			}
			if !slices.Equal(got, tt.expected) {
				t.Errorf("requests = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMapDexProviderIncludesUserIdentities(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	r := newTestReconciler(t, now,
		useridentity.New("sub-1", "corp-oidc").Object(),
		useridentity.New("sub-2", "other").Object(),
	)

	reqs := r.mapDexProvider(t.Context(), providerUnstructured("corp-oidc", "OIDC"))
	if len(reqs) != 1 || reqs[0].Name != naming.ExternalName("corp-oidc", "sub-1") {
		t.Errorf("requests = %v", reqs)
	}
}
