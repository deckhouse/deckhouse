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
	"encoding/base64"
	"fmt"
	"strings"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const rawBcryptHash = "$2y$10$9fdmv4ewdvzVCTQ01BnAZ.Cy27fdnfNkl.dLIge2YS2gSF4czqXUy"

var (
	encodedBcryptHash  = base64.StdEncoding.EncodeToString([]byte(rawBcryptHash))
	passwordObjectName = naming.ToFnvLikeDex("admin@yourcompany.com")
)

func TestReconcile_LockLocal(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		lockFor       string
		wantUntil     time.Time
		wantPermanent bool
	}{
		{name: "1h", lockFor: "1h", wantUntil: now.Add(time.Hour)},
		{name: "permanent", lockFor: "permanent", wantPermanent: true},
		{name: "7d expands to hours", lockFor: "7d", wantUntil: now.Add(7 * 24 * time.Hour)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pw := passwordObj("admin", nil)
			op := userOperationObj("user-operation-01", now, map[string]any{
				"type":          "Lock",
				"initiatorType": "Admin",
				"user":          "admin",
				"lock":          map[string]any{"for": tt.lockFor},
			})
			r := newTestReconciler(t, now, pw, op)
			if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}

			got := getUnstructured(t, r, controller.PasswordGVK, naming.DexNamespace, passwordObjectName)
			until, _, err := unstructured.NestedString(got.Object, "lockedUntil")
			if err != nil {
				t.Fatalf("lockedUntil: %v", err)
			}
			if tt.wantPermanent {
				parsed, parseErr := time.Parse(time.RFC3339, until)
				if parseErr != nil {
					t.Fatalf("parse lockedUntil: %v", parseErr)
				}
				if parsed.Year() < 9000 {
					t.Errorf("permanent lockedUntil year = %d, want >= 9000", parsed.Year())
				}
			} else if until != tt.wantUntil.UTC().Format(time.RFC3339) {
				t.Errorf("lockedUntil = %q, want %q", until, tt.wantUntil.UTC().Format(time.RFC3339))
			}
			annots := got.GetAnnotations()
			if annots[lockedByAdministratorAnnot] != "true" {
				t.Errorf("locked-by-administrator = %q, want true", annots[lockedByAdministratorAnnot])
			}
			assertPhase(t, r, "user-operation-01", "Succeeded", now)
		})
	}
}

func TestReconcile_LockLocalEndsSessionsKeepsSecondFactor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	localSession := naming.OfflineTokenName("admin", naming.LocalConnectorID)
	r := newTestReconciler(t, now,
		passwordObj("admin", nil),
		sessionObj(localSession, map[string]any{"userID": "admin", "connID": naming.LocalConnectorID, "totp": totpSecret, "totpConfirmed": true}),
		identityWithTOTP(t, "admin", naming.LocalConnectorID),
		refreshTokenObj("rt-1", naming.LocalConnectorID, "admin"),
		refreshTokenObj("rt-2", naming.LocalConnectorID, "admin"),
		userOperationObj("user-operation-01", now, map[string]any{
			"type":          "Lock",
			"initiatorType": "Admin",
			"user":          "admin",
			"lock":          map[string]any{"for": "1h"},
		}),
	)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-1")
	assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-2")
	assertSecondFactorKept(t, r, "admin", localSession)
	assertPhase(t, r, "user-operation-01", "Succeeded", now)
}

// assertSecondFactorKept checks that the TOTP secret of the local user is still in both places.
func assertSecondFactorKept(t *testing.T, r *Reconciler, username, session string) {
	t.Helper()
	sess := getUnstructured(t, r, controller.OfflineSessionsGVK, naming.DexNamespace, session)
	if totp, _, _ := unstructured.NestedString(sess.Object, "totp"); totp != totpSecret {
		t.Errorf("OfflineSessions totp = %q, want the secret kept", totp)
	}
	totp, err := getIdentity(t, r, username, naming.LocalConnectorID).TOTP()
	if err != nil || totp == nil || totp.Secret != totpSecret || !totp.Confirmed {
		t.Errorf("UserIdentity totp = %+v (%v), want the confirmed secret kept", totp, err)
	}
}

