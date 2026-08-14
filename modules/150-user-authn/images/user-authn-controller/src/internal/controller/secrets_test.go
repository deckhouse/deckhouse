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
	if len(byObject) != 3 {
		t.Fatalf("ByObject entries = %d, want 3", len(byObject))
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
