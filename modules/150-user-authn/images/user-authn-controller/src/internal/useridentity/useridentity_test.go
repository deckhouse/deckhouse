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

package useridentity

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"user-authn-controller/internal/naming"
)

// The fixture is json.Marshal of github.com/dexidp/dex/storage/kubernetes.UserIdentity at
// v2.46.0, the struct Dex writes the object with: field names, the zero blockedUntil, nanosecond
// times, and the replay counter of a TOTP secret are as Dex has them.
const dexFixture = "testdata/dex-v2.46.0-useridentity.json"

func TestIdentity_ReadsWhatDexWrites(t *testing.T) {
	t.Parallel()

	id := FromUnstructured(loadFixture(t))

	if id.UserID() != "jane" || id.ConnectorID() != "local" {
		t.Fatalf("pair = (%q, %q), want (jane, local)", id.UserID(), id.ConnectorID())
	}
	claims, err := id.Claims()
	if err != nil {
		t.Fatalf("Claims: %v", err)
	}
	expectedClaims := Claims{UserID: "jane", Username: "jane", Email: "jane@example.com", EmailVerified: true, Groups: []string{"admins"}}
	if !claims.Equal(expectedClaims) {
		t.Errorf("claims = %+v, want %+v", claims, expectedClaims)
	}
	blocked, err := id.BlockedUntil()
	if err != nil {
		t.Fatalf("BlockedUntil: %v", err)
	}
	if !blocked.IsZero() {
		t.Errorf("blockedUntil = %v, want zero: Dex writes 0001-01-01 for no lock", blocked)
	}
	lastLogin, err := id.LastLogin()
	if err != nil {
		t.Fatalf("LastLogin: %v", err)
	}
	if expected := time.Date(2026, 10, 14, 10, 0, 0, 123456789, time.UTC); !lastLogin.Equal(expected) {
		t.Errorf("lastLogin = %v, want %v", lastLogin, expected)
	}
	totp, err := id.TOTP()
	if err != nil {
		t.Fatalf("TOTP: %v", err)
	}
	if totp == nil || !totp.Confirmed || totp.Type != MFATypeTOTP || totp.AuthenticatorID != AuthenticatorTOTP {
		t.Fatalf("totp = %+v, want a confirmed TOTP entry", totp)
	}
	if id.LockedByAdministrator() {
		t.Error("LockedByAdministrator = true for an object without a lock")
	}
}

func TestIdentity_ConfirmTOTPKeepsReplayCounter(t *testing.T) {
	t.Parallel()

	obj := loadFixture(t)
	secrets := obj.Object["mfaSecrets"].(map[string]any)
	secrets["totp"].(map[string]any)["confirmed"] = false
	id := FromUnstructured(obj)

	id.ConfirmTOTP()

	entry := obj.Object["mfaSecrets"].(map[string]any)["totp"].(map[string]any)
	if entry["confirmed"] != true {
		t.Errorf("confirmed = %v, want true", entry["confirmed"])
	}
	if entry["lastUsedCounter"] == nil {
		t.Error("lastUsedCounter was dropped; Dex uses it to refuse a replayed code")
	}
}

func TestIdentity_SetTOTPWritesDexShape(t *testing.T) {
	t.Parallel()

	id := New("jane", naming.LocalConnectorID)
	created := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	id.SetTOTP(MFASecret{Secret: "otpauth://totp/x?secret=ABC", Confirmed: true, CreatedAt: created})

	expected := map[string]any{
		"authenticatorID": "totp",
		"type":            "TOTP",
		"secret":          "otpauth://totp/x?secret=ABC",
		"confirmed":       true,
		"createdAt":       "2026-09-01T08:00:00Z",
	}
	got := id.Object().Object["mfaSecrets"].(map[string]any)["totp"].(map[string]any)
	if len(got) != len(expected) {
		t.Fatalf("totp entry = %v, want %v", got, expected)
	}
	for key, value := range expected {
		if got[key] != value {
			t.Errorf("totp.%s = %v, want %v", key, got[key], value)
		}
	}
	if id.Object().GetName() != naming.OfflineTokenName("jane", naming.LocalConnectorID) {
		t.Errorf("name = %q, want the OfflineSessions name of the pair", id.Object().GetName())
	}
}

func TestIdentity_ClearMFARemovesEveryFactor(t *testing.T) {
	t.Parallel()

	obj := loadFixture(t)
	obj.Object["webauthnCredentials"] = map[string]any{"key": []any{map[string]any{"displayName": "yubikey"}}}
	id := FromUnstructured(obj)

	id.ClearMFA()

	for _, field := range []string{"mfaSecrets", "webauthnCredentials"} {
		if _, ok := obj.Object[field]; ok {
			t.Errorf("%s is still set", field)
		}
	}
	if _, ok := obj.Object["claims"]; !ok {
		t.Error("ClearMFA removed claims")
	}
}