func TestReconcile_LockExternal(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	sess := sessionObj("ext-sess", map[string]any{
		"userID":                         "jane",
		"connID":                         "my-ldap",
		"email":                          "Jane.Doe@example.org",
		"incorrectPasswordLoginAttempts": int64(4),
	})
	op := userOperationObj("user-operation-01", now, map[string]any{
		"type":          "Lock",
		"initiatorType": "Admin",
		"target": map[string]any{
			"connectorID": "my-ldap",
			"email":       "jane.doe@example.org",
		},
		"lock": map[string]any{"for": "1h"},
	})
	r := newTestReconciler(t, now, sess, op)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	got := getUnstructured(t, r, controller.OfflineSessionsGVK, naming.DexNamespace, "ext-sess")
	until, _, _ := unstructured.NestedString(got.Object, "lockedUntil")
	if until != now.Add(time.Hour).UTC().Format(time.RFC3339) {
		t.Errorf("lockedUntil = %q", until)
	}
	if got.GetAnnotations()[lockedByAdministratorAnnot] != "true" {
		t.Errorf("annotation = %q, want true", got.GetAnnotations()[lockedByAdministratorAnnot])
	}
	attempts, _, _ := unstructured.NestedInt64(got.Object, "incorrectPasswordLoginAttempts")
	if attempts != 0 {
		t.Errorf("attempts = %d, want 0", attempts)
	}
	assertPhase(t, r, "user-operation-01", "Succeeded", now)
}

func TestReconcile_UnlockLocal(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	pw := passwordObj("admin", map[string]any{
		"lockedUntil": "2077-07-12T00:00:00Z",
	})
	pw.SetAnnotations(map[string]string{lockedByAdministratorAnnot: ""})
	op := userOperationObj("user-operation-01", now, map[string]any{
		"type":          "Unlock",
		"initiatorType": "Admin",
		"user":          "admin",
	})
	r := newTestReconciler(t, now, pw, op)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	got := getUnstructured(t, r, controller.PasswordGVK, naming.DexNamespace, passwordObjectName)
	until, found, _ := unstructured.NestedFieldNoCopy(got.Object, "lockedUntil")
	if found && until != nil {
		t.Errorf("lockedUntil still set: %#v", until)
	}
	if _, ok := got.GetAnnotations()[lockedByAdministratorAnnot]; ok {
		t.Error("locked-by-administrator annotation still present")
	}
	assertPhase(t, r, "user-operation-01", "Succeeded", now)
}

func TestReconcile_UnlockExternal(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	sess := sessionObj("ext-sess", map[string]any{
		"userID":      "jane",
		"connID":      "my-ldap",
		"email":       "jane.doe@example.org",
		"lockedUntil": "2077-07-12T00:00:00Z",
	})
	sess.SetAnnotations(map[string]string{lockedByAdministratorAnnot: "true"})
	op := userOperationObj("user-operation-01", now, map[string]any{
		"type":          "Unlock",
		"initiatorType": "Admin",
		"target": map[string]any{
			"connectorID": "my-ldap",
			"email":       "jane.doe@example.org",
		},
	})
	r := newTestReconciler(t, now, sess, op)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	got := getUnstructured(t, r, controller.OfflineSessionsGVK, naming.DexNamespace, "ext-sess")
	until, found, _ := unstructured.NestedFieldNoCopy(got.Object, "lockedUntil")
	if found && until != nil {
		t.Errorf("lockedUntil still set: %#v", until)
	}
	if _, ok := got.GetAnnotations()[lockedByAdministratorAnnot]; ok {
		t.Error("locked-by-administrator annotation still present")
	}
	assertPhase(t, r, "user-operation-01", "Succeeded", now)
}

