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

package identitymigration

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

func TestReconcile_CopiesLocalPair(t *testing.T) {
	t.Parallel()

	refreshAt := time.Date(2026, 10, 10, 8, 30, 0, 0, time.UTC)
	session := sessionObj("jane", naming.LocalConnectorID, sessionOpts{
		email:         "jane@example.com",
		totp:          totpSecretA,
		totpConfirmed: true,
		refresh:       map[string]string{"kubeconfig-generator": "rt-1"},
		refreshAt:     refreshAt,
	})
	password := passwordObj("jane", "jane@example.com", passwordOpts{
		groups:      []string{"admins", "devs"},
		lockedUntil: &testLock,
		adminLock:   true,
	})
	h := newHarness(t, nil, session, password)
	name := useridentity.Name("jane", naming.LocalConnectorID)

	h.reconcile(t, name)

	id := h.identity(t, name)
	if id.UserID() != "jane" || id.ConnectorID() != naming.LocalConnectorID {
		t.Fatalf("pair = (%q, %q)", id.UserID(), id.ConnectorID())
	}
	claims, err := id.Claims()
	if err != nil {
		t.Fatal(err)
	}
	expectedClaims := useridentity.Claims{
		UserID:        "jane",
		Username:      "jane",
		Email:         "jane@example.com",
		EmailVerified: true,
		Groups:        []string{"admins", "devs"},
	}
	if !claims.Equal(expectedClaims) {
		t.Errorf("claims = %+v, want %+v (from the Password, as Dex's local connector)", claims, expectedClaims)
	}
	totp, err := id.TOTP()
	if err != nil || totp == nil {
		t.Fatalf("totp = %v, %v; want the secret read through the API reader", totp, err)
	}
	if totp.Secret != totpSecretA || !totp.Confirmed || !totp.CreatedAt.Equal(testCreatedAt) {
		t.Errorf("totp = %+v", totp)
	}
	blocked, _ := id.BlockedUntil()
	if !blocked.Equal(testLock) {
		t.Errorf("blockedUntil = %v, want %v", blocked, testLock)
	}
	if !id.LockedByAdministrator() {
		t.Error("the administrator lock of the Password lost its marker")
	}
	lastLogin, _ := id.LastLogin()
	if !lastLogin.Equal(refreshAt) {
		t.Errorf("lastLogin = %v, want %v (newest refresh token)", lastLogin, refreshAt)
	}
	created, _ := id.CreatedAt()
	if !created.Equal(testCreatedAt) {
		t.Errorf("createdAt = %v, want the creation time of OfflineSessions", created)
	}

	rec := h.record(t, name)
	expectedRecord := useridentity.Record{
		Time:      useridentity.FormatTime(testNow),
		Source:    "OfflineSessions,Password",
		TOTPFrom:  string(session.GetUID()),
		LockFrom:  useridentity.FormatTime(testLock),
		LastLogin: useridentity.FormatTime(refreshAt),
	}
	if rec != expectedRecord {
		t.Errorf("record = %+v, want %+v", rec, expectedRecord)
	}
	if got := metricValue(t, h.m.results.WithLabelValues(resultMigrated)); got != 1 {
		t.Errorf("migrated = %v, want 1", got)
	}
}

