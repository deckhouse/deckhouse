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

package controller

import (
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	toolscache "k8s.io/client-go/tools/cache"

	"user-authn-controller/internal/naming"
)

func TestStripDexSecretsRemovesFieldsWithoutMutatingInput(t *testing.T) {
	t.Parallel()

	original := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":                     "dex.coreos.com/v1",
		"kind":                           "Password",
		"email":                          "jane@example.com",
		"hash":                           "secret-hash",
		"previousHashes":                 []any{"old"},
		"connectorData":                  "secret-connector",
		"refresh":                        map[string]any{"tok": map[string]any{"ID": "1", "token": "secret-refresh"}},
		"totp":                           "secret-totp",
		"token":                          "secret-token",
		"obsoleteToken":                  "secret-obsolete-token",
		"mfaSecrets":                     map[string]any{"totp": map[string]any{"secret": "secret-mfa"}},
		"webauthnCredentials":            map[string]any{"key": []any{"secret-credential"}},
		"secret":                         "secret-session",
		"nonce":                          "secret-nonce",
		"incorrectPasswordLoginAttempts": int64(2),
	}}

	got, err := StripDexSecrets(original)
	if err != nil {
		t.Fatalf("StripDexSecrets: %v", err)
	}
	stripped, ok := got.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("result type %T, want *unstructured.Unstructured", got)
	}

	for _, key := range secretFields {
		if _, exists := stripped.Object[key]; exists {
			t.Errorf("stripped object still has %q", key)
		}
		if _, exists := original.Object[key]; !exists {
			t.Errorf("original object lost %q (must not mutate live object)", key)
		}
	}
	raw, err := json.Marshal(stripped.Object)
	if err != nil {
		t.Fatalf("marshal stripped object: %v", err)
	}
	if strings.Contains(string(raw), "secret-") {
		t.Errorf("stripped object still holds a credential: %s", raw)
	}
	refresh, ok := stripped.Object["refresh"].(map[string]any)
	if !ok {
		t.Fatalf("refresh missing or wrong type: %T", stripped.Object["refresh"])
	}
	entry, ok := refresh["tok"].(map[string]any)
	if !ok {
		t.Fatalf("refresh.tok missing or wrong type: %T", refresh["tok"])
	}
	if entry["ID"] != "1" {
		t.Errorf("refresh.tok.ID = %v, want 1", entry["ID"])
	}
	if _, exists := entry["token"]; exists {
		t.Error("refresh.tok.token was kept")
	}
	origRefresh, ok := original.Object["refresh"].(map[string]any)
	if !ok {
		t.Fatal("original refresh missing")
	}
	origTok, ok := origRefresh["tok"].(map[string]any)
	if !ok {
		t.Fatal("original refresh.tok missing")
	}
	if origTok["token"] != "secret-refresh" {
		t.Errorf("original refresh token mutated: %v", origTok["token"])
	}
	if stripped.Object["email"] != "jane@example.com" {
		t.Errorf("email = %v, want jane@example.com", stripped.Object["email"])
	}
	if original.Object["hash"] != "secret-hash" {
		t.Errorf("original hash mutated: %v", original.Object["hash"])
	}
}

func TestStripDexSecretsDeletedFinalStateUnknown(t *testing.T) {
	t.Parallel()

	inner := &unstructured.Unstructured{Object: map[string]any{
		"hash":  "secret",
		"email": "a@b.c",
	}}
	got, err := StripDexSecrets(toolscache.DeletedFinalStateUnknown{Key: "k", Obj: inner})
	if err != nil {
		t.Fatalf("StripDexSecrets: %v", err)
	}
	dfsu, ok := got.(toolscache.DeletedFinalStateUnknown)
	if !ok {
		t.Fatalf("result type %T", got)
	}
	u, ok := dfsu.Obj.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("inner type %T", dfsu.Obj)
	}
	if _, exists := u.Object["hash"]; exists {
		t.Fatal("deleted object still has hash")
	}
	if _, exists := inner.Object["hash"]; !exists {
		t.Fatal("original deleted object was mutated")
	}
}