func TestReconcile_ResetPassword(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	rec := &logRecorder{}
	pw := passwordObj("admin", map[string]any{"hash": "old-hash"})
	op := userOperationObj("user-operation-01", now, map[string]any{
		"type":          "ResetPassword",
		"initiatorType": "Admin",
		"user":          "admin",
		"resetPassword": map[string]any{"newPasswordHash": rawBcryptHash},
	})
	localSession := naming.OfflineTokenName("admin", naming.LocalConnectorID)
	r := newTestReconcilerWithLog(t, now, logr.New(rec), pw, op,
		sessionObj(localSession, map[string]any{"userID": "admin", "connID": naming.LocalConnectorID, "totp": totpSecret, "totpConfirmed": true}),
		identityWithTOTP(t, "admin", naming.LocalConnectorID),
		refreshTokenObj("rt-1", naming.LocalConnectorID, "admin"),
	)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	got := getUnstructured(t, r, controller.PasswordGVK, naming.DexNamespace, passwordObjectName)
	hash, _, _ := unstructured.NestedString(got.Object, "hash")
	if hash != encodedBcryptHash {
		t.Errorf("password hash = %q, want encoded bcrypt", hash)
	}
	requireReset, _, _ := unstructured.NestedBool(got.Object, "requireResetHashOnNextSuccLogin")
	if !requireReset {
		t.Error("requireResetHashOnNextSuccLogin = false, want true")
	}
	assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-1")
	assertSecondFactorKept(t, r, "admin", localSession)
	assertPhase(t, r, "user-operation-01", "Succeeded", now)

	uo := getUnstructured(t, r, controller.UserOperationGVK, "", "user-operation-01")
	_, found, _ := unstructured.NestedMap(uo.Object, "spec", "resetPassword")
	if found {
		t.Error("spec.resetPassword still present after success")
	}
	joined := strings.Join(rec.msgs, " ")
	if strings.Contains(joined, rawBcryptHash) || strings.Contains(joined, encodedBcryptHash) {
		t.Errorf("logs leaked password hash: %q", joined)
	}
}

func TestReconcile_Reset2FADeletesSessions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	localSession := naming.OfflineTokenName("admin", naming.LocalConnectorID)
	ldapSession := naming.OfflineTokenName("admin", "corp-ldap")
	r := newTestReconciler(t, now,
		passwordObj("admin", nil),
		sessionObj(localSession, map[string]any{"userID": "admin", "connID": naming.LocalConnectorID}),
		sessionObj(ldapSession, map[string]any{"userID": "admin", "connID": "corp-ldap"}),
		refreshTokenObj("rt-local", naming.LocalConnectorID, "admin"),
		refreshTokenObj("rt-ldap", "corp-ldap", "admin"),
		userOperationObj("user-operation-01", now, map[string]any{
			"type":          "Reset2FA",
			"initiatorType": "Admin",
			"user":          "admin",
		}),
	)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertDeleted(t, r, controller.OfflineSessionsGVK, naming.DexNamespace, localSession)
	assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-local")
	// The LDAP user with the same name keeps their sessions and v2.44 data.
	getUnstructured(t, r, controller.OfflineSessionsGVK, naming.DexNamespace, ldapSession)
	getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-ldap")
	assertPhase(t, r, "user-operation-01", "Succeeded", now)
}

// A local user and a user of another provider can share a name, an ID, and a preferred username.
// Lock and ResetPassword of the local user end only the local user's sessions.
func TestReconcile_EndSessionsOfTheLocalUserOnly(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		spec map[string]any
	}{
		{
			name: "lock",
			spec: map[string]any{"type": "Lock", "initiatorType": "Admin", "user": "admin", "lock": map[string]any{"for": "1h"}},
		},
		{
			name: "reset password",
			spec: map[string]any{
				"type": "ResetPassword", "initiatorType": "Admin", "user": "admin",
				"resetPassword": map[string]any{"newPasswordHash": rawBcryptHash},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newTestReconciler(t, now,
				passwordObj("admin", nil),
				refreshTokenObj("rt-local", naming.LocalConnectorID, "admin"),
				refreshTokenObj("rt-ldap", "corp-ldap", "admin"),
				refreshTokenObj("rt-other-local", naming.LocalConnectorID, "jane"),
				userOperationObj("user-operation-01", now, tt.spec),
			)
			if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-local")
			getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-ldap")
			getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-other-local")
			assertPhase(t, r, "user-operation-01", "Succeeded", now)
		})
	}
}

