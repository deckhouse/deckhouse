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
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

var (
	adminPair = naming.OfflineTokenName("admin", naming.LocalConnectorID)
	ldapPair  = naming.OfflineTokenName("admin", "corp-ldap")
)

// signedIn are the Dex objects of the local user admin, signed in to console through a browser
// session and to kubeconfig-generator without one, and of an LDAP user with the same user ID.
func signedIn() []client.Object {
	return []client.Object{
		passwordObj("admin", nil),
		sessionObj(adminPair, map[string]any{
			"userID": "admin", "connID": naming.LocalConnectorID,
			"refresh": map[string]any{
				"console":              map[string]any{"ID": "rt-console", "SessionID": "sid-admin"},
				"kubeconfig-generator": map[string]any{"ID": "rt-kubeconfig"},
			},
		}),
		refreshTokenFor("rt-console", naming.LocalConnectorID, "admin", "console"),
		refreshTokenFor("rt-kubeconfig", naming.LocalConnectorID, "admin", "kubeconfig-generator"),
		authSessionObj("sid-admin", naming.LocalConnectorID, "admin", "console"),

		identityObj("admin", "corp-ldap", func(id *useridentity.Identity) {
			id.SetClaims(useridentity.Claims{UserID: "admin", Email: "admin@corp.example.org"})
			id.SetGeneration(7)
		}),
		sessionObj(ldapPair, map[string]any{
			"userID": "admin", "connID": "corp-ldap", "email": "admin@corp.example.org",
			"refresh": map[string]any{"console": map[string]any{"ID": "rt-ldap", "SessionID": "sid-ldap"}},
		}),
		refreshTokenFor("rt-ldap", "corp-ldap", "admin", "console"),
		authSessionObj("sid-ldap", "corp-ldap", "admin", "console"),
	}
}

// refreshTokenFor is refreshTokenObj issued to clientID.
func refreshTokenFor(name, connID, userID, clientID string) *unstructured.Unstructured {
	obj := refreshTokenObj(name, connID, userID)
	obj.Object["clientID"] = clientID
	return obj
}

func authSessionObj(sid, connID, userID string, clients ...string) *unstructured.Unstructured {
	states := map[string]any{}
	for _, c := range clients {
		states[c] = map[string]any{"AuthenticatedAt": "2026-08-13T11:00:00Z"}
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":   controller.AuthSessionGVK.GroupVersion().String(),
		"kind":         controller.AuthSessionGVK.Kind,
		"metadata":     map[string]any{"name": sid, "namespace": naming.DexNamespace},
		"userID":       userID,
		"connectorID":  connID,
		"clientStates": states,
	}}
}

func TestReconcile_RevokingOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		spec   map[string]any
		reason string
	}{
		{
			name:   "lock",
			spec:   map[string]any{"type": "Lock", "initiatorType": "admin", "user": "admin", "lock": map[string]any{"for": "1h"}},
			reason: revocation.ReasonLock,
		},
		{
			name: "reset password",
			spec: map[string]any{
				"type": "ResetPassword", "initiatorType": "admin", "user": "admin",
				"resetPassword": map[string]any{"newPasswordHash": rawBcryptHash},
			},
			reason: revocation.ReasonResetPassword,
		},
		{
			name:   "reset 2fa",
			spec:   map[string]any{"type": "Reset2FA", "initiatorType": "admin", "user": "admin"},
			reason: revocation.ReasonReset2FA,
		},
		{
			name:   "logout",
			spec:   map[string]any{"type": "Logout", "initiatorType": "self", "user": "admin"},
			reason: revocation.ReasonLogout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newTestReconciler(t, operationTime, append(signedIn(), userOperationObj("op", operationTime, tt.spec))...)
			if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			assertPhase(t, r, "op", "Succeeded", operationTime)

			name := revocation.PairName(adminPair, 1)
			uo := getUnstructured(t, r, controller.UserOperationGVK, "", "op")
			status, _, _ := unstructured.NestedMap(uo.Object, "status", "revocation")
			expected := map[string]any{"name": name, "generation": int64(1), "refreshTokensDeleted": int64(2), "authSessionsDeleted": int64(1)}
			if fmt.Sprint(status) != fmt.Sprint(expected) {
				t.Errorf("status.revocation = %v, want %v", status, expected)
			}
			if msg, _, _ := unstructured.NestedString(uo.Object, "status", "message"); msg != "generation 1; 2 refresh tokens and 1 sessions deleted" {
				t.Errorf("status.message = %q", msg)
			}

			rev := getRevocationObj(t, r, name)
			if rev.Reason != tt.reason || rev.OperationUID != "uid-op" || !slices.Equal(rev.Clients, []string{"console", "kubeconfig-generator"}) {
				t.Errorf("revocation = %+v", rev)
			}
			if g, _ := getIdentity(t, r, "admin", naming.LocalConnectorID).Generation(); g != 1 {
				t.Errorf("generation = %d, want 1", g)
			}
			assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-console")
			assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-kubeconfig")
			assertDeleted(t, r, controller.AuthSessionGVK, naming.DexNamespace, "sid-admin")
			assertLDAPPairUntouched(t, r)
		})
	}
}

