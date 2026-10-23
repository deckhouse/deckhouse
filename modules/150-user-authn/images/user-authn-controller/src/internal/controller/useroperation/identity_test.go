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

package useroperation

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/controller/identitymigration"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

const totpSecret = "otpauth://totp/Dex:admin@yourcompany.com?algorithm=SHA1&digits=6&issuer=Dex&period=30&secret=AAAAAAAAAAAAAAAA"

var operationTime = time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

func TestReconcile_LockLocalWritesUserIdentity(t *testing.T) {
	t.Parallel()

	enrolled := identityWithTOTP(t, "admin", naming.LocalConnectorID)
	r := newTestReconciler(t, operationTime,
		passwordObj("admin", map[string]any{"groups": []any{"admins"}}),
		enrolled,
		userOperationObj("user-operation-01", operationTime, map[string]any{
			"type": "Lock", "initiatorType": "Admin", "user": "admin", "lock": map[string]any{"for": "1h"},
		}),
	)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	id := getIdentity(t, r, "admin", naming.LocalConnectorID)
	blocked, _ := id.BlockedUntil()
	if expected := operationTime.Add(time.Hour); !blocked.Equal(expected) {
		t.Errorf("blockedUntil = %v, want %v", blocked, expected)
	}
	if !id.LockedByAdministrator() {
		t.Error("the lock is not marked as the administrator's")
	}
	if totp, _ := id.TOTP(); totp == nil {
		t.Error("Lock removed the second factor from UserIdentity; only Reset2FA does")
	}
	rec, _, _ := id.Record()
	if rec.LockFrom != useridentity.FormatTime(operationTime.Add(time.Hour)) {
		t.Errorf("record lockFrom = %q: the copy would take the old-place lock again", rec.LockFrom)
	}
	assertPhase(t, r, "user-operation-01", "Succeeded", operationTime)
}

func TestReconcile_LockLocalCreatesUserIdentity(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, operationTime,
		passwordObj("admin", map[string]any{"groups": []any{"admins"}}),
		userOperationObj("user-operation-01", operationTime, map[string]any{
			"type": "Lock", "initiatorType": "Admin", "user": "admin", "lock": map[string]any{"for": "permanent"},
		}),
	)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	id := getIdentity(t, r, "admin", naming.LocalConnectorID)
	blocked, _ := id.BlockedUntil()
	if !blocked.Equal(foreverTime) {
		t.Errorf("blockedUntil = %v, want %v", blocked, foreverTime)
	}
	claims, _ := id.Claims()
	expected := useridentity.Claims{UserID: "admin", Username: "admin", Email: "admin@yourcompany.com", EmailVerified: true, Groups: []string{"admins"}}
	if !claims.Equal(expected) {
		t.Errorf("claims = %+v, want %+v", claims, expected)
	}
	rec, ok, _ := id.Record()
	if !ok || rec.Source != useridentity.SourceUserOperation {
		t.Errorf("record = %+v, %v", rec, ok)
	}
}

func TestReconcile_LockExternalWritesUserIdentity(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, operationTime,
		sessionObj(naming.OfflineTokenName("jane", "my-ldap"), map[string]any{
			"userID": "jane", "connID": "my-ldap", "email": "jane.doe@example.org",
		}),
		userOperationObj("user-operation-01", operationTime, map[string]any{
			"type": "Lock", "initiatorType": "Admin",
			"target": map[string]any{"connectorID": "my-ldap", "email": "jane.doe@example.org"},
			"lock":   map[string]any{"for": "1h"},
		}),
	)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	id := getIdentity(t, r, "jane", "my-ldap")
	blocked, _ := id.BlockedUntil()
	if !blocked.Equal(operationTime.Add(time.Hour)) || !id.LockedByAdministrator() {
		t.Errorf("blockedUntil = %v, admin = %v", blocked, id.LockedByAdministrator())
	}
	claims, _ := id.Claims()
	if claims.Email != "jane.doe@example.org" || claims.UserID != "jane" {
		t.Errorf("claims = %+v", claims)
	}
}