func TestInformerCacheByObjectHasTransform(t *testing.T) {
	t.Parallel()

	byObject := InformerCacheByObject()
	if len(byObject) != 6 {
		t.Fatalf("ByObject entries = %d, want 6", len(byObject))
	}
	for obj, cfg := range byObject {
		if cfg.Transform == nil {
			t.Errorf("Transform is nil for %T", obj)
		}
		if _, ok := cfg.Namespaces[naming.DexNamespace]; !ok {
			t.Errorf("missing namespace %q for %T", naming.DexNamespace, obj)
		}
	}
}

func TestStripDexSecretsRemovesUserIdentityAndSessionSecrets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		object   map[string]any
		stripped []string
		kept     []string
	}{
		{
			name: "user identity",
			object: map[string]any{
				"kind":         "UserIdentity",
				"userID":       "jane",
				"connectorID":  "local",
				"claims":       map[string]any{"email": "jane@example.com"},
				"blockedUntil": "2077-01-01T00:00:00Z",
				"mfaSecrets": map[string]any{
					"totp": map[string]any{"secret": "otpauth://totp/x?secret=ABC", "confirmed": true},
				},
				"webauthnCredentials": map[string]any{"key": []any{map[string]any{"publicKey": "AAA"}}},
			},
			kept: []string{"userID", "connectorID", "claims", "blockedUntil", "mfaSecrets", "webauthnCredentials"},
		},
		{
			name: "auth session",
			object: map[string]any{
				"kind":        "AuthSession",
				"userID":      "jane",
				"connectorID": "local",
				"secret":      "cookie-secret",
				"nonce":       "nonce",
			},
			stripped: []string{"secret", "nonce"},
			kept:     []string{"userID", "connectorID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := StripDexSecrets(&unstructured.Unstructured{Object: tt.object})
			if err != nil {
				t.Fatalf("StripDexSecrets: %v", err)
			}
			stripped, ok := got.(*unstructured.Unstructured)
			if !ok {
				t.Fatalf("result type %T, want *unstructured.Unstructured", got)
			}
			for _, key := range tt.stripped {
				if _, exists := stripped.Object[key]; exists {
					t.Errorf("stripped object still has %q", key)
				}
			}
			for _, key := range tt.kept {
				if _, exists := stripped.Object[key]; !exists {
					t.Errorf("stripped object lost %q", key)
				}
			}
		})
	}
}

func TestStripDexSecretsReducesMFASecretsToFlags(t *testing.T) {
	t.Parallel()

	original := &unstructured.Unstructured{Object: map[string]any{
		"kind":   "UserIdentity",
		"userID": "jane",
		"mfaSecrets": map[string]any{
			"totp": map[string]any{
				"authenticatorID": "totp",
				"type":            "TOTP",
				"secret":          "otpauth://totp/x?secret=ABC",
				"confirmed":       true,
				"createdAt":       "2026-10-01T10:00:00Z",
				"lastUsedCounter": int64(59000000),
			},
			"broken": "not an entry",
		},
	}}

	got, err := StripDexSecrets(original)
	if err != nil {
		t.Fatalf("StripDexSecrets: %v", err)
	}
	stripped, ok := got.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("result type %T, want *unstructured.Unstructured", got)
	}
	secrets, ok := stripped.Object["mfaSecrets"].(map[string]any)
	if !ok {
		t.Fatalf("mfaSecrets missing or wrong type: %T", stripped.Object["mfaSecrets"])
	}
	if _, exists := secrets["broken"]; exists {
		t.Error("an entry that is not an object was kept")
	}
	entry, ok := secrets["totp"].(map[string]any)
	if !ok {
		t.Fatalf("mfaSecrets.totp missing or wrong type: %T", secrets["totp"])
	}
	expected := map[string]any{"authenticatorID": "totp", "type": "TOTP", "confirmed": true, "createdAt": "2026-10-01T10:00:00Z"}
	if len(entry) != len(expected) {
		t.Errorf("mfaSecrets.totp = %v, want only %v", entry, expected)
	}
	for key, value := range expected {
		if entry[key] != value {
			t.Errorf("mfaSecrets.totp.%s = %v, want %v", key, entry[key], value)
		}
	}
	origEntry := original.Object["mfaSecrets"].(map[string]any)["totp"].(map[string]any)
	if origEntry["secret"] != "otpauth://totp/x?secret=ABC" {
		t.Errorf("original secret mutated: %v", origEntry["secret"])
	}

	empty := &unstructured.Unstructured{Object: map[string]any{"mfaSecrets": map[string]any{"x": "y"}}}
	got, err = StripDexSecrets(empty)
	if err != nil {
		t.Fatalf("StripDexSecrets: %v", err)
	}
	if _, exists := got.(*unstructured.Unstructured).Object["mfaSecrets"]; exists {
		t.Error("mfaSecrets without an entry was kept")
	}
}