func TestReconcile_CopiesExternalPair(t *testing.T) {
	t.Parallel()

	older := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	session := sessionObj("cn=alice,ou=people", "corp-oidc", sessionOpts{
		email:       "alice@old.example.com",
		totp:        totpSecretA, // TOTP was only ever enabled for local users: not copied
		lockedUntil: &testLock,
		adminLock:   true,
		refresh:     map[string]string{"grafana": "rt-a", "kubeconfig-generator": "rt-b"},
		refreshAt:   older,
	})
	tokens := []client.Object{
		tokenObj("rt-a", "corp-oidc", older, map[string]any{
			"userID": "cn=alice,ou=people", "username": "alice-old", "email": "alice@old.example.com", "groups": []any{"devs"},
		}),
		tokenObj("rt-b", "corp-oidc", newer, map[string]any{
			"userID": "cn=alice,ou=people", "username": "alice", "preferredUsername": "alice",
			"email": "alice@example.com", "emailVerified": true, "groups": []any{"admins"},
		}),
	}
	h := newHarness(t, nil, append(tokens, session)...)
	name := useridentity.Name("cn=alice,ou=people", "corp-oidc")

	h.reconcile(t, name)

	id := h.identity(t, name)
	claims, _ := id.Claims()
	expectedClaims := useridentity.Claims{
		UserID:            "cn=alice,ou=people",
		Username:          "alice",
		PreferredUsername: "alice",
		Email:             "alice@example.com",
		EmailVerified:     true,
		Groups:            []string{"admins", "devs"},
	}
	if !claims.Equal(expectedClaims) {
		t.Errorf("claims = %+v, want %+v (newest token, groups of every client)", claims, expectedClaims)
	}
	if totp, _ := id.TOTP(); totp != nil {
		t.Errorf("totp = %+v, want none for an external pair", totp)
	}
	blocked, _ := id.BlockedUntil()
	if !blocked.Equal(testLock) || !id.LockedByAdministrator() {
		t.Errorf("blockedUntil = %v, admin = %v; want the administrator lock of OfflineSessions", blocked, id.LockedByAdministrator())
	}
	if rec := h.record(t, name); rec.Source != useridentity.SourceOfflineSessions || rec.TOTPFrom != "" {
		t.Errorf("record = %+v", rec)
	}
}

func TestReconcile_LockedLocalUserWithoutSession(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		lockedUntil  *time.Time
		expectCreate bool
	}{
		{name: "lock in force creates the identity", lockedUntil: &testLock, expectCreate: true},
		{name: "expired lock does not", lockedUntil: ptr(testNow.Add(-time.Hour)), expectCreate: false},
		{name: "no lock does not", expectCreate: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, nil, passwordObj("bob", "bob@example.com", passwordOpts{lockedUntil: tt.lockedUntil}))
			name := useridentity.Name("bob", naming.LocalConnectorID)

			h.reconcile(t, name)

			if got := h.identityExists(t, name); got != tt.expectCreate {
				t.Fatalf("identity exists = %v, want %v", got, tt.expectCreate)
			}
			if !tt.expectCreate {
				return
			}
			id := h.identity(t, name)
			blocked, _ := id.BlockedUntil()
			claims, _ := id.Claims()
			if !blocked.Equal(testLock) || claims.Email != "bob@example.com" {
				t.Errorf("blockedUntil = %v, claims = %+v", blocked, claims)
			}
			if id.LockedByAdministrator() {
				t.Error("an automatic lock got the administrator marker")
			}
		})
	}
}

func TestReconcile_SkipsPairsItCannotCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pair    string
		objects []client.Object
	}{
		{
			name:    "local session of a deleted user",
			pair:    useridentity.Name("gone", naming.LocalConnectorID),
			objects: []client.Object{sessionObj("gone", naming.LocalConnectorID, sessionOpts{totp: totpSecretA, totpConfirmed: true})},
		},
		{
			name:    "session named after another pair",
			pair:    "notthepairname",
			objects: []client.Object{sessionNamed("notthepairname", "jane", "corp-ldap", sessionOpts{})},
		},
		{
			name:    "session without user id",
			pair:    useridentity.Name("", "corp-ldap"),
			objects: []client.Object{sessionNamed(useridentity.Name("", "corp-ldap"), "", "corp-ldap", sessionOpts{})},
		},
		{
			name: "identity of another pair under the name",
			pair: useridentity.Name("jane", "corp-ldap"),
			objects: []client.Object{
				sessionObj("jane", "corp-ldap", sessionOpts{email: "jane@example.com"}),
				func() client.Object {
					obj := useridentity.New("jane", "corp-ldap").Object()
					obj.Object["userID"] = "john"
					return obj
				}(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, nil, tt.objects...)

			h.reconcile(t, tt.pair)

			if got := metricValue(t, h.m.results.WithLabelValues(resultSkipped)); got != 1 {
				t.Errorf("skipped = %v, want 1", got)
			}
			if tt.name != "identity of another pair under the name" && h.identityExists(t, tt.pair) {
				t.Error("a skipped pair got a UserIdentity")
			}
			if h.m.progress.value() != 0 {
				t.Errorf("pending = %d, want 0: a skipped pair is not retried", h.m.progress.value())
			}
		})
	}
}