// Reset2FA addresses the pair: it deletes the OfflineSessions named after (admin, local), also one
// written by an older Dex without userID, and no object of another name, whatever its tokens.
func TestReconcile_Reset2FADeletesTheOfflineSessionsOfThePair(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	localSession := naming.OfflineTokenName("admin", naming.LocalConnectorID)
	r := newTestReconciler(t, now,
		passwordObj("admin", nil),
		sessionObj(localSession, map[string]any{
			"connID": "local",
			"refresh": map[string]any{
				"console": map[string]any{"ID": "rt-1"},
			},
		}),
		sessionObj("offsess-no-userid", map[string]any{
			"connID": "local",
			"refresh": map[string]any{
				"console": map[string]any{"ID": "rt-2"},
			},
		}),
		refreshTokenObj("rt-1", naming.LocalConnectorID, "admin"),
		refreshTokenObj("rt-2", naming.LocalConnectorID, "admin"),
		refreshTokenObj("rt-ldap", "corp-ldap", "admin"),
		userOperationObj("user-operation-01", now, map[string]any{
			"type":          "Reset2FA",
			"initiatorType": "Admin",
			"user":          "admin",
		}),
	)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertDeleted(t, r, controller.OfflineSessionsGVK, naming.DexNamespace, localSession)
	assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-1")
	assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-2")
	getUnstructured(t, r, controller.OfflineSessionsGVK, naming.DexNamespace, "offsess-no-userid")
	getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-ldap")
	assertPhase(t, r, "user-operation-01", "Succeeded", now)
}

func TestReconcile_Reset2FAIdempotent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	r := newTestReconciler(t, now, userOperationObj("user-operation-01", now, map[string]any{
		"type":          "Reset2FA",
		"initiatorType": "Admin",
		"user":          "admin",
	}))
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertPhase(t, r, "user-operation-01", "Succeeded", now)
}

func TestReconcile_AlreadyTerminalIsNoOp(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	pw := passwordObj("admin", nil)
	op := userOperationObj("user-operation-01", now, map[string]any{
		"type":          "Lock",
		"initiatorType": "Admin",
		"user":          "admin",
		"lock":          map[string]any{"for": "1h"},
	})
	_ = unstructured.SetNestedMap(op.Object, map[string]any{
		"phase":       "Succeeded",
		"completedAt": now.UTC().Format(time.RFC3339),
	}, "status")

	r := newTestReconciler(t, now, pw, op)
	if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	got := getUnstructured(t, r, controller.PasswordGVK, naming.DexNamespace, passwordObjectName)
	_, found, _ := unstructured.NestedString(got.Object, "lockedUntil")
	if found {
		t.Error("already-terminal Lock must not patch Password")
	}
	assertPhase(t, r, "user-operation-01", "Succeeded", now)
}

func TestReconcile_FailCases(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		objs []client.Object
	}{
		{
			name: "lock without lock spec",
			objs: []client.Object{
				passwordObj("admin", nil),
				userOperationObj("user-operation-01", now, map[string]any{
					"type": "Lock", "initiatorType": "Admin", "user": "admin",
				}),
			},
		},
		{
			name: "lock without password",
			objs: []client.Object{
				userOperationObj("user-operation-01", now, map[string]any{
					"type": "Lock", "initiatorType": "Admin", "user": "admin",
					"lock": map[string]any{"for": "1h"},
				}),
			},
		},
		{
			name: "unlock without password",
			objs: []client.Object{
				userOperationObj("user-operation-01", now, map[string]any{
					"type": "Unlock", "initiatorType": "Admin", "user": "admin",
				}),
			},
		},
		{
			name: "reset password without spec",
			objs: []client.Object{
				passwordObj("admin", nil),
				userOperationObj("user-operation-01", now, map[string]any{
					"type": "ResetPassword", "initiatorType": "Admin", "user": "admin",
				}),
			},
		},
		{
			name: "reset password without password entity",
			objs: []client.Object{
				userOperationObj("user-operation-01", now, map[string]any{
					"type": "ResetPassword", "initiatorType": "Admin", "user": "admin",
					"resetPassword": map[string]any{"newPasswordHash": rawBcryptHash},
				}),
			},
		},
		{
			name: "reset2FA external target",
			objs: []client.Object{
				sessionObj("offsess-1", map[string]any{"userID": "admin"}),
				refreshTokenObj("rt-1", naming.LocalConnectorID, "admin"),
				userOperationObj("user-operation-01", now, map[string]any{
					"type":          "Reset2FA",
					"initiatorType": "Admin",
					"target":        map[string]any{"connectorID": "my-ldap", "email": "jane.doe@example.org"},
				}),
			},
		},
		{
			name: "decode error marks Failed",
			objs: []client.Object{
				brokenUserOperation("user-operation-01", now),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newTestReconciler(t, now, tt.objs...)
			if _, err := r.Reconcile(t.Context(), opRequest("user-operation-01")); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			uo := getUnstructured(t, r, controller.UserOperationGVK, "", "user-operation-01")
			phase, _, _ := unstructured.NestedString(uo.Object, "status", "phase")
			if phase != "Failed" {
				t.Errorf("phase = %q, want Failed", phase)
			}
			msg, _, _ := unstructured.NestedString(uo.Object, "status", "message")
			if msg == "" {
				t.Error("status.message is empty")
			}
			completed, _, _ := unstructured.NestedString(uo.Object, "status", "completedAt")
			if completed != now.UTC().Format(time.RFC3339) {
				t.Errorf("completedAt = %q", completed)
			}
			if tt.name == "reset2FA external target" {
				getUnstructured(t, r, controller.OfflineSessionsGVK, naming.DexNamespace, "offsess-1")
				getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-1")
			}
		})
	}
}