func TestStripDexSecretsReducesWebAuthnCredentialsToACount(t *testing.T) {
	t.Parallel()

	original := &unstructured.Unstructured{Object: map[string]any{
		"kind":   "UserIdentity",
		"userID": "jane",
		"webauthnCredentials": map[string]any{
			"webauthn": []any{
				map[string]any{"id": "secret-id-1", "publicKey": "secret-key-1", "signCount": int64(3)},
				map[string]any{"id": "secret-id-2", "publicKey": "secret-key-2"},
			},
			"empty":  []any{},
			"broken": "not a list",
		},
	}}

	got, err := StripDexSecrets(original)
	if err != nil {
		t.Fatalf("StripDexSecrets: %v", err)
	}
	stripped := got.(*unstructured.Unstructured)
	byAuthenticator, ok := stripped.Object["webauthnCredentials"].(map[string]any)
	if !ok {
		t.Fatalf("webauthnCredentials missing or wrong type: %T", stripped.Object["webauthnCredentials"])
	}
	if len(byAuthenticator) != 1 {
		t.Errorf("webauthnCredentials = %v, want only the authenticator with keys", byAuthenticator)
	}
	list, _ := byAuthenticator["webauthn"].([]any)
	if len(list) != 2 {
		t.Fatalf("webauthnCredentials.webauthn = %v, want two entries", list)
	}
	for _, entry := range list {
		if m, ok := entry.(map[string]any); !ok || len(m) != 0 {
			t.Errorf("an entry still holds %v", entry)
		}
	}
	raw, err := json.Marshal(stripped.Object)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-") {
		t.Errorf("stripped object still holds a credential: %s", raw)
	}
	if len(original.Object["webauthnCredentials"].(map[string]any)["webauthn"].([]any)[0].(map[string]any)) != 3 {
		t.Error("the original was mutated")
	}

	for _, malformed := range []any{"keys", map[string]any{}, map[string]any{"webauthn": []any{}}} {
		got, err := StripDexSecrets(&unstructured.Unstructured{Object: map[string]any{"webauthnCredentials": malformed}})
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := got.(*unstructured.Unstructured).Object["webauthnCredentials"]; exists {
			t.Errorf("webauthnCredentials %v was kept", malformed)
		}
	}
}

func TestSecretFieldsReturnsClone(t *testing.T) {
	t.Parallel()

	got := SecretFields()
	if len(got) == 0 {
		t.Fatal("SecretFields is empty")
	}
	got[0] = "mutated"
	for _, key := range secretFields {
		if key == "mutated" {
			t.Fatal("SecretFields mutation leaked into package slice")
		}
	}
}