// A1: the LDAP user with the same user ID as the local one is another pair, and none of the
// operations of the local user touches it.
func assertLDAPPairUntouched(t *testing.T, r *Reconciler) {
	t.Helper()
	getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-ldap")
	getUnstructured(t, r, controller.AuthSessionGVK, naming.DexNamespace, "sid-ldap")
	sess := getUnstructured(t, r, controller.OfflineSessionsGVK, naming.DexNamespace, ldapPair)
	if refresh, _, _ := unstructured.NestedMap(sess.Object, "refresh"); len(refresh) != 1 {
		t.Errorf("LDAP references = %v", refresh)
	}
	ldap := getIdentity(t, r, "admin", "corp-ldap")
	if g, _ := ldap.Generation(); g != 7 {
		t.Errorf("LDAP generation = %d, want 7", g)
	}
	if blocked, _ := ldap.BlockedUntil(); !blocked.IsZero() {
		t.Errorf("LDAP user blocked until %v", blocked)
	}
	list := controller.List(controller.RevocationGVK)
	if err := r.client.List(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	for i := range list.Items {
		if list.Items[i].GetLabels()[revocation.PairLabel] == ldapPair {
			t.Errorf("revocation %s names the LDAP pair", list.Items[i].GetName())
		}
	}
}

func TestReconcile_LockExternalRevokes(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, operationTime, append(signedIn(), userOperationObj("op", operationTime, map[string]any{
		"type": "Lock", "initiatorType": "admin",
		"target": map[string]any{"connectorID": "corp-ldap", "email": "admin@corp.example.org"},
		"lock":   map[string]any{"for": "1h"},
	}))...)
	if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertPhase(t, r, "op", "Succeeded", operationTime)
	if g, _ := getIdentity(t, r, "admin", "corp-ldap").Generation(); g != 8 {
		t.Errorf("LDAP generation = %d, want 8", g)
	}
	rev := getRevocationObj(t, r, revocation.PairName(ldapPair, 8))
	if rev.ConnectorID != "corp-ldap" || rev.Reason != revocation.ReasonLock {
		t.Errorf("revocation = %+v", rev)
	}
	assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-ldap")
	assertDeleted(t, r, controller.AuthSessionGVK, naming.DexNamespace, "sid-ldap")
	// The local user with the same ID keeps everything.
	getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-console")
	getUnstructured(t, r, controller.AuthSessionGVK, naming.DexNamespace, "sid-admin")
}

