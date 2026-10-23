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

func TestDecodePasswordAndSessionNil(t *testing.T) {
	t.Parallel()

	if _, err := decodePassword(nil); err == nil {
		t.Fatal("nil password: want error")
	}
	if _, err := decodeSession(nil); err == nil {
		t.Fatal("nil session: want error")
	}
}

func TestDecodeOperationLogout(t *testing.T) {
	t.Parallel()

	op := decodeOperation(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "op-1", "uid": "0b7e7f0e"},
		"spec": map[string]any{
			"type":          "Logout",
			"initiatorType": "self",
			"target":        map[string]any{"email": "jane@example.org", "connectorID": "corp-ldap", "userID": "cn=jane"},
			"logout":        map[string]any{"session": "sid-a"},
		},
	}})
	if op.DecodeErr != nil {
		t.Fatalf("DecodeErr = %v", op.DecodeErr)
	}
	if op.UID != "0b7e7f0e" || op.Spec.Type != typeLogout {
		t.Errorf("operation = %+v", op)
	}
	if op.Spec.Target == nil || op.Spec.Target.UserID != "cn=jane" || op.Spec.Target.ConnectorID != "corp-ldap" {
		t.Errorf("target = %+v", op.Spec.Target)
	}
	if op.Spec.Logout == nil || op.Spec.Logout.Session != "sid-a" || op.Spec.Logout.ExceptSession != "" {
		t.Errorf("logout = %+v", op.Spec.Logout)
	}
}

func TestDecodeSessionFields(t *testing.T) {
	t.Parallel()

	sess, err := decodeSession(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "sess", "namespace": "d8-user-authn"},
		"userID":   "jane",
		"connID":   "ldap",
		"email":    "jane@example.com",
	}})
	if err != nil {
		t.Fatalf("decodeSession: %v", err)
	}
	if sess.UserID != "jane" || sess.ConnID != "ldap" || sess.Email != "jane@example.com" {
		t.Errorf("session = %+v", sess)
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