// In 1.77, deleting a User deleted only its Password and left the OfflineSessions with the TOTP
// secret. After the upgrade, a user created again with the same name must not get that secret.
func TestReconcile_UserDeletedBeforeTheUpgrade(t *testing.T) {
	t.Parallel()

	var deletedUID atomic.Value
	funcs := &interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			var o client.DeleteOptions
			o.ApplyOptions(opts)
			if o.Preconditions != nil && o.Preconditions.UID != nil {
				deletedUID.Store(string(*o.Preconditions.UID))
			}
			return c.Delete(ctx, obj, opts...)
		},
	}
	orphan := sessionObj("jane", naming.LocalConnectorID, sessionOpts{uid: "old-uid", totp: totpSecretA, totpConfirmed: true})
	h := newHarness(t, funcs, orphan)
	name := useridentity.Name("jane", naming.LocalConnectorID)

	// The first pass after the upgrade.
	h.reconcile(t, name)

	if h.sessionExists(t, name) {
		t.Fatal("the OfflineSessions of the deleted user is kept: the old Dex would hand its secret to a new jane")
	}
	if got, _ := deletedUID.Load().(string); got != "old-uid" {
		t.Errorf("delete precondition uid = %q, want old-uid", got)
	}
	if h.identityExists(t, name) {
		t.Error("the pair of the deleted user got a UserIdentity")
	}
	if got := metricValue(t, h.m.results.WithLabelValues(resultSkipped)); got != 1 {
		t.Errorf("skipped = %v, want 1", got)
	}

	// The administrator creates jane again; the controller creates her Password.
	for _, obj := range []client.Object{userObj("jane", "jane@example.com"), passwordObj("jane", "jane@example.com", passwordOpts{})} {
		if err := h.api.Create(t.Context(), obj); err != nil {
			t.Fatal(err)
		}
	}
	h.reconcile(t, name)

	if h.identityExists(t, name) {
		if totp, _ := h.identity(t, name).TOTP(); totp != nil {
			t.Errorf("totp of the new jane = %+v, want none", totp)
		}
	}
}

// A User whose Password the User controller has not created yet keeps its OfflineSessions; the copy
// waits for the Password.
func TestReconcile_LocalUserWithoutAPasswordYet(t *testing.T) {
	t.Parallel()

	h := newHarness(t, nil,
		userObj("jane", "jane@example.com"),
		sessionObj("jane", naming.LocalConnectorID, sessionOpts{totp: totpSecretA, totpConfirmed: true}),
	)
	name := useridentity.Name("jane", naming.LocalConnectorID)

	h.reconcile(t, name)

	if !h.sessionExists(t, name) {
		t.Fatal("the OfflineSessions of an existing User was deleted")
	}
	if h.identityExists(t, name) {
		t.Fatal("a pair without a Password got a UserIdentity")
	}

	if err := h.api.Create(t.Context(), passwordObj("jane", "jane@example.com", passwordOpts{})); err != nil {
		t.Fatal(err)
	}
	h.reconcile(t, name)

	if totp, _ := h.identity(t, name).TOTP(); totp == nil || totp.Secret != totpSecretA {
		t.Errorf("totp = %+v, want the secret of the existing User copied", totp)
	}
}

func TestReconcile_SecondRunWritesNothing(t *testing.T) {
	t.Parallel()

	var writes atomic.Int32
	countWrites := &interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			writes.Add(1)
			return c.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			writes.Add(1)
			return c.Update(ctx, obj, opts...)
		},
	}
	h := newHarness(t, countWrites,
		sessionObj("jane", naming.LocalConnectorID, sessionOpts{
			email: "jane@example.com", totp: totpSecretA, totpConfirmed: true,
			refresh: map[string]string{"console": "rt-1"}, refreshAt: testCreatedAt.Add(time.Hour),
		}),
		passwordObj("jane", "jane@example.com", passwordOpts{lockedUntil: &testLock}),
		tokenObj("rt-1", naming.LocalConnectorID, testCreatedAt.Add(time.Hour), map[string]any{"userID": "jane", "email": "jane@example.com"}),
	)
	name := useridentity.Name("jane", naming.LocalConnectorID)

	h.reconcile(t, name)
	first := writes.Load()
	h.reconcile(t, name)

	if first != 1 {
		t.Errorf("first run writes = %d, want 1", first)
	}
	if writes.Load() != first {
		t.Errorf("second run wrote %d times, want 0", writes.Load()-first)
	}
	if got := metricValue(t, h.m.results.WithLabelValues(resultUnchanged)); got != 1 {
		t.Errorf("unchanged = %v, want 1", got)
	}
}