// The steps of a local Lock: the lock in UserIdentity and Password (the dual write of MR1), then
// the revocation in its order (generation, Revocation, sessions, grants), then the status.
func TestReconcile_LockLocalOrder(t *testing.T) {
	t.Parallel()

	rec := &callLog{}
	funcs := interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			rec.add("create", obj)
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			rec.add("update", obj)
			return c.Update(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			rec.add("patch", obj)
			return c.Patch(ctx, obj, patch, opts...)
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			rec.add("delete", obj)
			return c.Delete(ctx, obj, opts...)
		},
		SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			rec.add("update "+sub, obj)
			return c.SubResource(sub).Update(ctx, obj, opts...)
		},
	}
	r := newTestReconcilerWithFuncs(t, operationTime, funcs, append(signedIn(),
		identityObj("admin", naming.LocalConnectorID, func(*useridentity.Identity) {}),
		userOperationObj("op", operationTime, map[string]any{
			"type": "Lock", "initiatorType": "admin", "user": "admin", "lock": map[string]any{"for": "1h"},
		}))...)
	if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	expected := []string{
		"update UserIdentity " + adminPair,
		"patch Password " + passwordObjectName,
		"update UserIdentity " + adminPair,
		"create Revocation " + revocation.PairName(adminPair, 1),
		"delete AuthSession sid-admin",
		"delete RefreshToken rt-console",
		"delete RefreshToken rt-kubeconfig",
		"patch OfflineSessions " + adminPair,
		"patch OfflineSessions " + adminPair,
		"update status UserOperation op",
	}
	if got := rec.list(); !slices.Equal(got, expected) {
		t.Errorf("calls:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}
}

// The status write fails after the revocation: the retry of the operation finds its Revocation by
// the UID and does not raise the generation again.
func TestReconcile_RetryKeepsTheGeneration(t *testing.T) {
	t.Parallel()

	var failed atomic.Bool
	r := newTestReconcilerWithFuncs(t, operationTime, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if !failed.Swap(true) {
				return apierrors.NewServiceUnavailable("etcd is away")
			}
			return c.SubResource(sub).Update(ctx, obj, opts...)
		},
	}, append(signedIn(), userOperationObj("op", operationTime, map[string]any{
		"type": "Lock", "initiatorType": "admin", "user": "admin", "lock": map[string]any{"for": "1h"},
	}))...)
	if _, err := r.Reconcile(t.Context(), opRequest("op")); err == nil {
		t.Fatal("Reconcile succeeded through a failed status write")
	}
	if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
		t.Fatalf("retry: %v", err)
	}
	assertPhase(t, r, "op", "Succeeded", operationTime)
	if g, _ := getIdentity(t, r, "admin", naming.LocalConnectorID).Generation(); g != 1 {
		t.Errorf("generation = %d after the retry, want 1", g)
	}
	list := controller.List(controller.RevocationGVK)
	if err := r.client.List(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 {
		t.Errorf("revocations = %d, want 1", len(list.Items))
	}
}