func TestIdentity_SetTimeKeepsEqualInstant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		current  any
		set      time.Time
		expected any
	}{
		{
			name:     "same instant written by dex with nanoseconds",
			current:  "2026-10-14T10:00:00.000000000Z",
			set:      time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC),
			expected: "2026-10-14T10:00:00.000000000Z",
		},
		{
			name:     "dex zero time stays when cleared",
			current:  "0001-01-01T00:00:00Z",
			set:      time.Time{},
			expected: "0001-01-01T00:00:00Z",
		},
		{
			name:     "lock removed",
			current:  "2077-01-01T00:00:00Z",
			set:      time.Time{},
			expected: nil,
		},
		{
			name:     "new lock",
			current:  nil,
			set:      time.Date(2077, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: "2077-01-01T00:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id := New("jane", naming.LocalConnectorID)
			if tt.current != nil {
				id.Object().Object["blockedUntil"] = tt.current
			}
			if err := id.SetBlockedUntil(tt.set); err != nil {
				t.Fatalf("SetBlockedUntil: %v", err)
			}
			if got := id.Object().Object["blockedUntil"]; got != tt.expected {
				t.Errorf("blockedUntil = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIdentity_LockedByAdministrator(t *testing.T) {
	t.Parallel()

	lock := time.Date(2077, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		blocked    time.Time
		annotation *time.Time
		expected   bool
	}{
		{name: "administrator lock", blocked: lock, annotation: &lock, expected: true},
		{name: "automatic lock after an older administrator lock", blocked: lock.Add(time.Hour), annotation: &lock, expected: false},
		{name: "automatic lock", blocked: lock, expected: false},
		{name: "annotation without lock", annotation: &lock, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id := New("jane", "ldap")
			if err := id.SetBlockedUntil(tt.blocked); err != nil {
				t.Fatalf("SetBlockedUntil: %v", err)
			}
			if tt.annotation != nil {
				id.SetAdminLock(*tt.annotation)
			}
			if got := id.LockedByAdministrator(); got != tt.expected {
				t.Errorf("LockedByAdministrator = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRecord_RoundTripAndSources(t *testing.T) {
	t.Parallel()

	id := New("jane", naming.LocalConnectorID)
	if _, ok, err := id.Record(); ok || err != nil {
		t.Fatalf("Record on a new object = (%v, %v), want none", ok, err)
	}

	rec := Record{Time: "2026-10-14T10:00:00Z", TOTPFrom: "uid-1"}
	rec.AddSource(SourcePassword)
	rec.AddSource(SourceOfflineSessions)
	rec.AddSource(SourcePassword)
	if rec.Source != "OfflineSessions,Password" {
		t.Errorf("source = %q, want sorted and without duplicates", rec.Source)
	}
	if err := id.SetRecord(rec); err != nil {
		t.Fatalf("SetRecord: %v", err)
	}

	got, ok, err := id.Record()
	if err != nil || !ok {
		t.Fatalf("Record = (%v, %v)", ok, err)
	}
	if got != rec {
		t.Errorf("record = %+v, want %+v", got, rec)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(id.Object().GetAnnotations()[RecordAnnotation]), &raw); err != nil {
		t.Fatalf("annotation is not JSON: %v", err)
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, []string{"source", "time", "totpFrom"}) {
		t.Errorf("annotation keys = %v", keys)
	}
}

func TestRecord_BrokenAnnotation(t *testing.T) {
	t.Parallel()

	id := New("jane", naming.LocalConnectorID)
	id.Object().SetAnnotations(map[string]string{RecordAnnotation: "true"})
	if _, ok, err := id.Record(); !ok || err == nil {
		t.Fatalf("Record = (%v, %v), want an error for a value that is not a record", ok, err)
	}
}

func TestIdentity_Migrated(t *testing.T) {
	t.Parallel()

	id := New("jane", naming.LocalConnectorID)
	if id.Migrated() {
		t.Fatal("Migrated = true for an object without the annotation")
	}
	// Dex checks only that the annotation is there; so does Migrated, even for a broken value.
	id.Object().SetAnnotations(map[string]string{RecordAnnotation: "true"})
	if !id.Migrated() {
		t.Error("Migrated = false for an object with the annotation")
	}
}

func TestIdentity_IncorrectPasswordLoginAttempts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     any
		expected  int64
		expectErr bool
	}{
		{name: "missing", value: nil, expected: 0},
		{name: "as decoded from json", value: float64(3), expected: 3},
		{name: "integer", value: int64(5), expected: 5},
		{name: "not a number", value: "three", expectErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id := New("alice", "corp-ldap")
			if tt.value != nil {
				id.Object().Object["incorrectPasswordLoginAttempts"] = tt.value
			}
			got, err := id.IncorrectPasswordLoginAttempts()
			if (err != nil) != tt.expectErr {
				t.Fatalf("err = %v, want error %v", err, tt.expectErr)
			}
			if got != tt.expected {
				t.Errorf("attempts = %d, want %d", got, tt.expected)
			}
		})
	}
}

func loadFixture(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	raw, err := os.ReadFile(dexFixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(raw); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return obj
}
