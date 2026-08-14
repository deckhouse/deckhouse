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
	"testing"
	"time"

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

	got := projectLocal(pw, user, now)
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
			wantLocked:  true,
			wantAdmin:   true,
			wantLabel:   "true",
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
			got := projectLocal(pw, nil, now)
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