func TestStripDexSecretsPassthroughAndMalformedRefresh(t *testing.T) {
	t.Parallel()

	t.Run("nil pointer", func(t *testing.T) {
		t.Parallel()
		var obj *unstructured.Unstructured
		got, err := StripDexSecrets(obj)
		if err != nil {
			t.Fatalf("StripDexSecrets: %v", err)
		}
		u, ok := got.(*unstructured.Unstructured)
		if !ok || u != nil {
			t.Fatalf("got %T %v, want nil pointer", got, got)
		}
	})

	t.Run("value unstructured", func(t *testing.T) {
		t.Parallel()
		original := unstructured.Unstructured{Object: map[string]any{"hash": "secret", "email": "a@b.c"}}
		got, err := StripDexSecrets(original)
		if err != nil {
			t.Fatalf("StripDexSecrets: %v", err)
		}
		stripped, ok := got.(unstructured.Unstructured)
		if !ok {
			t.Fatalf("result type %T", got)
		}
		if _, exists := stripped.Object["hash"]; exists {
			t.Fatal("hash kept")
		}
		if original.Object["hash"] != "secret" {
			t.Fatal("input mutated")
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		t.Parallel()
		got, err := StripDexSecrets("plain")
		if err != nil {
			t.Fatalf("StripDexSecrets: %v", err)
		}
		if got != "plain" {
			t.Errorf("got %v", got)
		}
	})

	t.Run("malformed refresh dropped", func(t *testing.T) {
		t.Parallel()
		obj := &unstructured.Unstructured{Object: map[string]any{
			"refresh": "not-a-map",
		}}
		got, err := StripDexSecrets(obj)
		if err != nil {
			t.Fatalf("StripDexSecrets: %v", err)
		}
		stripped := got.(*unstructured.Unstructured)
		if _, exists := stripped.Object["refresh"]; exists {
			t.Fatal("malformed refresh kept")
		}
	})

	t.Run("refresh lowercase id", func(t *testing.T) {
		t.Parallel()
		obj := &unstructured.Unstructured{Object: map[string]any{
			"refresh": map[string]any{"tok": map[string]any{"id": "abc", "token": "secret"}},
		}}
		got, err := StripDexSecrets(obj)
		if err != nil {
			t.Fatalf("StripDexSecrets: %v", err)
		}
		stripped := got.(*unstructured.Unstructured)
		refresh := stripped.Object["refresh"].(map[string]any)
		entry := refresh["tok"].(map[string]any)
		if entry["ID"] != "abc" {
			t.Errorf("ID = %v, want abc", entry["ID"])
		}
		if _, exists := entry["token"]; exists {
			t.Fatal("token kept")
		}
	})

	t.Run("grants keep the device of each grant", func(t *testing.T) {
		t.Parallel()
		device := map[string]any{
			"ID":        "g1",
			"ClientID":  "console",
			"SessionID": "sid-1",
			"CreatedAt": "2026-10-16T10:00:00Z",
			"LastUsed":  "2026-10-16T11:00:00Z",
			"IPAddress": "203.0.113.7",
			"UserAgent": "Mozilla/5.0",
		}
		entry := maps.Clone(device)
		entry["Unknown"] = "dropped"
		obj := &unstructured.Unstructured{Object: map[string]any{
			"grants":  map[string]any{"g1": entry},
			"refresh": map[string]any{"console": maps.Clone(entry)},
		}}
		got, err := StripDexSecrets(obj)
		if err != nil {
			t.Fatalf("StripDexSecrets: %v", err)
		}
		stripped := got.(*unstructured.Unstructured)
		for _, path := range [][]string{{"grants", "g1"}, {"refresh", "console"}} {
			kept, _, err := unstructured.NestedMap(stripped.Object, path...)
			if err != nil {
				t.Fatalf("%v: %v", path, err)
			}
			if !reflect.DeepEqual(kept, device) {
				t.Errorf("%v = %v, want %v", path, kept, device)
			}
		}
	})

	t.Run("refresh without ids dropped", func(t *testing.T) {
		t.Parallel()
		obj := &unstructured.Unstructured{Object: map[string]any{
			"refresh": map[string]any{"tok": "nope", "other": map[string]any{"token": "x"}},
		}}
		got, err := StripDexSecrets(obj)
		if err != nil {
			t.Fatalf("StripDexSecrets: %v", err)
		}
		stripped := got.(*unstructured.Unstructured)
		if _, exists := stripped.Object["refresh"]; exists {
			t.Fatal("empty reduced refresh kept")
		}
	})
}

// managedFields are half of a Dex object in the cache and nothing reads them from it. They go as nil,
// not as an empty list: an object written back with an empty list resets its managed fields on the API
// server, one written back without them keeps them.
func TestStripDexSecretsDropsManagedFields(t *testing.T) {
	t.Parallel()

	original := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "dex.coreos.com/v1",
		"kind":       "Password",
		"metadata": map[string]any{
			"name":          "pw",
			"namespace":     naming.DexNamespace,
			"labels":        map[string]any{"a": "b"},
			"managedFields": []any{map[string]any{"manager": "dex", "operation": "Update"}},
		},
		"email": "jane@example.com",
		"hash":  "secret-hash",
	}}
	for name, input := range map[string]any{
		"pointer": original,
		"value":   *original.DeepCopy(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := StripDexSecrets(input)
			if err != nil {
				t.Fatalf("StripDexSecrets: %v", err)
			}
			var stripped unstructured.Unstructured
			switch typed := got.(type) {
			case *unstructured.Unstructured:
				stripped = *typed
			case unstructured.Unstructured:
				stripped = typed
			default:
				t.Fatalf("result type %T", got)
			}
			metadata, ok := stripped.Object["metadata"].(map[string]any)
			if !ok {
				t.Fatalf("metadata = %T", stripped.Object["metadata"])
			}
			if _, exists := metadata["managedFields"]; exists {
				t.Errorf("metadata still has managedFields: %v", metadata["managedFields"])
			}
			if stripped.GetName() != "pw" || stripped.GetLabels()["a"] != "b" || stripped.Object["email"] != "jane@example.com" {
				t.Errorf("stripped object lost a field it keeps: %v", stripped.Object)
			}
		})
	}
	if len(original.GetManagedFields()) != 1 {
		t.Errorf("original managedFields mutated: %v", original.GetManagedFields())
	}
}

