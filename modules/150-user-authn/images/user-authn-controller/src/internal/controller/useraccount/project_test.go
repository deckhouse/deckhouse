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
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

func TestProjectLocalAttemptsLockEmail(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	until := now.Add(time.Hour)
	pw := passwordView{
		Email:       "jane@example.com",
		Username:    "jane",
		UserID:      "jane",
		Attempts:    3,
		LockedUntil: &until,
		Groups:      []string{"from-password"},
	}
	user := &userView{
		Name:   "jane",
		UID:    types.UID("uid-jane"),
		Email:  "Jane@Example.com",
		Groups: []string{"admins"},
	}

	got := projectLocal(pw, nil, user, now)
	if got.Status.Email != "jane@example.com" {
		t.Errorf("email = %q", got.Status.Email)
	}
	if got.Status.IncorrectLoginAttempts != 3 {
		t.Errorf("attempts = %d, want 3", got.Status.IncorrectLoginAttempts)
	}
	if !got.Status.Locked {
		t.Error("locked = false, want true")
	}
	if got.Labels[v1alpha1.LabelLocked] != "true" {
		t.Errorf("locked label = %q, want true", got.Labels[v1alpha1.LabelLocked])
	}
	if got.Status.Kind != v1alpha1.KindLocal || got.Status.ConnectorID != naming.LocalConnectorID {
		t.Errorf("kind/connector = %q/%q", got.Status.Kind, got.Status.ConnectorID)
	}
	if got.Status.ProviderType != "" {
		t.Errorf("providerType = %q, want empty", got.Status.ProviderType)
	}
	if got.Status.UserRef != "jane" {
		t.Errorf("userRef = %q", got.Status.UserRef)
	}
	if len(got.Status.Groups) != 1 || got.Status.Groups[0] != "admins" {
		t.Errorf("groups = %v, want [admins] from User", got.Status.Groups)
	}
	if got.Owner == nil || got.Owner.Name != "jane" || got.Owner.UID != "uid-jane" {
		t.Errorf("owner = %+v", got.Owner)
	}
	if got.Owner != nil && (got.Owner.Controller == nil || !*got.Owner.Controller) {
		t.Error("owner.controller want true")
	}
	if got.Owner != nil && (got.Owner.BlockOwnerDeletion == nil || *got.Owner.BlockOwnerDeletion) {
		t.Error("owner.blockOwnerDeletion want false")
	}

	raw, err := json.Marshal(got.Status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	assertNoSecretKeys(t, raw)
}

func TestProjectLocalExpiredLockedUntil(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Minute)

	tests := []struct {
		name        string
		annotations map[string]string
		wantLocked  bool
		wantAdmin   bool
		wantLabel   string
	}{
		{
			name:       "expired without admin annotation",
			wantLocked: false,
			wantLabel:  "false",
		},
		{
			name:        "expired with admin annotation",
			annotations: map[string]string{lockedByAdministratorAnnot: ""},
			wantLocked:  false,
			wantAdmin:   false,
			wantLabel:   "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pw := passwordView{
				Email:       "jane@example.com",
				Username:    "jane",
				LockedUntil: &expired,
				Annotations: tt.annotations,
			}
			got := projectLocal(pw, nil, nil, now)
			if got.Status.Locked != tt.wantLocked {
				t.Errorf("locked = %v, want %v", got.Status.Locked, tt.wantLocked)
			}
			if got.Status.LockedByAdministrator != tt.wantAdmin {
				t.Errorf("lockedByAdministrator = %v, want %v", got.Status.LockedByAdministrator, tt.wantAdmin)
			}
			if got.Labels[v1alpha1.LabelLocked] != tt.wantLabel {
				t.Errorf("locked label = %q, want %q", got.Labels[v1alpha1.LabelLocked], tt.wantLabel)
			}
		})
	}
}