func TestReconcile_DoesNotBringBackWhatWasRemoved(t *testing.T) {
	t.Parallel()

	session := sessionObj("jane", naming.LocalConnectorID, sessionOpts{
		uid: "session-x", email: "jane@example.com", totp: totpSecretA, totpConfirmed: true,
	})
	password := passwordObj("jane", "jane@example.com", passwordOpts{lockedUntil: &testLock, adminLock: true})
	// The identity got both copied once; then its second factor was reset and its lock lifted in
	// the new place only.
	cleared := identityObj(t, "jane", naming.LocalConnectorID, func(id *useridentity.Identity) {
		id.SetClaims(useridentity.Claims{UserID: "jane", Username: "jane", Email: "jane@example.com", EmailVerified: true})
		if err := id.SetCreatedAt(testCreatedAt); err != nil {
			t.Fatal(err)
		}
		if err := id.SetRecord(useridentity.Record{
			Time:     useridentity.FormatTime(testNow.Add(-time.Hour)),
			Source:   "OfflineSessions,Password",
			TOTPFrom: "session-x",
			LockFrom: useridentity.FormatTime(testLock),
		}); err != nil {
			t.Fatal(err)
		}
	})
	h := newHarness(t, nil, session, password, cleared)
	name := useridentity.Name("jane", naming.LocalConnectorID)

	h.reconcile(t, name)

	id := h.identity(t, name)
	if totp, _ := id.TOTP(); totp != nil {
		t.Errorf("totp = %+v: the secret removed from UserIdentity came back", totp)
	}
	if blocked, _ := id.BlockedUntil(); !blocked.IsZero() {
		t.Errorf("blockedUntil = %v: the lock lifted in UserIdentity came back", blocked)
	}
}