func TestReconcile_ExternalTargetInUserIdentity(t *testing.T) {
	t.Parallel()

	oidcIdentity := func(userID, email string) *unstructured.Unstructured {
		return identityObj(userID, "corp-oidc", func(id *useridentity.Identity) {
			id.SetClaims(useridentity.Claims{UserID: userID, Email: email})
		})
	}
	lockOp := userOperationObj("user-operation-01", operationTime, map[string]any{
		"type": "Lock", "initiatorType": "Admin",
		"target": map[string]any{"connectorID": "corp-oidc", "email": "bob@example.com"},
		"lock":   map[string]any{"for": "1h"},
	})

	t.Run("an identity without OfflineSessions is locked", func(t *testing.T) {
		t.Parallel()
		r := newTestReconciler(t, operationTime, oidcIdentity("sub-1", "Bob@Example.com"), lockOp.DeepCopy())
		if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		blocked, _ := getIdentity(t, r, "sub-1", "corp-oidc").BlockedUntil()
		if !blocked.Equal(operationTime.Add(time.Hour)) {
			t.Errorf("blockedUntil = %v, want %v", blocked, operationTime.Add(time.Hour))
		}
		assertPhase(t, r, "user-operation-01", "Succeeded", operationTime)
	})

	t.Run("the OfflineSessions of Dex v2.46 has no email and is locked too", func(t *testing.T) {
		t.Parallel()
		session := naming.OfflineTokenName("sub-1", "corp-oidc")
		r := newTestReconciler(t, operationTime,
			oidcIdentity("sub-1", "bob@example.com"),
			sessionObj(session, map[string]any{"userID": "sub-1", "connID": "corp-oidc"}),
			lockOp.DeepCopy(),
		)
		if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		if !oldPlaceLocked(t, t.Context(), r.client, "corp-oidc", sessionObj(session, nil)) {
			t.Error("OfflineSessions lockedUntil not set")
		}
		assertPhase(t, r, "user-operation-01", "Succeeded", operationTime)
	})

	t.Run("two accounts with the email fail the operation", func(t *testing.T) {
		t.Parallel()
		r := newTestReconciler(t, operationTime,
			oidcIdentity("sub-1", "bob@example.com"),
			oidcIdentity("sub-2", "bob@example.com"),
			lockOp.DeepCopy(),
		)
		if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		assertPhase(t, r, "user-operation-01", "Failed", operationTime)
		for _, userID := range []string{"sub-1", "sub-2"} {
			if blocked, _ := getIdentity(t, r, userID, "corp-oidc").BlockedUntil(); !blocked.IsZero() {
				t.Errorf("%s blockedUntil = %v, want no lock", userID, blocked)
			}
		}
	})

	t.Run("an identity of another connector is not the target", func(t *testing.T) {
		t.Parallel()
		other := identityObj("sub-1", "other-oidc", func(id *useridentity.Identity) {
			id.SetClaims(useridentity.Claims{UserID: "sub-1", Email: "bob@example.com"})
		})
		r := newTestReconciler(t, operationTime, other, lockOp.DeepCopy())
		if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		assertPhase(t, r, "user-operation-01", "Failed", operationTime)
	})
}

func TestReconcile_UnlockClearsBothPlaces(t *testing.T) {
	t.Parallel()

	lock := time.Date(2077, 7, 12, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		userID  string
		connID  string
		objects []client.Object
		spec    map[string]any
	}{
		{
			name:   "local",
			userID: "admin",
			connID: naming.LocalConnectorID,
			objects: []client.Object{
				passwordObj("admin", map[string]any{"lockedUntil": lock.Format(time.RFC3339)}),
			},
			spec: map[string]any{"type": "Unlock", "initiatorType": "Admin", "user": "admin"},
		},
		{
			name:   "external",
			userID: "jane",
			connID: "my-ldap",
			objects: []client.Object{
				sessionObj(naming.OfflineTokenName("jane", "my-ldap"), map[string]any{
					"userID": "jane", "connID": "my-ldap", "email": "jane.doe@example.org", "lockedUntil": lock.Format(time.RFC3339),
				}),
			},
			spec: map[string]any{
				"type": "Unlock", "initiatorType": "Admin",
				"target": map[string]any{"connectorID": "my-ldap", "email": "jane.doe@example.org"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			locked := identityObj(tt.userID, tt.connID, func(id *useridentity.Identity) {
				_ = id.SetBlockedUntil(lock)
				id.SetAdminLock(lock)
			})
			objs := append([]client.Object{locked, userOperationObj("user-operation-01", operationTime, tt.spec)}, tt.objects...)

			// The old place must already be clear when UserIdentity is written.
			var oldPlaceClearFirst bool
			r := newTestReconcilerWithFuncs(t, operationTime, interceptor.Funcs{
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					if obj.GetObjectKind().GroupVersionKind().Kind == controller.UserIdentityGVK.Kind {
						oldPlaceClearFirst = !oldPlaceLocked(t, ctx, c, tt.connID, tt.objects[0])
					}
					return c.Update(ctx, obj, opts...)
				},
			}, objs...)
			if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}

			id := getIdentity(t, r, tt.userID, tt.connID)
			if blocked, _ := id.BlockedUntil(); !blocked.IsZero() {
				t.Errorf("blockedUntil = %v, want none", blocked)
			}
			if _, ok := id.AdminLock(); ok {
				t.Error("admin-lock annotation is still set")
			}
			if rec, _, _ := id.Record(); rec.ClearedAt == "" {
				t.Error("record clearedAt is not set")
			}
			if !oldPlaceClearFirst {
				t.Error("UserIdentity was written before the old place was cleared")
			}
			assertPhase(t, r, "user-operation-01", "Succeeded", operationTime)
		})
	}
}