func TestDecodePasswordOmitsSecrets(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":                     "dex.coreos.com/v1",
		"kind":                           "Password",
		"metadata":                       map[string]any{"name": "pw", "namespace": naming.DexNamespace},
		"email":                          "jane@example.com",
		"username":                       "jane",
		"hash":                           "secret-hash",
		"previousHashes":                 []any{"old"},
		"incorrectPasswordLoginAttempts": int64(4),
	}}

	pw, err := decodePassword(obj)
	if err != nil {
		t.Fatalf("decodePassword: %v", err)
	}
	if pw.Attempts != 4 {
		t.Errorf("attempts = %d, want 4", pw.Attempts)
	}

	raw, err := json.Marshal(pw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	assertNoSecretKeys(t, raw)
}

func TestDecodeSessionOmitsSecrets(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":                     "dex.coreos.com/v1",
		"kind":                           "OfflineSessions",
		"metadata":                       map[string]any{"name": "sess", "namespace": naming.DexNamespace},
		"email":                          "alice@example.com",
		"userID":                         "u1",
		"connID":                         "ldap-main",
		"connectorData":                  "secret",
		"refresh":                        map[string]any{"a": map[string]any{"ID": "1"}},
		"totp":                           "secret-totp",
		"incorrectPasswordLoginAttempts": int64(1),
	}}

	sess, err := decodeSession(obj)
	if err != nil {
		t.Fatalf("decodeSession: %v", err)
	}
	if sess.Attempts != 1 || sess.ConnID != "ldap-main" {
		t.Errorf("session = %+v", sess)
	}
	raw, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	assertNoSecretKeys(t, raw)
}

func TestDecodeUserOmitsPassword(t *testing.T) {
	t.Parallel()

	obj := userUnstructured("jane", "jane@example.com", []string{"admins"}, "2026-12-01T00:00:00Z")
	user, err := decodeUser(obj)
	if err != nil {
		t.Fatalf("decodeUser: %v", err)
	}
	raw, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	assertNoSecretKeys(t, raw)
	if _, ok := collectJSONKeys(mustJSON(t, raw))["password"]; ok {
		t.Fatalf("decoded user JSON contains password: %s", raw)
	}
	if user.Email != "jane@example.com" || user.Name != "jane" {
		t.Errorf("user = %+v", user)
	}
}

func mustJSON(t *testing.T, raw []byte) any {
	t.Helper()
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return decoded
}

func assertNoSecretKeys(t *testing.T, raw []byte) {
	t.Helper()
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	keys := collectJSONKeys(decoded)
	for _, key := range controller.SecretFields() {
		if _, ok := keys[key]; ok {
			t.Errorf("JSON contains forbidden key %q: %s", key, raw)
		}
	}
}

func collectJSONKeys(v any) map[string]struct{} {
	keys := make(map[string]struct{})
	collectJSONKeysInto(v, keys)
	return keys
}

func collectJSONKeysInto(v any, keys map[string]struct{}) {
	switch typed := v.(type) {
	case map[string]any:
		for key, child := range typed {
			keys[key] = struct{}{}
			collectJSONKeysInto(child, keys)
		}
	case []any:
		for _, child := range typed {
			collectJSONKeysInto(child, keys)
		}
	}
}