func TestReconcile_TOTPRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		sourceUID       string
		sourceSecret    string
		sourceConfirmed bool
		current         *useridentity.MFASecret
		totpFrom        string
		expectSecret    string
		expectConfirmed bool
		expectFrom      string
	}{
		{
			name:      "confirmation is copied",
			sourceUID: "x", sourceSecret: totpSecretA, sourceConfirmed: true,
			current:      &useridentity.MFASecret{Secret: totpSecretA},
			totpFrom:     "x",
			expectSecret: totpSecretA, expectConfirmed: true, expectFrom: "x",
		},
		{
			name:      "newer enrollment in the old place replaces an older copy",
			sourceUID: "y", sourceSecret: totpSecretB, sourceConfirmed: true,
			current:      &useridentity.MFASecret{Secret: totpSecretA, Confirmed: true},
			totpFrom:     "x",
			expectSecret: totpSecretB, expectConfirmed: true, expectFrom: "y",
		},
		{
			name:      "secret enrolled after a reset in the old place is copied",
			sourceUID: "y", sourceSecret: totpSecretB, sourceConfirmed: false,
			totpFrom:     "x",
			expectSecret: totpSecretB, expectConfirmed: false, expectFrom: "y",
		},
		{
			name:      "confirmed secret enrolled in the new place stays",
			sourceUID: "x", sourceSecret: totpSecretA, sourceConfirmed: true,
			current:      &useridentity.MFASecret{Secret: totpSecretB, Confirmed: true},
			expectSecret: totpSecretB, expectConfirmed: true, expectFrom: "x",
		},
		{
			name:      "unconfirmed secret of the new place yields to a confirmed one",
			sourceUID: "x", sourceSecret: totpSecretA, sourceConfirmed: true,
			current:      &useridentity.MFASecret{Secret: totpSecretB},
			expectSecret: totpSecretA, expectConfirmed: true, expectFrom: "x",
		},
		{
			name:      "unconfirmed enrollment started after the copy stays",
			sourceUID: "x", sourceSecret: totpSecretA, sourceConfirmed: true,
			current:      &useridentity.MFASecret{Secret: totpSecretB},
			totpFrom:     "x",
			expectSecret: totpSecretB, expectConfirmed: false, expectFrom: "x",
		},
		{
			name:      "confirmed copy is not replaced by an unconfirmed enrollment",
			sourceUID: "y", sourceSecret: totpSecretB, sourceConfirmed: false,
			current:      &useridentity.MFASecret{Secret: totpSecretA, Confirmed: true},
			totpFrom:     "x",
			expectSecret: totpSecretA, expectConfirmed: true, expectFrom: "x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := sessionObj("jane", naming.LocalConnectorID, sessionOpts{
				uid: tt.sourceUID, totp: tt.sourceSecret, totpConfirmed: tt.sourceConfirmed,
			})
			existing := identityObj(t, "jane", naming.LocalConnectorID, func(id *useridentity.Identity) {
				id.SetClaims(useridentity.Claims{UserID: "jane", Email: "jane@example.com"})
				if tt.current != nil {
					id.SetTOTP(*tt.current)
				}
				if tt.totpFrom != "" {
					if err := id.SetRecord(useridentity.Record{Time: "2026-10-01T00:00:00Z", Source: "OfflineSessions", TOTPFrom: tt.totpFrom}); err != nil {
						t.Fatal(err)
					}
				}
			})
			h := newHarness(t, nil, session, existing, passwordObj("jane", "jane@example.com", passwordOpts{}))
			name := useridentity.Name("jane", naming.LocalConnectorID)

			h.reconcile(t, name)

			totp, err := h.identity(t, name).TOTP()
			if err != nil || totp == nil {
				t.Fatalf("totp = %v, %v", totp, err)
			}
			if totp.Secret != tt.expectSecret || totp.Confirmed != tt.expectConfirmed {
				t.Errorf("totp = (%s, confirmed %v), want (%s, confirmed %v)", totp.Secret, totp.Confirmed, tt.expectSecret, tt.expectConfirmed)
			}
			if rec := h.record(t, name); rec.TOTPFrom != tt.expectFrom {
				t.Errorf("totpFrom = %q, want %q", rec.TOTPFrom, tt.expectFrom)
			}
		})
	}
}

// The old Dex enrolls a new secret unconfirmed and marks it confirmed only after the user enters a
// code. A copy that sees the enrollment before the confirmation holds it back behind the confirmed
// copy and replaces the copy once the enrollment is confirmed.
func TestReconcile_EnrollmentConfirmedLaterReplacesAnOlderCopy(t *testing.T) {
	t.Parallel()

	copied := identityObj(t, "jane", naming.LocalConnectorID, func(id *useridentity.Identity) {
		id.SetClaims(useridentity.Claims{UserID: "jane", Email: "jane@example.com"})
		id.SetTOTP(useridentity.MFASecret{Secret: totpSecretA, Confirmed: true})
		if err := id.SetRecord(useridentity.Record{Time: "2026-10-01T00:00:00Z", Source: "OfflineSessions", TOTPFrom: "uid-1"}); err != nil {
			t.Fatal(err)
		}
	})
	// Lock or ResetPassword deleted the session uid-1; the next login created uid-2 with a new secret.
	enrolled := sessionObj("jane", naming.LocalConnectorID, sessionOpts{uid: "uid-2", totp: totpSecretB})
	h := newHarness(t, nil, copied, enrolled, passwordObj("jane", "jane@example.com", passwordOpts{}))
	name := useridentity.Name("jane", naming.LocalConnectorID)

	h.reconcile(t, name)

	if totp, _ := h.identity(t, name).TOTP(); totp == nil || totp.Secret != totpSecretA || !totp.Confirmed {
		t.Fatalf("totp before the confirmation = %+v, want the confirmed copy kept", totp)
	}
	if rec := h.record(t, name); rec.TOTPFrom != "uid-1" {
		t.Fatalf("totpFrom before the confirmation = %q, want uid-1: the enrollment is only held back", rec.TOTPFrom)
	}

	session := controller.Object(controller.OfflineSessionsGVK)
	if err := h.api.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, session); err != nil {
		t.Fatal(err)
	}
	session.Object["totpConfirmed"] = true
	if err := h.api.Update(t.Context(), session); err != nil {
		t.Fatal(err)
	}
	h.reconcile(t, name)

	totp, _ := h.identity(t, name).TOTP()
	if totp == nil || totp.Secret != totpSecretB || !totp.Confirmed {
		t.Errorf("totp after the confirmation = %+v, want the newer secret, confirmed", totp)
	}
	if rec := h.record(t, name); rec.TOTPFrom != "uid-2" {
		t.Errorf("totpFrom after the confirmation = %q, want uid-2", rec.TOTPFrom)
	}
}

