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
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

func TestCopier_Copy(t *testing.T) {
	t.Parallel()

	local := useridentity.Name("jane", naming.LocalConnectorID)

	t.Run("a new object takes the secret and the lock", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil,
			passwordObj("jane", "jane@example.com", passwordOpts{lockedUntil: &testLock}),
			sessionObj("jane", naming.LocalConnectorID, sessionOpts{totp: totpSecretA, totpConfirmed: true}),
		)
		id, err := NewCopier(h.m.client, h.api, logr.Discard()).Copy(t.Context(), local, nil, testNow)
		if err != nil {
			t.Fatalf("Copy: %v", err)
		}
		if id == nil {
			t.Fatal("Copy returned no object")
		}
		if totp, _ := id.TOTP(); totp == nil || totp.Secret != totpSecretA || !totp.Confirmed {
			t.Errorf("totp = %+v", totp)
		}
		if blocked, _ := id.BlockedUntil(); !blocked.Equal(testLock) {
			t.Errorf("blockedUntil = %v, want %v", blocked, testLock)
		}
		if rec, ok, _ := id.Record(); !ok || rec.TOTPFrom == "" || rec.LockFrom == "" {
			t.Errorf("record = %+v, %v; the copy must not take the same data again", rec, ok)
		}
	})

	t.Run("an existing object is filled", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t, nil,
			passwordObj("jane", "jane@example.com", passwordOpts{}),
			sessionObj("jane", naming.LocalConnectorID, sessionOpts{totp: totpSecretA, totpConfirmed: true}),
		)
		current := useridentity.FromUnstructured(identityObj(t, "jane", naming.LocalConnectorID, func(*useridentity.Identity) {}))
		id, err := NewCopier(h.m.client, h.api, logr.Discard()).Copy(t.Context(), local, current, testNow)
		if err != nil {
			t.Fatalf("Copy: %v", err)
		}
		if id != current {
			t.Fatal("Copy did not return the object it was given")
		}
		if totp, _ := id.TOTP(); totp == nil || totp.Secret != totpSecretA {
			t.Errorf("totp = %+v", totp)
		}
	})

	unchanged := []struct {
		name    string
		pair    string
		objs    []client.Object
		current func(t *testing.T) *useridentity.Identity
	}{
		{
			name: "old places hold nothing",
			pair: local,
		},
		{
			name: "local pair of a deleted user",
			pair: local,
			objs: []client.Object{sessionObj("jane", naming.LocalConnectorID, sessionOpts{totp: totpSecretA})},
		},
		{
			name: "object of another pair",
			pair: local,
			objs: []client.Object{
				passwordObj("jane", "jane@example.com", passwordOpts{}),
				sessionObj("jane", naming.LocalConnectorID, sessionOpts{totp: totpSecretA}),
			},
			current: func(t *testing.T) *useridentity.Identity {
				obj := identityObj(t, "bob", naming.LocalConnectorID, func(*useridentity.Identity) {})
				obj.SetName(local)
				return useridentity.FromUnstructured(obj)
			},
		},
	}
	for _, tt := range unchanged {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, nil, tt.objs...)
			var current *useridentity.Identity
			var before map[string]any
			if tt.current != nil {
				current = tt.current(t)
				before = current.Object().DeepCopy().Object
			}
			id, err := NewCopier(h.m.client, h.api, logr.Discard()).Copy(t.Context(), tt.pair, current, testNow)
			if err != nil {
				t.Fatalf("Copy: %v", err)
			}
			if id != current {
				t.Fatalf("Copy returned %v, want the object it was given", id)
			}
			if current != nil && !reflect.DeepEqual(before, current.Object().Object) {
				t.Errorf("Copy changed the object of another pair")
			}
		})
	}
}