func TestReconcile_LogoutNarrowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		logout       map[string]any
		phase        string
		message      string
		revocations  []string
		deleted      []string
		kept         []string
		tokensStatus int64
	}{
		{
			name:         "session",
			logout:       map[string]any{"session": "sid-admin"},
			phase:        "Succeeded",
			message:      "1 refresh tokens and 1 sessions deleted",
			revocations:  []string{revocation.SessionName("sid-admin")},
			deleted:      []string{"rt-console"},
			kept:         []string{"rt-kubeconfig"},
			tokensStatus: 1,
		},
		{
			name:         "except session",
			logout:       map[string]any{"exceptSession": "sid-admin"},
			phase:        "Succeeded",
			message:      "1 refresh tokens and 0 sessions deleted",
			revocations:  []string{revocation.GrantName("rt-kubeconfig")},
			deleted:      []string{"rt-kubeconfig"},
			kept:         []string{"rt-console"},
			tokensStatus: 1,
		},
		{
			name:         "grant",
			logout:       map[string]any{"grant": "rt-kubeconfig"},
			phase:        "Succeeded",
			message:      "1 refresh tokens and 0 sessions deleted",
			revocations:  []string{revocation.GrantName("rt-kubeconfig")},
			deleted:      []string{"rt-kubeconfig"},
			kept:         []string{"rt-console"},
			tokensStatus: 1,
		},
		{
			name:         "except grant",
			logout:       map[string]any{"exceptGrant": "rt-kubeconfig"},
			phase:        "Succeeded",
			message:      "1 refresh tokens and 1 sessions deleted",
			revocations:  []string{revocation.SessionName("sid-admin")},
			deleted:      []string{"rt-console"},
			kept:         []string{"rt-kubeconfig"},
			tokensStatus: 1,
		},
		{
			name:    "missing grant",
			logout:  map[string]any{"grant": "rt-missing"},
			phase:   "Failed",
			message: "grant not found",
			kept:    []string{"rt-console", "rt-kubeconfig"},
		},
		{
			name:    "session of another user",
			logout:  map[string]any{"session": "sid-ldap"},
			phase:   "Failed",
			message: "session not found",
			kept:    []string{"rt-console", "rt-kubeconfig"},
		},
		{
			name:    "missing session",
			logout:  map[string]any{"exceptSession": "sid-missing"},
			phase:   "Failed",
			message: "session not found",
			kept:    []string{"rt-console", "rt-kubeconfig"},
		},
		{
			name:    "two narrowings",
			logout:  map[string]any{"session": "sid-admin", "exceptSession": "sid-admin"},
			phase:   "Failed",
			message: "logout accepts at most one of session, exceptSession, grant, and exceptGrant",
			kept:    []string{"rt-console", "rt-kubeconfig"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newTestReconciler(t, operationTime, append(signedIn(), userOperationObj("op", operationTime, map[string]any{
				"type": "Logout", "initiatorType": "admin", "user": "admin", "logout": tt.logout,
			}))...)
			if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			assertPhase(t, r, "op", tt.phase, operationTime)
			uo := getUnstructured(t, r, controller.UserOperationGVK, "", "op")
			if msg, _, _ := unstructured.NestedString(uo.Object, "status", "message"); msg != tt.message {
				t.Errorf("status.message = %q, want %q", msg, tt.message)
			}
			if tt.phase == "Succeeded" {
				deleted, _, _ := unstructured.NestedInt64(uo.Object, "status", "revocation", "refreshTokensDeleted")
				if deleted != tt.tokensStatus {
					t.Errorf("status.revocation.refreshTokensDeleted = %d, want %d", deleted, tt.tokensStatus)
				}
				if _, found, _ := unstructured.NestedFieldNoCopy(uo.Object, "status", "revocation", "generation"); found {
					t.Error("a narrowed logout reports a generation")
				}
			}

			var names []string
			list := controller.List(controller.RevocationGVK)
			if err := r.client.List(t.Context(), list); err != nil {
				t.Fatal(err)
			}
			for i := range list.Items {
				names = append(names, list.Items[i].GetName())
			}
			if !slices.Equal(names, tt.revocations) {
				t.Errorf("revocations = %v, want %v", names, tt.revocations)
			}
			for _, name := range tt.deleted {
				assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, name)
			}
			for _, name := range tt.kept {
				getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, name)
			}
			if g, _ := getIdentityOrNil(t, r, "admin", naming.LocalConnectorID); g != 0 {
				t.Errorf("generation = %d, want none: a narrowed logout keeps the generation", g)
			}
			assertLDAPPairUntouched(t, r)
		})
	}
}