func TestReconcile_LaterLockWins(t *testing.T) {
	t.Parallel()

	earlier := testLock.Add(-24 * time.Hour)
	later := testLock.Add(24 * time.Hour)
	tests := []struct {
		name          string
		sourceLock    time.Time
		sourceAdmin   bool
		currentLock   time.Time
		currentAdmin  bool
		expectLock    time.Time
		expectByAdmin bool
	}{
		{name: "later old-place lock replaces", sourceLock: later, currentLock: earlier, currentAdmin: true, expectLock: later},
		{name: "later administrator lock replaces", sourceLock: later, sourceAdmin: true, currentLock: earlier, expectLock: later, expectByAdmin: true},
		{name: "earlier old-place lock does not shorten", sourceLock: earlier, sourceAdmin: true, currentLock: later, expectLock: later},
		{name: "permanent lock in the new place stays", sourceLock: later, currentLock: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), currentAdmin: true, expectLock: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), expectByAdmin: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := sessionObj("jane", "corp-ldap", sessionOpts{email: "jane@example.com", lockedUntil: &tt.sourceLock, adminLock: tt.sourceAdmin})
			existing := identityObj(t, "jane", "corp-ldap", func(id *useridentity.Identity) {
				id.SetClaims(useridentity.Claims{UserID: "jane", Email: "jane@example.com"})
				if err := id.SetBlockedUntil(tt.currentLock); err != nil {
					t.Fatal(err)
				}
				if tt.currentAdmin {
					id.SetAdminLock(tt.currentLock)
				}
			})
			h := newHarness(t, nil, session, existing)
			name := useridentity.Name("jane", "corp-ldap")

			h.reconcile(t, name)

			id := h.identity(t, name)
			blocked, _ := id.BlockedUntil()
			if !blocked.Equal(tt.expectLock) {
				t.Errorf("blockedUntil = %v, want %v", blocked, tt.expectLock)
			}
			if id.LockedByAdministrator() != tt.expectByAdmin {
				t.Errorf("locked by administrator = %v, want %v", id.LockedByAdministrator(), tt.expectByAdmin)
			}
		})
	}
}

func TestReconcile_ClaimsFollowTheOldPlaceUntilDexLogsTheUserIn(t *testing.T) {
	t.Parallel()

	copiedLogin := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	dexLogin := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	newLogin := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		lastLogin      time.Time
		expectEmail    string
		expectLogin    time.Time
		expectInRecord string
	}{
		{name: "controller still owns the claims", lastLogin: copiedLogin, expectEmail: "alice@new.example.com", expectLogin: newLogin, expectInRecord: useridentity.FormatTime(newLogin)},
		{name: "dex has logged the user in since", lastLogin: dexLogin, expectEmail: "alice@old.example.com", expectLogin: dexLogin, expectInRecord: useridentity.FormatTime(copiedLogin)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			session := sessionObj("alice", "corp-ldap", sessionOpts{
				email: "alice@new.example.com", refresh: map[string]string{"console": "rt-new"}, refreshAt: newLogin,
			})
			token := tokenObj("rt-new", "corp-ldap", newLogin, map[string]any{"userID": "alice", "email": "alice@new.example.com"})
			existing := identityObj(t, "alice", "corp-ldap", func(id *useridentity.Identity) {
				id.SetClaims(useridentity.Claims{UserID: "alice", Email: "alice@old.example.com"})
				if err := id.SetLastLogin(tt.lastLogin); err != nil {
					t.Fatal(err)
				}
				if err := id.SetRecord(useridentity.Record{
					Time:      "2026-10-01T00:00:00Z",
					Source:    "OfflineSessions",
					LastLogin: useridentity.FormatTime(copiedLogin),
				}); err != nil {
					t.Fatal(err)
				}
			})
			h := newHarness(t, nil, session, token, existing)
			name := useridentity.Name("alice", "corp-ldap")

			h.reconcile(t, name)

			id := h.identity(t, name)
			claims, _ := id.Claims()
			lastLogin, _ := id.LastLogin()
			if claims.Email != tt.expectEmail || !lastLogin.Equal(tt.expectLogin) {
				t.Errorf("email = %q, lastLogin = %v; want %q, %v", claims.Email, lastLogin, tt.expectEmail, tt.expectLogin)
			}
			if rec := h.record(t, name); rec.LastLogin != tt.expectInRecord {
				t.Errorf("record lastLogin = %q, want %q", rec.LastLogin, tt.expectInRecord)
			}
		})
	}
}