func TestReconcile_Reset2FAClearsUserIdentity(t *testing.T) {
	t.Parallel()

	enrolled := identityWithTOTP(t, "admin", naming.LocalConnectorID)
	enrolled.Object["webauthnCredentials"] = map[string]any{"key": []any{map[string]any{"displayName": "yubikey"}}}
	sessionName := naming.OfflineTokenName("admin", naming.LocalConnectorID)

	var sessionGoneFirst bool
	r := newTestReconcilerWithFuncs(t, operationTime, interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if obj.GetObjectKind().GroupVersionKind().Kind == controller.UserIdentityGVK.Kind {
				err := c.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: sessionName}, controller.Object(controller.OfflineSessionsGVK))
				sessionGoneFirst = apierrors.IsNotFound(err)
			}
			return c.Update(ctx, obj, opts...)
		},
	},
		passwordObj("admin", nil),
		sessionObj(sessionName, map[string]any{"userID": "admin", "connID": naming.LocalConnectorID, "totp": totpSecret, "totpConfirmed": true}),
		enrolled,
		userOperationObj("user-operation-01", operationTime, map[string]any{
			"type": "Reset2FA", "initiatorType": "Admin", "user": "admin",
		}),
	)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	id := getIdentity(t, r, "admin", naming.LocalConnectorID)
	for _, field := range []string{"mfaSecrets", "webauthnCredentials"} {
		if _, ok := id.Object().Object[field]; ok {
			t.Errorf("%s is still set", field)
		}
	}
	if claims, _ := id.Claims(); claims.Email == "" {
		t.Error("Reset2FA removed the claims")
	}
	assertDeleted(t, r, controller.OfflineSessionsGVK, naming.DexNamespace, sessionName)
	if !sessionGoneFirst {
		t.Error("UserIdentity was written before the old place was cleared")
	}
	assertPhase(t, r, "user-operation-01", "Succeeded", operationTime)
}

func TestReconcile_Reset2FACreatesUserIdentityWhenMissing(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, operationTime,
		passwordObj("admin", nil),
		userOperationObj("user-operation-01", operationTime, map[string]any{
			"type": "Reset2FA", "initiatorType": "Admin", "user": "admin",
		}),
	)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// A copy that read the old place before the reset must find the object changed and retry.
	rec, ok, _ := getIdentity(t, r, "admin", naming.LocalConnectorID).Record()
	if !ok || rec.ClearedAt != useridentity.FormatTime(operationTime) {
		t.Errorf("record = %+v, %v; want clearedAt set", rec, ok)
	}
}

func TestReconcile_Reset2FAWithoutPasswordWritesNoUserIdentity(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, operationTime, userOperationObj("user-operation-01", operationTime, map[string]any{
		"type": "Reset2FA", "initiatorType": "Admin", "user": "admin",
	}))
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	assertDeleted(t, r, controller.UserIdentityGVK, naming.DexNamespace, naming.OfflineTokenName("admin", naming.LocalConnectorID))
	assertPhase(t, r, "user-operation-01", "Succeeded", operationTime)
}