func TestReconcile_LogoutTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  map[string]any
		phase   string
		message string
	}{
		{
			name:   "email only",
			target: map[string]any{"email": "Admin@Corp.Example.Org"},
			phase:  "Succeeded",
		},
		{
			name:   "connector and user id",
			target: map[string]any{"email": "admin@corp.example.org", "connectorID": "corp-ldap", "userID": "admin"},
			phase:  "Succeeded",
		},
		{
			name:    "another user id",
			target:  map[string]any{"email": "admin@corp.example.org", "connectorID": "corp-ldap", "userID": "jdoe"},
			phase:   "Failed",
			message: `no account found for connector "corp-ldap" and email "admin@corp.example.org" (the user has likely never signed in through this provider)`,
		},
		{
			name:    "unknown email",
			target:  map[string]any{"email": "nobody@example.org"},
			phase:   "Failed",
			message: `no account found for email "nobody@example.org" (the user has likely never signed in)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newTestReconciler(t, operationTime, append(signedIn(), userOperationObj("op", operationTime, map[string]any{
				"type": "Logout", "initiatorType": "self", "target": tt.target,
			}))...)
			if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			assertPhase(t, r, "op", tt.phase, operationTime)
			uo := getUnstructured(t, r, controller.UserOperationGVK, "", "op")
			if tt.phase == "Failed" {
				if msg, _, _ := unstructured.NestedString(uo.Object, "status", "message"); msg != tt.message {
					t.Errorf("status.message = %q, want %q", msg, tt.message)
				}
				return
			}
			if g, _ := getIdentity(t, r, "admin", "corp-ldap").Generation(); g != 8 {
				t.Errorf("LDAP generation = %d, want 8", g)
			}
			assertDeleted(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-ldap")
			getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-console")
		})
	}
}

// R16: without connectorID the email may match accounts of several providers; that is ambiguous.
func TestReconcile_TargetAmbiguousAcrossProviders(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, operationTime,
		identityObj("jane", "corp-ldap", func(id *useridentity.Identity) {
			id.SetClaims(useridentity.Claims{UserID: "jane", Email: "jane@example.org"})
		}),
		identityObj("10001", "corp-oidc", func(id *useridentity.Identity) {
			id.SetClaims(useridentity.Claims{UserID: "10001", Email: "jane@example.org"})
		}),
		userOperationObj("op", operationTime, map[string]any{
			"type": "Lock", "initiatorType": "admin",
			"target": map[string]any{"email": "jane@example.org"},
			"lock":   map[string]any{"for": "1h"},
		}),
	)
	if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertPhase(t, r, "op", "Failed", operationTime)
	uo := getUnstructured(t, r, controller.UserOperationGVK, "", "op")
	if msg, _, _ := unstructured.NestedString(uo.Object, "status", "message"); msg != `2 accounts have email "jane@example.org"; the target is ambiguous` {
		t.Errorf("status.message = %q", msg)
	}
}

// A target never resolves to a local user: the local path also writes the Password object.
func TestReconcile_TargetSkipsLocalUsers(t *testing.T) {
	t.Parallel()

	objects := func(spec map[string]any) []client.Object {
		return []client.Object{
			identityObj("jane", naming.LocalConnectorID, func(id *useridentity.Identity) {
				id.SetClaims(useridentity.Claims{UserID: "jane", Email: "jane@example.org"})
			}),
			identityObj("cn=jane", "corp-ldap", func(id *useridentity.Identity) {
				id.SetClaims(useridentity.Claims{UserID: "cn=jane", Email: "jane@example.org"})
			}),
			userOperationObj("op", operationTime, spec),
		}
	}

	t.Run("without connectorID", func(t *testing.T) {
		t.Parallel()
		r := newTestReconciler(t, operationTime, objects(map[string]any{
			"type": "Lock", "initiatorType": "admin",
			"target": map[string]any{"email": "jane@example.org"},
			"lock":   map[string]any{"for": "1h"},
		})...)
		if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		assertPhase(t, r, "op", "Succeeded", operationTime)
		if g, _ := getIdentityOrNil(t, r, "jane", naming.LocalConnectorID); g != 0 {
			t.Errorf("local generation = %d, want the local user untouched", g)
		}
		if g, _ := getIdentityOrNil(t, r, "cn=jane", "corp-ldap"); g != 1 {
			t.Errorf("ldap generation = %d, want 1", g)
		}
	})

	t.Run("local connectorID", func(t *testing.T) {
		t.Parallel()
		r := newTestReconciler(t, operationTime, objects(map[string]any{
			"type": "Lock", "initiatorType": "admin",
			"target": map[string]any{"email": "jane@example.org", "connectorID": naming.LocalConnectorID},
			"lock":   map[string]any{"for": "1h"},
		})...)
		if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		assertPhase(t, r, "op", "Failed", operationTime)
		uo := getUnstructured(t, r, controller.UserOperationGVK, "", "op")
		if msg, _, _ := unstructured.NestedString(uo.Object, "status", "message"); msg != "target cannot name the local provider; use spec.user for a local user" {
			t.Errorf("status.message = %q", msg)
		}
	})
}

func TestReconcile_RequirePasswordChange(t *testing.T) {
	t.Parallel()

	t.Run("local user", func(t *testing.T) {
		t.Parallel()
		r := newTestReconciler(t, operationTime, append(signedIn(), userOperationObj("op", operationTime, map[string]any{
			"type": "RequirePasswordChange", "initiatorType": "self", "user": "admin",
		}))...)
		if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
		assertPhase(t, r, "op", "Succeeded", operationTime)
		pw := getUnstructured(t, r, controller.PasswordGVK, naming.DexNamespace, passwordObjectName)
		if flag, _, _ := unstructured.NestedBool(pw.Object, "requireResetHashOnNextSuccLogin"); !flag {
			t.Error("requireResetHashOnNextSuccLogin is not set")
		}
		if hash, _, _ := unstructured.NestedString(pw.Object, "hash"); hash != "old" {
			t.Errorf("hash = %q, want it unchanged", hash)
		}
		// Nothing is revoked: the user keeps the sessions until the next sign-in.
		getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-console")
		getUnstructured(t, r, controller.AuthSessionGVK, naming.DexNamespace, "sid-admin")
		uo := getUnstructured(t, r, controller.UserOperationGVK, "", "op")
		if _, found, _ := unstructured.NestedFieldNoCopy(uo.Object, "status", "revocation"); found {
			t.Error("status.revocation is set")
		}
		if g, _ := getIdentityOrNil(t, r, "admin", naming.LocalConnectorID); g != 0 {
			t.Errorf("generation = %d, want none", g)
		}
	})

	for _, tt := range []struct {
		name string
		spec map[string]any
	}{
		{
			name: "external target",
			spec: map[string]any{
				"type": "RequirePasswordChange", "initiatorType": "self",
				"target": map[string]any{"email": "admin@corp.example.org"},
			},
		},
		{
			name: "unknown user",
			spec: map[string]any{"type": "RequirePasswordChange", "initiatorType": "self", "user": "nobody"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := newTestReconciler(t, operationTime, append(signedIn(), userOperationObj("op", operationTime, tt.spec))...)
			if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			assertPhase(t, r, "op", "Failed", operationTime)
		})
	}
}

// Unlock changes no generation and writes no Revocation.
func TestReconcile_UnlockRevokesNothing(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, operationTime, append(signedIn(), userOperationObj("op", operationTime, map[string]any{
		"type": "Unlock", "initiatorType": "admin", "user": "admin",
	}))...)
	if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertPhase(t, r, "op", "Succeeded", operationTime)
	if g, _ := getIdentityOrNil(t, r, "admin", naming.LocalConnectorID); g != 0 {
		t.Errorf("generation = %d, want none", g)
	}
	getUnstructured(t, r, controller.RefreshTokenGVK, naming.DexNamespace, "rt-console")
}

// callLog records "<verb> <Kind> <name>" of every write.
type callLog struct {
	mu    sync.Mutex
	calls []string
}

func (l *callLog) add(verb string, obj client.Object) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, verb+" "+obj.GetObjectKind().GroupVersionKind().Kind+" "+obj.GetName())
}

func (l *callLog) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.calls)
}

func getRevocationObj(t *testing.T, r *Reconciler, name string) revocation.Revocation {
	t.Helper()
	rev, err := revocation.FromObject(getUnstructured(t, r, controller.RevocationGVK, naming.DexNamespace, name))
	if err != nil {
		t.Fatal(err)
	}
	return rev
}

// getIdentityOrNil returns the generation of the pair, 0 when it has no UserIdentity.
func getIdentityOrNil(t *testing.T, r *Reconciler, userID, connID string) (uint64, bool) {
	t.Helper()
	obj := controller.Object(controller.UserIdentityGVK)
	err := r.client.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: useridentity.Name(userID, connID)}, obj)
	if apierrors.IsNotFound(err) {
		return 0, false
	}
	if err != nil {
		t.Fatal(err)
	}
	g, err := useridentity.FromUnstructured(obj).Generation()
	if err != nil {
		t.Fatal(err)
	}
	return g, true
}

// An unreadable generation fails the operation: Dex already refuses the pair, and a retry would
// change nothing.
func TestReconcile_InvalidGenerationFails(t *testing.T) {
	t.Parallel()

	broken := identityObj("admin", naming.LocalConnectorID, func(id *useridentity.Identity) {
		id.Object().SetAnnotations(map[string]string{useridentity.GenerationAnnotation: "07"})
	})
	r := newTestReconciler(t, operationTime, append(signedIn(), broken, userOperationObj("op", operationTime, map[string]any{
		"type": "ResetPassword", "initiatorType": "admin", "user": "admin",
		"resetPassword": map[string]any{"newPasswordHash": rawBcryptHash},
	}))...)
	if _, err := r.Reconcile(t.Context(), opRequest("op")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertPhase(t, r, "op", "Failed", operationTime)
	uo := getUnstructured(t, r, controller.UserOperationGVK, "", "op")
	if msg, _, _ := unstructured.NestedString(uo.Object, "status", "message"); !strings.Contains(msg, "not a generation") {
		t.Errorf("status.message = %q", msg)
	}
}