func TestProjectSessions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 16, 12, 30, 0, 0, time.UTC)
	at := func(d time.Duration) time.Time { return now.Add(d) }
	hour := func(h int) *metav1.Time { return metaTimeOrNil(time.Date(2026, 10, 16, h, 0, 0, 0, time.UTC)) }
	browser := browserView{
		ID:           "sid-1",
		Clients:      []string{"console"},
		IPAddress:    "203.0.113.7",
		UserAgent:    "Firefox",
		CreatedAt:    at(-5 * time.Hour),
		LastActivity: at(-3 * time.Hour),
		Expiry:       at(time.Hour),
	}

	tests := []struct {
		name     string
		browsers []browserView
		grants   []grantView
		expected []v1alpha1.UserAccountSession
	}{
		{
			name:     "the grants of a live browser session are its applications",
			browsers: []browserView{browser},
			grants: []grantView{
				{ID: "g-console", ClientID: "console", SessionID: "sid-1", IPAddress: "198.51.100.1", LastUsed: at(-2 * time.Hour)},
				{ID: "g-grafana", ClientID: "grafana", SessionID: "sid-1", LastUsed: at(-90 * time.Minute)},
			},
			expected: []v1alpha1.UserAccountSession{{
				ID:           "sid-1",
				Kind:         v1alpha1.SessionKindBrowser,
				Clients:      []string{"console", "grafana"},
				IPAddress:    "203.0.113.7",
				UserAgent:    "Firefox",
				SignedInAt:   metaTimeOrNil(at(-5 * time.Hour)),
				LastActivity: hour(11), // grafana, 11:00, rounded down
			}},
		},
		{
			name:     "an ended browser session leaves its grants as devices",
			browsers: []browserView{{ID: "sid-1", Expiry: at(-time.Minute), LastActivity: at(-time.Hour)}},
			grants: []grantView{{
				ID: "g-console", ClientID: "console", SessionID: "sid-1", UserAgent: "Firefox",
				CreatedAt: at(-5 * time.Hour), LastUsed: at(-10 * time.Minute),
			}},
			expected: []v1alpha1.UserAccountSession{{
				ID:           "g-console",
				Kind:         v1alpha1.SessionKindGrant,
				Clients:      []string{"console"},
				UserAgent:    "Firefox",
				SignedInAt:   metaTimeOrNil(at(-5 * time.Hour)),
				LastActivity: hour(12),
			}},
		},
		{
			name: "a device signed in within the hour is active from its sign-in, not from the hour before",
			browsers: []browserView{{
				ID: "sid-2", Clients: []string{"console"}, CreatedAt: at(-11 * time.Minute),
				LastActivity: at(-5 * time.Minute), Expiry: at(time.Hour),
			}},
			expected: []v1alpha1.UserAccountSession{{
				ID:           "sid-2",
				Kind:         v1alpha1.SessionKindBrowser,
				Clients:      []string{"console"},
				SignedInAt:   metaTimeOrNil(at(-11 * time.Minute)),
				LastActivity: metaTimeOrNil(at(-11 * time.Minute)), // 12:19, not 12:00
			}},
		},
		{
			name:     "grants without a browser come apart, the most recently active first",
			browsers: []browserView{browser},
			grants: []grantView{
				{ID: "g-old", ClientID: "kubeconfig-generator", CreatedAt: at(-48 * time.Hour), LastUsed: at(-24 * time.Hour)},
				{ID: "g-new", ClientID: "kubeconfig-generator", CreatedAt: at(-time.Hour)},
			},
			expected: []v1alpha1.UserAccountSession{
				{
					ID:           "g-new",
					Kind:         v1alpha1.SessionKindGrant,
					Clients:      []string{"kubeconfig-generator"},
					SignedInAt:   metaTimeOrNil(at(-time.Hour)),
					LastActivity: metaTimeOrNil(at(-time.Hour)), // 11:30: the hour rounds down to before the sign-in
				},
				{
					ID:           "sid-1",
					Kind:         v1alpha1.SessionKindBrowser,
					Clients:      []string{"console"},
					IPAddress:    "203.0.113.7",
					UserAgent:    "Firefox",
					SignedInAt:   metaTimeOrNil(at(-5 * time.Hour)),
					LastActivity: hour(9),
				},
				{
					ID:           "g-old",
					Kind:         v1alpha1.SessionKindGrant,
					Clients:      []string{"kubeconfig-generator"},
					SignedInAt:   metaTimeOrNil(at(-48 * time.Hour)),
					LastActivity: metaTimeOrNil(time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)),
				},
			},
		},
		{
			name: "nothing signed in",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, truncated := projectSessions(tt.browsers, tt.grants, nil, DefaultEstablishedDeviceAge, now)
			if truncated {
				t.Error("truncated = true, want false")
			}
			if !reflect.DeepEqual(got, tt.expected) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				wantJSON, _ := json.MarshalIndent(tt.expected, "", "  ")
				t.Errorf("sessions =\n%s\nwant\n%s", gotJSON, wantJSON)
			}
		})
	}
}