func TestReconcile_CleansUpOldOperations(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	old := now.Add(-25 * time.Hour)
	op := userOperationObj("old-user-operation-1", old, map[string]any{
		"type":          "Lock",
		"initiatorType": "Admin",
		"user":          "admin",
		"lock":          map[string]any{"for": "1h"},
	})
	_ = unstructured.SetNestedMap(op.Object, map[string]any{
		"phase":       "Succeeded",
		"completedAt": old.UTC().Format(time.RFC3339),
	}, "status")

	r := newTestReconciler(t, now, op)
	if _, err := r.Reconcile(t.Context(), opRequest("old-user-operation-1")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertDeleted(t, r, controller.UserOperationGVK, "", "old-user-operation-1")
}

func TestReconcile_HonorsCanceledContext(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, time.Now())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := r.Reconcile(ctx, opRequest("x"))
	if err == nil {
		t.Fatal("Reconcile with canceled context: want error")
	}
}

func newTestReconciler(t *testing.T, now time.Time, objs ...client.Object) *Reconciler {
	t.Helper()
	return newTestReconcilerWithLog(t, now, logr.Discard(), objs...)
}

func newTestReconcilerWithLog(t *testing.T, now time.Time, log logr.Logger, objs ...client.Object) *Reconciler {
	t.Helper()
	c := newFakeClientBuilder(objs...).Build()
	return New(c, c, log, func() time.Time { return now })
}

// newFakeClientBuilder returns a fake client builder with the objects, the status subresource of
// UserOperation, and every index of the controller.
func newFakeClientBuilder(objs ...client.Object) *fake.ClientBuilder {
	builder := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithRESTMapper(newTestMapper()).
		WithStatusSubresource(controller.Object(controller.UserOperationGVK)).
		WithObjects(objs...)
	for _, index := range controller.Indexes() {
		builder = builder.WithIndex(index.Object, index.Field, index.Extract)
	}
	return builder
}

func newTestMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "dex.coreos.com", Version: "v1"},
		{Group: "deckhouse.io", Version: "v1"},
	})
	m.Add(controller.PasswordGVK, meta.RESTScopeNamespace)
	m.Add(controller.OfflineSessionsGVK, meta.RESTScopeNamespace)
	m.Add(controller.RefreshTokenGVK, meta.RESTScopeNamespace)
	m.Add(controller.UserIdentityGVK, meta.RESTScopeNamespace)
	m.Add(controller.AuthSessionGVK, meta.RESTScopeNamespace)
	m.Add(controller.RevocationGVK, meta.RESTScopeNamespace)
	m.Add(controller.AuthCodeGVK, meta.RESTScopeNamespace)
	m.Add(controller.AuthRequestGVK, meta.RESTScopeNamespace)
	m.Add(controller.UserOperationGVK, meta.RESTScopeRoot)
	return m
}