// The copy runs next to the operations. After Reset2FA it must not bring the secret back, and
// after Lock it must not take the old-place lock again.
func TestUserOperation_CopyDoesNotUndoTheOperation(t *testing.T) {
	t.Parallel()

	sessionName := naming.OfflineTokenName("admin", naming.LocalConnectorID)
	c := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithRESTMapper(newTestMapper()).
		WithStatusSubresource(controller.Object(controller.UserOperationGVK)).
		WithIndex(controller.Object(controller.PasswordGVK), controller.PasswordPairField, controller.PasswordPair).
		WithObjects(
			passwordObj("admin", nil),
			sessionObj(sessionName, map[string]any{
				"userID": "admin", "connID": naming.LocalConnectorID, "email": "admin@yourcompany.com",
				"totp": totpSecret, "totpConfirmed": true,
				"metadata": map[string]any{"name": sessionName, "namespace": naming.DexNamespace, "uid": "session-1"},
			}),
			userOperationObj("reset", operationTime, map[string]any{"type": "Reset2FA", "initiatorType": "Admin", "user": "admin"}),
		).
		Build()
	ops := New(c, c, logr.Discard(), func() time.Time { return operationTime })
	migration := identitymigration.New(c, c, logr.Discard(), func() time.Time { return operationTime })
	pair := reconcile.Request{NamespacedName: types.NamespacedName{Name: sessionName}}

	if _, err := migration.Reconcile(t.Context(), pair); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if totp, _ := getIdentityFrom(t, c, "admin", naming.LocalConnectorID).TOTP(); totp == nil {
		t.Fatal("the copy did not take the secret")
	}

	if _, err := ops.Reconcile(t.Context(), opRequest("reset")); err != nil {
		t.Fatalf("Reset2FA: %v", err)
	}
	if _, err := migration.Reconcile(t.Context(), pair); err != nil {
		t.Fatalf("copy after Reset2FA: %v", err)
	}
	if totp, _ := getIdentityFrom(t, c, "admin", naming.LocalConnectorID).TOTP(); totp != nil {
		t.Errorf("totp = %+v: the copy brought back the secret Reset2FA removed", totp)
	}
}

// An operation can run before the copy reaches the pair, right after the upgrade, or after it.
// Its write marks the pair as copied, and Dex stops reading the old places of a marked pair, so the
// UserIdentity must hold the TOTP secret and the old-place lock as soon as the operation is done,
// and end up the same in either order.
func TestUserOperation_CopyAndOperationInEitherOrder(t *testing.T) {
	t.Parallel()

	sessionName := naming.OfflineTokenName("admin", naming.LocalConnectorID)
	oldLock := operationTime.Add(48 * time.Hour)
	tests := []struct {
		name        string
		oldLock     bool // Password.lockedUntil of the old Dex, an automatic lock
		spec        map[string]any
		wantTOTP    bool
		wantBlocked time.Time
		wantAdmin   bool
	}{
		{
			name:     "unlock without a lock keeps the second factor",
			spec:     map[string]any{"type": "Unlock", "initiatorType": "Admin", "user": "admin"},
			wantTOTP: true,
		},
		{
			name:     "unlock lifts the old lock and keeps the second factor",
			oldLock:  true,
			spec:     map[string]any{"type": "Unlock", "initiatorType": "Admin", "user": "admin"},
			wantTOTP: true,
		},
		{
			name:        "reset2fa keeps the old lock",
			oldLock:     true,
			spec:        map[string]any{"type": "Reset2FA", "initiatorType": "Admin", "user": "admin"},
			wantBlocked: oldLock,
		},
		{
			name:        "lock keeps the second factor",
			spec:        map[string]any{"type": "Lock", "initiatorType": "Admin", "user": "admin", "lock": map[string]any{"for": "1h"}},
			wantTOTP:    true,
			wantBlocked: operationTime.Add(time.Hour),
			wantAdmin:   true,
		},
	}

	for _, tt := range tests {
		for _, operationFirst := range []bool{true, false} {
			order := "copy first"
			if operationFirst {
				order = "operation first"
			}
			t.Run(tt.name+", "+order, func(t *testing.T) {
				t.Parallel()

				password := passwordObj("admin", nil)
				if tt.oldLock {
					password = passwordObj("admin", map[string]any{"lockedUntil": oldLock.Format(time.RFC3339)})
				}
				c := fake.NewClientBuilder().
					WithScheme(runtime.NewScheme()).
					WithRESTMapper(newTestMapper()).
					WithStatusSubresource(controller.Object(controller.UserOperationGVK)).
					WithIndex(controller.Object(controller.PasswordGVK), controller.PasswordPairField, controller.PasswordPair).
					WithObjects(
						password,
						sessionObj(sessionName, map[string]any{
							"userID": "admin", "connID": naming.LocalConnectorID, "email": "admin@yourcompany.com",
							"totp": totpSecret, "totpConfirmed": true,
							"metadata": map[string]any{"name": sessionName, "namespace": naming.DexNamespace, "uid": "session-1"},
						}),
						userOperationObj("op", operationTime, tt.spec),
					).
					Build()
				ops := New(c, c, logr.Discard(), func() time.Time { return operationTime })
				migration := identitymigration.New(c, c, logr.Discard(), func() time.Time { return operationTime })
				copyPair := func() {
					t.Helper()
					if _, err := migration.Reconcile(t.Context(), reconcile.Request{NamespacedName: types.NamespacedName{Name: sessionName}}); err != nil {
						t.Fatalf("copy: %v", err)
					}
				}
				check := func(when string) {
					t.Helper()
					id := getIdentityFrom(t, c, "admin", naming.LocalConnectorID)
					if !id.Migrated() {
						t.Fatalf("%s: the UserIdentity is not marked", when)
					}
					totp, err := id.TOTP()
					if err != nil {
						t.Fatalf("%s: totp: %v", when, err)
					}
					switch {
					case tt.wantTOTP && (totp == nil || totp.Secret != totpSecret || !totp.Confirmed):
						t.Errorf("%s: totp = %+v, want the confirmed secret of OfflineSessions", when, totp)
					case !tt.wantTOTP && totp != nil:
						t.Errorf("%s: totp = %+v, want none", when, totp)
					}
					blocked, err := id.BlockedUntil()
					if err != nil {
						t.Fatalf("%s: blockedUntil: %v", when, err)
					}
					if !blocked.Equal(tt.wantBlocked) {
						t.Errorf("%s: blockedUntil = %v, want %v", when, blocked, tt.wantBlocked)
					}
					if id.LockedByAdministrator() != tt.wantAdmin {
						t.Errorf("%s: locked by administrator = %v, want %v", when, id.LockedByAdministrator(), tt.wantAdmin)
					}
				}

				if !operationFirst {
					copyPair()
				}
				if _, err := ops.Reconcile(t.Context(), opRequest("op")); err != nil {
					t.Fatalf("operation: %v", err)
				}
				uo := controller.Object(controller.UserOperationGVK)
				if err := c.Get(t.Context(), types.NamespacedName{Name: "op"}, uo); err != nil {
					t.Fatalf("get operation: %v", err)
				}
				if phase, _, _ := unstructured.NestedString(uo.Object, "status", "phase"); phase != "Succeeded" {
					t.Fatalf("operation phase = %q", phase)
				}
				check("after the operation")
				copyPair()
				check("after the copy")
			})
		}
	}
}