func TestProjectSessionsKeepsTheMostRecent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	grants := make([]grantView, 0, v1alpha1.MaxSessions+5)
	for i := range v1alpha1.MaxSessions + 5 {
		grants = append(grants, grantView{
			ID:       fmt.Sprintf("g-%03d", i),
			ClientID: "kubeconfig-generator",
			LastUsed: now.Add(-time.Duration(i) * time.Hour),
		})
	}

	got, truncated := projectSessions(nil, grants, nil, DefaultEstablishedDeviceAge, now)
	if !truncated {
		t.Error("truncated = false, want true")
	}
	if len(got) != v1alpha1.MaxSessions {
		t.Fatalf("len = %d, want %d", len(got), v1alpha1.MaxSessions)
	}
	if last := fmt.Sprintf("g-%03d", v1alpha1.MaxSessions-1); got[0].ID != "g-000" || got[len(got)-1].ID != last {
		t.Errorf("kept %s..%s, want g-000..%s", got[0].ID, got[len(got)-1].ID, last)
	}
}

func TestProjectSessionsOrderHoldsWithinAnHour(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 16, 12, 50, 0, 0, time.UTC)
	at := func(m int) time.Time { return time.Date(2026, 10, 16, 12, m, 0, 0, time.UTC) }
	project := func(consoleUsed, kubeUsed time.Time) []v1alpha1.UserAccountSession {
		got, _ := projectSessions(nil, []grantView{
			{ID: "g-kube", ClientID: "kubeconfig-generator", LastUsed: kubeUsed},
			{ID: "g-console", ClientID: "console", LastUsed: consoleUsed},
		}, nil, DefaultEstablishedDeviceAge, now)
		return got
	}
	// Two devices taking turns within one hour leave the status as it was.
	first := project(at(10), at(20))
	second := project(at(40), at(30))
	if !reflect.DeepEqual(first, second) {
		t.Errorf("order changed within the hour:\n%+v\n%+v", first, second)
	}
	if first[0].ID != "g-console" {
		t.Errorf("first = %s, want g-console (the IDs order devices of one hour)", first[0].ID)
	}
}

func TestProjectSessionsKeepsEstablishedDevices(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	grants := []grantView{{
		ID: "g-owner", ClientID: "console", CreatedAt: now.Add(-48 * time.Hour), LastUsed: now.Add(-40 * time.Hour),
	}}
	for i := range v1alpha1.MaxSessions + 5 {
		grants = append(grants, grantView{
			ID: fmt.Sprintf("g-new-%03d", i), ClientID: "console", CreatedAt: now.Add(-time.Hour), LastUsed: now,
		})
	}

	got, truncated := projectSessions(nil, grants, nil, DefaultEstablishedDeviceAge, now)
	if !truncated || len(got) != v1alpha1.MaxSessions {
		t.Fatalf("len = %d, truncated = %t; want %d, true", len(got), truncated, v1alpha1.MaxSessions)
	}
	if !slices.ContainsFunc(got, func(s v1alpha1.UserAccountSession) bool { return s.ID == "g-owner" }) {
		t.Error("the device signed in two days ago was pushed off the list by new ones")
	}
	if got[len(got)-1].ID != "g-owner" {
		t.Errorf("last = %s, want g-owner: the kept list is ordered by activity", got[len(got)-1].ID)
	}

	// With a minimum device age of three days, the owner's device is as new as the others, and the
	// least recently active goes first.
	got, _ = projectSessions(nil, grants, nil, 72*time.Hour, now)
	if slices.ContainsFunc(got, func(s v1alpha1.UserAccountSession) bool { return s.ID == "g-owner" }) {
		t.Error("the device signed in two days ago was kept under a minimum age of three days")
	}
}

