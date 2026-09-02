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
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDecodeOperationMissingSpec(t *testing.T) {
	t.Parallel()

	op := decodeOperation(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "op-1"},
	}})
	if op.DecodeErr == nil {
		t.Fatal("missing spec: want DecodeErr")
	}
	if op.Name != "op-1" {
		t.Errorf("name = %q", op.Name)
	}
}

func TestDecodePasswordAndRefreshNil(t *testing.T) {
	t.Parallel()

	if _, err := decodePassword(nil); err == nil {
		t.Fatal("nil password: want error")
	}
	if _, err := decodeSession(nil); err == nil {
		t.Fatal("nil session: want error")
	}
	if _, err := decodeRefreshToken(nil); err == nil {
		t.Fatal("nil refresh token: want error")
	}
}

func TestDecodeSessionRefreshIDs(t *testing.T) {
	t.Parallel()

	sess, err := decodeSession(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "sess", "namespace": "d8-user-authn"},
		"userID":   "jane",
		"connID":   "ldap",
		"email":    "jane@example.com",
		"refresh": map[string]any{
			"tok":   map[string]any{"ID": "id-upper", "token": "secret"},
			"tok2":  map[string]any{"id": "id-lower"},
			"bad":   "skip-me",
			"empty": map[string]any{"token": "x"},
		},
	}})
	if err != nil {
		t.Fatalf("decodeSession: %v", err)
	}
	if sess.UserID != "jane" || sess.ConnID != "ldap" {
		t.Errorf("session = %+v", sess)
	}
	want := map[string]struct{}{"id-upper": {}, "id-lower": {}}
	if len(sess.RefreshTokenIDs) != 2 {
		t.Fatalf("refresh ids = %v, want 2", sess.RefreshTokenIDs)
	}
	for _, id := range sess.RefreshTokenIDs {
		if _, ok := want[id]; !ok {
			t.Errorf("unexpected id %q", id)
		}
	}
}

func TestDecodeRefreshTokenClaims(t *testing.T) {
	t.Parallel()

	rt, err := decodeRefreshToken(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "rt"},
		"claims": map[string]any{
			"userId":             "uid",
			"username":           "jane",
			"preferred_username": "jane-pref",
		},
	}})
	if err != nil {
		t.Fatalf("decodeRefreshToken: %v", err)
	}
	if rt.ClaimsUserID != "uid" || rt.ClaimsUsername != "jane" || rt.ClaimsPreferred != "jane-pref" {
		t.Errorf("claims = %+v", rt)
	}
}

func TestOperationLogKV(t *testing.T) {
	t.Parallel()

	op := operation{
		Name:        "op",
		Namespace:   "d8-user-authn",
		Annotations: map[string]string{initiatorAnnot: "admin"},
		Spec: operationSpec{
			Type:          typeLock,
			InitiatorType: "User",
			User:          "jane",
			Target:        &operationTarget{ConnectorID: "ldap", Email: "ext@example.com"},
		},
	}
	kv := operationLogKV(op)
	if len(kv)%2 != 0 {
		t.Fatalf("odd kv length %d: %v", len(kv), kv)
	}
	got := map[string]any{}
	for i := 0; i < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			t.Fatalf("key %d is %T", i, kv[i])
		}
		got[key] = kv[i+1]
	}
	if got["initiator"] != "admin" {
		t.Errorf("initiator = %v", got["initiator"])
	}
	if got["targetUser"] != "jane" {
		t.Errorf("targetUser = %v", got["targetUser"])
	}
	if got["targetEmail"] != "ext@example.com" {
		t.Errorf("targetEmail = %v", got["targetEmail"])
	}
}