func newTestReconcilerWithFuncs(t *testing.T, now time.Time, funcs interceptor.Funcs, objs ...client.Object) *Reconciler {
	t.Helper()
	c := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithRESTMapper(newTestMapper()).
		WithStatusSubresource(controller.Object(controller.UserOperationGVK)).
		WithIndex(controller.Object(controller.PasswordGVK), controller.PasswordPairField, controller.PasswordPair).
		WithObjects(objs...).
		WithInterceptorFuncs(funcs).
		Build()
	return New(c, c, logr.Discard(), func() time.Time { return now })
}

func identityObj(userID, connID string, build func(*useridentity.Identity)) *unstructured.Unstructured {
	id := useridentity.New(userID, connID)
	id.SetClaims(useridentity.Claims{UserID: userID, Email: userID + "@yourcompany.com"})
	build(id)
	return id.Object()
}

func identityWithTOTP(t *testing.T, userID, connID string) *unstructured.Unstructured {
	t.Helper()
	return identityObj(userID, connID, func(id *useridentity.Identity) {
		id.SetTOTP(useridentity.MFASecret{Secret: totpSecret, Confirmed: true, CreatedAt: operationTime.Add(-time.Hour)})
	})
}

func getIdentity(t *testing.T, r *Reconciler, userID, connID string) *useridentity.Identity {
	t.Helper()
	return useridentity.FromUnstructured(getUnstructured(t, r, controller.UserIdentityGVK, naming.DexNamespace, useridentity.Name(userID, connID)))
}

func getIdentityFrom(t *testing.T, c client.Client, userID, connID string) *useridentity.Identity {
	t.Helper()
	obj := controller.Object(controller.UserIdentityGVK)
	key := types.NamespacedName{Namespace: naming.DexNamespace, Name: useridentity.Name(userID, connID)}
	if err := c.Get(t.Context(), key, obj); err != nil {
		t.Fatalf("get useridentity: %v", err)
	}
	return useridentity.FromUnstructured(obj)
}

func oldPlaceLocked(t *testing.T, ctx context.Context, c client.Client, connID string, old client.Object) bool {
	t.Helper()
	gvk := controller.PasswordGVK
	if connID != naming.LocalConnectorID {
		gvk = controller.OfflineSessionsGVK
	}
	obj := controller.Object(gvk)
	if err := c.Get(ctx, types.NamespacedName{Namespace: old.GetNamespace(), Name: old.GetName()}, obj); err != nil {
		t.Errorf("get old place: %v", err)
		return true
	}
	until, found, _ := unstructured.NestedFieldNoCopy(obj.Object, "lockedUntil")
	return found && until != nil
}