// A browser that completed a login before its current sign-in is known since its first login; a
// browser seen first at this sign-in, or one Dex no longer knows, is as old as the sign-in.
func TestProjectSessionsKnownSince(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	firstSeen := now.Add(-72 * time.Hour)
	// "new" is the first login of its browser: remembered at the first factor, a moment before the session.
	known := map[string]time.Time{"known": firstSeen, "new": now.Add(-time.Hour - 20*time.Second)}
	got, _ := projectSessions(
		[]browserView{
			{ID: "sid-known", DeviceHash: "known", CreatedAt: now.Add(-time.Hour), Expiry: now.Add(time.Hour)},
			{ID: "sid-new", DeviceHash: "new", CreatedAt: now.Add(-time.Hour), Expiry: now.Add(time.Hour)},
			{ID: "sid-forgotten", DeviceHash: "forgotten", CreatedAt: now.Add(-time.Hour), Expiry: now.Add(time.Hour)},
		},
		[]grantView{{ID: "g-known", ClientID: "kubeconfig-generator", DeviceHash: "known", CreatedAt: now.Add(-2 * time.Hour)}},
		known, DefaultEstablishedDeviceAge, now)
	since := map[string]*metav1.Time{}
	for _, s := range got {
		since[s.ID] = s.KnownSince
	}
	if since["sid-known"] == nil || !since["sid-known"].Time.Equal(firstSeen) {
		t.Errorf("sid-known knownSince = %v, want %v", since["sid-known"], firstSeen)
	}
	if since["g-known"] == nil || !since["g-known"].Time.Equal(firstSeen) {
		t.Errorf("g-known knownSince = %v, want %v", since["g-known"], firstSeen)
	}
	for _, id := range []string{"sid-new", "sid-forgotten"} {
		if since[id] != nil {
			t.Errorf("%s knownSince = %v, want none", id, since[id])
		}
	}
}

// A device whose browser has been known for a day is kept when the list is cut, even if it signed in
// again moments ago.
func TestProjectSessionsKeepsKnownDevices(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	grants := []grantView{{
		ID: "g-owner", ClientID: "console", DeviceHash: "owner", CreatedAt: now.Add(-time.Hour), LastUsed: now.Add(-40 * time.Hour),
	}}
	for i := range v1alpha1.MaxSessions + 5 {
		grants = append(grants, grantView{
			ID: fmt.Sprintf("g-new-%03d", i), ClientID: "console", CreatedAt: now.Add(-time.Hour), LastUsed: now,
		})
	}
	got, _ := projectSessions(nil, grants, map[string]time.Time{"owner": now.Add(-48 * time.Hour)}, DefaultEstablishedDeviceAge, now)
	if !slices.ContainsFunc(got, func(s v1alpha1.UserAccountSession) bool { return s.ID == "g-owner" }) {
		t.Error("the device of a browser known for two days was pushed off the list by new ones")
	}
}

func TestProjectSessionsBoundsWhatADeviceSends(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	// A two-byte rune crosses the bound: the cut goes before it.
	agent := strings.Repeat("a", maxUserAgent-1) + "é" + strings.Repeat("b", 100)
	got, _ := projectSessions(nil, []grantView{{ID: "g", UserAgent: agent, IPAddress: strings.Repeat("1", 100)}}, nil, DefaultEstablishedDeviceAge, now)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if want := strings.Repeat("a", maxUserAgent-1); got[0].UserAgent != want {
		t.Errorf("userAgent has %d bytes, want the %d before the cut rune", len(got[0].UserAgent), len(want))
	}
	if !utf8.ValidString(got[0].UserAgent) {
		t.Error("userAgent is not valid UTF-8")
	}
	if len(got[0].IPAddress) != maxIPAddress {
		t.Errorf("ipAddress has %d bytes, want %d", len(got[0].IPAddress), maxIPAddress)
	}
}

func TestDecodeGrants(t *testing.T) {
	t.Parallel()

	got := decodeGrants(
		map[string]refTokenFields{
			"g2": {ID: "g2", ClientID: "console", SessionID: "sid-1", IPAddress: "203.0.113.7", CreatedAt: "2026-10-16T10:00:00.123456789Z"},
		},
		map[string]refTokenFields{
			// The last grant of console, already in grants.
			"console": {ID: "g2", ClientID: "console"},
			// Written by an old Dex: not in grants, the client named only by the key.
			"kubeconfig-generator": {ID: "g1", LastUsed: "0001-01-01T00:00:00Z"},
		},
	)
	want := []grantView{
		{ID: "g1", ClientID: "kubeconfig-generator"},
		{
			ID: "g2", ClientID: "console", SessionID: "sid-1", IPAddress: "203.0.113.7",
			CreatedAt: time.Date(2026, 10, 16, 10, 0, 0, 123456789, time.UTC),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("grants = %+v, want %+v", got, want)
	}
}