func opRequest(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func passwordObj(username string, extra map[string]any) *unstructured.Unstructured {
	email := username + "@yourcompany.com"
	obj := map[string]any{
		"apiVersion": controller.PasswordGVK.GroupVersion().String(),
		"kind":       controller.PasswordGVK.Kind,
		"metadata": map[string]any{
			"name":      naming.ToFnvLikeDex(email),
			"namespace": naming.DexNamespace,
		},
		"email":    email,
		"username": username,
		"userID":   username,
		"hash":     "old",
	}
	for k, v := range extra {
		obj[k] = v
	}
	return &unstructured.Unstructured{Object: obj}
}

func sessionObj(name string, fields map[string]any) *unstructured.Unstructured {
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

// refreshTokenObj is a refresh token Dex issued through connID to the user userID, whose
// username and preferred username are userID too.
func refreshTokenObj(name, connID, userID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.RefreshTokenGVK.GroupVersion().String(),
		"kind":       controller.RefreshTokenGVK.Kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": naming.DexNamespace,
		},
		"connectorID": connID,
		"claims": map[string]any{
			"email":             userID + "@yourcompany.com",
			"username":          userID,
			"preferredUsername": userID,
			"userID":            userID,
		},
	}}
}

func userOperationObj(name string, created time.Time, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.UserOperationGVK.GroupVersion().String(),
		"kind":       controller.UserOperationGVK.Kind,
		"metadata": map[string]any{
			"name":              name,
			"uid":               "uid-" + name,
			"creationTimestamp": created.UTC().Format(time.RFC3339),
		},
		"spec": spec,
	}}
}

func brokenUserOperation(name string, created time.Time) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.UserOperationGVK.GroupVersion().String(),
		"kind":       controller.UserOperationGVK.Kind,
		"metadata": map[string]any{
			"name":              name,
			"creationTimestamp": created.UTC().Format(time.RFC3339),
		},
		"spec": "this should be an object",
	}}
}

func getUnstructured(t *testing.T, r *Reconciler, gvk schema.GroupVersionKind, namespace, name string) *unstructured.Unstructured {
	t.Helper()
	obj := controller.Object(gvk)
	if err := r.client.Get(t.Context(), types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
		t.Fatalf("get %s %s/%s: %v", gvk.Kind, namespace, name, err)
	}
	return obj
}

func assertDeleted(t *testing.T, r *Reconciler, gvk schema.GroupVersionKind, namespace, name string) {
	t.Helper()
	obj := controller.Object(gvk)
	err := r.client.Get(t.Context(), types.NamespacedName{Namespace: namespace, Name: name}, obj)
	if err == nil {
		t.Fatalf("%s %s/%s still exists", gvk.Kind, namespace, name)
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("get %s %s/%s: %v", gvk.Kind, namespace, name, err)
	}
}

func assertPhase(t *testing.T, r *Reconciler, name, wantPhase string, now time.Time) {
	t.Helper()
	uo := getUnstructured(t, r, controller.UserOperationGVK, "", name)
	phase, _, _ := unstructured.NestedString(uo.Object, "status", "phase")
	if phase != wantPhase {
		t.Errorf("phase = %q, want %q", phase, wantPhase)
	}
	completed, _, _ := unstructured.NestedString(uo.Object, "status", "completedAt")
	if completed != now.UTC().Format(time.RFC3339) {
		t.Errorf("completedAt = %q, want %q", completed, now.UTC().Format(time.RFC3339))
	}
}

type logRecorder struct {
	msgs []string
}

func (l *logRecorder) Init(logr.RuntimeInfo) {}
func (l *logRecorder) Enabled(int) bool      { return true }
func (l *logRecorder) Info(_ int, msg string, kv ...any) {
	l.msgs = append(l.msgs, fmt.Sprint(msg, kv))
}
func (l *logRecorder) Error(err error, msg string, kv ...any) {
	l.msgs = append(l.msgs, fmt.Sprint(err, msg, kv))
}
func (l *logRecorder) WithValues(...any) logr.LogSink { return l }
func (l *logRecorder) WithName(string) logr.LogSink   { return l }

var _ logr.LogSink = (*logRecorder)(nil)