// fullRefreshToken is a RefreshToken as Dex writes it.
func fullRefreshToken() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "dex.coreos.com/v1",
		"kind":       "RefreshToken",
		"metadata": map[string]any{
			"name":              "rt-1",
			"namespace":         naming.DexNamespace,
			"uid":               "uid-1",
			"resourceVersion":   "42",
			"creationTimestamp": "2026-11-16T10:00:00Z",
			"labels":            map[string]any{"l": "v"},
			"managedFields":     []any{map[string]any{"manager": "Go-http-client", "operation": "Update"}},
		},
		"id":            "rt-1",
		"clientID":      "console",
		"connectorID":   "ldap",
		"scopes":        []any{"openid", "groups", "offline_access"},
		"nonce":         "secret-nonce",
		"token":         "secret-token",
		"obsoleteToken": "secret-obsolete",
		"connectorData": "secret-connector-data",
		"claims": map[string]any{
			"userID":        "u-1",
			"username":      "jane",
			"email":         "jane@example.com",
			"emailVerified": true,
			"groups":        []any{"admins"},
		},
		"CreatedAt":  "2026-11-16T10:00:00Z",
		"LastUsed":   "2026-11-16T11:00:00Z",
		"generation": int64(3),
		"ipAddress":  "192.0.2.10",
		"userAgent":  "Mozilla/5.0",
		"deviceHash": "d-1",
	}}
}