func TestReconcile_RetriesAfterAConflictWithDex(t *testing.T) {
	t.Parallel()

	var conflicts atomic.Int32
	funcs := &interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if conflicts.Add(1) == 1 {
				return apierrors.NewConflict(schema.GroupResource{Group: "dex.coreos.com", Resource: "useridentities"}, obj.GetName(), errors.New("dex wrote first"))
			}
			return c.Update(ctx, obj, opts...)
		},
	}
	existing := identityObj(t, "jane", "corp-ldap", func(id *useridentity.Identity) {
		id.SetClaims(useridentity.Claims{UserID: "jane"})
	})
	h := newHarness(t, funcs, existing, sessionObj("jane", "corp-ldap", sessionOpts{email: "jane@example.com", lockedUntil: &testLock}))
	name := useridentity.Name("jane", "corp-ldap")

	h.reconcile(t, name)

	if blocked, _ := h.identity(t, name).BlockedUntil(); !blocked.Equal(testLock) {
		t.Errorf("blockedUntil = %v, want %v after the retry", blocked, testLock)
	}
}

func TestReconcile_FailedPairStaysPending(t *testing.T) {
	t.Parallel()

	var fail atomic.Bool
	fail.Store(true)
	funcs := &interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if fail.Load() {
				return apierrors.NewServiceUnavailable("api server is restarting")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}
	h := newHarness(t, funcs, sessionObj("jane", "corp-ldap", sessionOpts{email: "jane@example.com"}))
	name := useridentity.Name("jane", "corp-ldap")
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}

	if _, err := h.m.Reconcile(t.Context(), req); err == nil {
		t.Fatal("Reconcile succeeded with the API reader failing")
	}
	if h.m.progress.value() != 1 {
		t.Errorf("pending = %d, want 1 after a failure", h.m.progress.value())
	}
	if got := metricValue(t, h.m.results.WithLabelValues(resultFailed)); got != 1 {
		t.Errorf("failed = %v, want 1", got)
	}

	fail.Store(false)
	if _, err := h.m.Reconcile(t.Context(), req); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if h.m.progress.value() != 0 {
		t.Errorf("pending = %d, want 0 after the retry succeeded", h.m.progress.value())
	}
}

func TestReconcile_NothingInTheOldPlaces(t *testing.T) {
	t.Parallel()

	h := newHarness(t, nil)
	name := useridentity.Name("nobody", "corp-ldap")

	h.reconcile(t, name)

	if h.identityExists(t, name) {
		t.Error("a pair without old places got a UserIdentity")
	}
}