// The cache of RefreshToken keeps only what the controller reads from it: the pair index
// (claims.userID, connectorID), the client and connector of a token, its issue and last use, the
// generation and the metadata. A field missing here reads as empty everywhere, so the list is pinned.
func TestReduceRefreshToken(t *testing.T) {
	t.Parallel()

	original := fullRefreshToken()
	before := original.DeepCopy()

	got, err := ReduceRefreshToken(original)
	if err != nil {
		t.Fatalf("ReduceRefreshToken: %v", err)
	}
	reduced, ok := got.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("result type %T", got)
	}
	if !reflect.DeepEqual(original, before) {
		t.Errorf("input mutated:\n got %v\nwant %v", original.Object, before.Object)
	}

	keys := slices.Sorted(maps.Keys(reduced.Object))
	want := []string{"CreatedAt", "LastUsed", "apiVersion", "claims", "clientID", "connectorID", "generation", "kind", "metadata"}
	if !slices.Equal(keys, want) {
		t.Errorf("kept fields = %v, want %v", keys, want)
	}
	if reduced.GetManagedFields() != nil {
		t.Errorf("managedFields = %v, want none", reduced.GetManagedFields())
	}
	wantMeta := before.DeepCopy()
	wantMeta.SetManagedFields(nil)
	if !reflect.DeepEqual(reduced.Object["metadata"], wantMeta.Object["metadata"]) {
		t.Errorf("metadata = %v, want %v", reduced.Object["metadata"], wantMeta.Object["metadata"])
	}
	if !reflect.DeepEqual(reduced.Object["claims"], before.Object["claims"]) {
		t.Errorf("claims = %v, want %v", reduced.Object["claims"], before.Object["claims"])
	}
	if pair, wantPair := RefreshTokenPair(reduced), RefreshTokenPair(before); !slices.Equal(pair, wantPair) || len(pair) != 1 {
		t.Errorf("pair index = %v, want %v", pair, wantPair)
	}
	raw, err := json.Marshal(reduced.Object)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "secret-") {
		t.Errorf("reduced token holds a credential: %s", raw)
	}

	// The cached object shares nothing with the input: changing the input leaves it as it was.
	claims, ok := original.Object["claims"].(map[string]any)
	if !ok {
		t.Fatal("original claims missing")
	}
	claims["userID"] = "changed"
	if pair := RefreshTokenPair(reduced); len(pair) != 1 || pair[0] != RefreshTokenPair(before)[0] {
		t.Errorf("reduced token follows a change of the input: %v", pair)
	}
}

func TestReduceRefreshTokenOtherShapes(t *testing.T) {
	t.Parallel()

	t.Run("value", func(t *testing.T) {
		t.Parallel()
		got, err := ReduceRefreshToken(*fullRefreshToken())
		if err != nil {
			t.Fatalf("ReduceRefreshToken: %v", err)
		}
		reduced, ok := got.(unstructured.Unstructured)
		if !ok {
			t.Fatalf("result type %T", got)
		}
		if _, exists := reduced.Object["token"]; exists {
			t.Error("token kept")
		}
	})
	t.Run("deleted final state unknown", func(t *testing.T) {
		t.Parallel()
		got, err := ReduceRefreshToken(toolscache.DeletedFinalStateUnknown{Key: "k", Obj: fullRefreshToken()})
		if err != nil {
			t.Fatalf("ReduceRefreshToken: %v", err)
		}
		dfsu, ok := got.(toolscache.DeletedFinalStateUnknown)
		if !ok {
			t.Fatalf("result type %T", got)
		}
		inner, ok := dfsu.Obj.(*unstructured.Unstructured)
		if !ok {
			t.Fatalf("inner type %T", dfsu.Obj)
		}
		if _, exists := inner.Object["token"]; exists {
			t.Error("token kept in the tombstone")
		}
	})
	t.Run("not unstructured", func(t *testing.T) {
		t.Parallel()
		got, err := ReduceRefreshToken("plain")
		if err != nil || got != "plain" {
			t.Errorf("ReduceRefreshToken(plain) = %v, %v", got, err)
		}
	})
	t.Run("nil pointer", func(t *testing.T) {
		t.Parallel()
		var nilToken *unstructured.Unstructured
		got, err := ReduceRefreshToken(nilToken)
		if err != nil || got != any(nilToken) {
			t.Errorf("ReduceRefreshToken(nil) = %v, %v", got, err)
		}
	})
}

func TestInformerCacheByObjectReducesRefreshTokens(t *testing.T) {
	t.Parallel()

	for obj, cfg := range InformerCacheByObject() {
		if obj.GetObjectKind().GroupVersionKind() != RefreshTokenGVK {
			continue
		}
		got, err := cfg.Transform(fullRefreshToken())
		if err != nil {
			t.Fatalf("transform: %v", err)
		}
		reduced, ok := got.(*unstructured.Unstructured)
		if !ok {
			t.Fatalf("result type %T", got)
		}
		if _, exists := reduced.Object["scopes"]; exists {
			t.Error("the RefreshToken cache keeps the whole token")
		}
		return
	}
	t.Fatal("no cache config for RefreshToken")
}