func TestChangesOf(t *testing.T) {
	t.Parallel()

	base := sessionObj("jane", "corp-ldap", sessionOpts{email: "jane@example.com", refresh: map[string]string{"console": "rt-1"}, refreshAt: testCreatedAt})
	stripped := func(u *unstructured.Unstructured) *unstructured.Unstructured {
		obj := u.DeepCopy()
		stripInPlace(obj)
		return obj
	}
	refreshed := base.DeepCopy()
	refreshed.Object["refresh"].(map[string]any)["console"].(map[string]any)["LastUsed"] = testNow.Format(time.RFC3339)
	relogged := base.DeepCopy()
	relogged.Object["refresh"].(map[string]any)["console"].(map[string]any)["ID"] = "rt-2"
	locked := base.DeepCopy()
	locked.Object["lockedUntil"] = testLock.Format(time.RFC3339)

	pred := changesOf(sessionFingerprint)
	tests := []struct {
		name     string
		pass     func() bool
		expected bool
	}{
		{name: "object of the initial list", pass: func() bool {
			return pred.Create(event.CreateEvent{Object: stripped(base), IsInInitialList: true})
		}, expected: false},
		{name: "object created later", pass: func() bool {
			return pred.Create(event.CreateEvent{Object: stripped(base)})
		}, expected: true},
		{name: "token refresh", pass: func() bool {
			return pred.Update(event.UpdateEvent{ObjectOld: stripped(base), ObjectNew: stripped(refreshed)})
		}, expected: false},
		{name: "new login", pass: func() bool {
			return pred.Update(event.UpdateEvent{ObjectOld: stripped(base), ObjectNew: stripped(relogged)})
		}, expected: true},
		{name: "lock", pass: func() bool {
			return pred.Update(event.UpdateEvent{ObjectOld: stripped(base), ObjectNew: stripped(locked)})
		}, expected: true},
		{name: "deletion", pass: func() bool {
			return pred.Delete(event.DeleteEvent{Object: stripped(base)})
		}, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.pass(); got != tt.expected {
				t.Errorf("passes = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMapPassword(t *testing.T) {
	t.Parallel()

	reqs := mapPassword(t.Context(), passwordObj("jane", "jane@example.com", passwordOpts{}))
	expected := []reconcile.Request{{NamespacedName: types.NamespacedName{Name: useridentity.Name("jane", naming.LocalConnectorID)}}}
	if !slices.Equal(reqs, expected) {
		t.Errorf("requests = %v, want %v", reqs, expected)
	}
	if reqs := mapPassword(t.Context(), controller.Object(controller.PasswordGVK)); len(reqs) != 0 {
		t.Errorf("requests for a Password without userID = %v", reqs)
	}
}

func TestRegister_WiresIntoAManager(t *testing.T) {
	t.Parallel()

	// Nothing connects to this address: the manager is built, never started.
	mgr, err := manager.New(&rest.Config{Host: "https://127.0.0.1:1"}, manager.Options{
		Scheme:                 runtime.NewScheme(),
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		MapperProvider: func(*rest.Config, *http.Client) (meta.RESTMapper, error) {
			return newTestMapper(), nil
		},
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := Register(mgr); err != nil {
		t.Fatalf("Register: %v", err)
	}
}

func TestNewRateLimiter_CapsTheRetryDelay(t *testing.T) {
	t.Parallel()

	limiter := newRateLimiter()
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "pair"}}

	if got := limiter.When(req); got != retryBaseDelay {
		t.Fatalf("first retry after %s, want %s", got, retryBaseDelay)
	}
	var last time.Duration
	for range 30 {
		last = limiter.When(req)
		if last > retryMaxDelay {
			t.Fatalf("retry after %s, want at most %s", last, retryMaxDelay)
		}
	}
	if last != retryMaxDelay {
		t.Fatalf("retry after many failures after %s, want %s", last, retryMaxDelay)
	}

	limiter.Forget(req)
	if got := limiter.When(req); got != retryBaseDelay {
		t.Fatalf("first retry after Forget after %s, want %s", got, retryBaseDelay)
	}
}

// Every pair failing at once, for example without rights on useridentities, must not delay the
// retries of one another: each keeps its own delay, at most a minute.
func TestNewRateLimiter_ManyPairsFailingAtOnce(t *testing.T) {
	t.Parallel()

	const pairs = 10000
	limiter := newRateLimiter()
	reqs := make([]reconcile.Request, pairs)
	for i := range reqs {
		reqs[i] = reconcile.Request{NamespacedName: types.NamespacedName{Name: "pair-" + strconv.Itoa(i)}}
	}

	// The delay doubles from a second and reaches the cap on the seventh failure in a row.
	for failure := range 8 {
		expected := min(retryBaseDelay<<failure, retryMaxDelay)
		for _, req := range reqs {
			if got := limiter.When(req); got != expected {
				t.Fatalf("failure %d of %s: retry after %s, want %s", failure+1, req.Name, got, expected)
			}
		}
	}
}
