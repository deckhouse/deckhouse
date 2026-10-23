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

package revoke

import (
	"maps"
	"slices"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

// knownDevicesObjects are the objects of jane with three browsers known: the laptop of session
// sid-laptop, the phone of session sid-phone, and an old browser with no sign-in now.
func knownDevicesObjects() []client.Object {
	identity := identityObj(naming.LocalConnectorID, "jane", 2)
	identity.Object["knownDevices"] = []any{
		map[string]any{"hash": "laptop", "firstSeen": "2026-10-01T09:00:00Z", "lastSeen": "2026-10-16T09:00:00Z"},
		map[string]any{"hash": "phone", "firstSeen": "2026-10-02T09:00:00Z", "lastSeen": "2026-10-16T09:00:00Z"},
		map[string]any{"hash": "old", "firstSeen": "2026-09-01T09:00:00Z", "lastSeen": "2026-09-02T09:00:00Z"},
	}
	laptop := sessionObj("sid-laptop", naming.LocalConnectorID, "jane", 2, "console")
	laptop.Object["deviceHash"] = "laptop"
	phone := sessionObj("sid-phone", naming.LocalConnectorID, "jane", 2, "console")
	phone.Object["deviceHash"] = "phone"
	offline := offlineObj(naming.LocalConnectorID, "jane", map[string][2]string{"console": {"rt-phone", "sid-phone"}})
	offline.Object["grants"] = map[string]any{
		"rt-laptop": map[string]any{"ID": "rt-laptop", "ClientID": "console", "SessionID": "sid-laptop", "DeviceHash": "laptop"},
		"rt-phone":  map[string]any{"ID": "rt-phone", "ClientID": "console", "SessionID": "sid-phone", "DeviceHash": "phone"},
	}
	return []client.Object{
		identity,
		offline,
		tokenObj("rt-laptop", naming.LocalConnectorID, "jane", "console", 2),
		tokenObj("rt-phone", naming.LocalConnectorID, "jane", "console", 2),
		laptop,
		phone,
	}
}

// withGrant adds a grant of jane to objs: a refresh token of clientID issued in the browser hash,
// under the session sid ("" for none), and its reference in the OfflineSessions.
func withGrant(objs []client.Object, id, clientID, sid, hash string) []client.Object {
	for _, obj := range objs {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok || u.GetKind() != controller.OfflineSessionsGVK.Kind {
			continue
		}
		grants, _ := u.Object["grants"].(map[string]any)
		grants[id] = map[string]any{"ID": id, "ClientID": clientID, "SessionID": sid, "DeviceHash": hash}
	}
	return append(objs, tokenObj(id, naming.LocalConnectorID, "jane", clientID, 2))
}

func knownHashes(t *testing.T, c client.Client) []string {
	t.Helper()
	obj := controller.Object(controller.UserIdentityGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: useridentity.Name("jane", naming.LocalConnectorID)}, obj); err != nil {
		t.Fatalf("get useridentity: %v", err)
	}
	return slices.Sorted(maps.Keys(useridentity.FromUnstructured(obj).KnownDevices()))
}

// A device ended by someone else is forgotten: its browser signs in as a new device next time. The
// browser of the device that asked, and the browsers with no sign-in ended, stay known.
func TestLogoutForgetsTheEndedDevices(t *testing.T) {
	t.Parallel()

	t.Run("one session", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, knownDevicesObjects()...)
		if _, err := newTestRevoker(c).LogoutSession(t.Context(), jane, "sid-laptop", "op-1"); err != nil {
			t.Fatalf("LogoutSession: %v", err)
		}
		if got := knownHashes(t, c); !slices.Equal(got, []string{"old", "phone"}) {
			t.Errorf("known devices = %v, want old and phone", got)
		}
	})

	t.Run("every session but the phone", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, knownDevicesObjects()...)
		if _, err := newTestRevoker(c).LogoutExceptSession(t.Context(), jane, "sid-phone", "op-1"); err != nil {
			t.Fatalf("LogoutExceptSession: %v", err)
		}
		// The old browser has no sign-in now, but it is one of the other devices too: a thief who signed
		// in and out before must not come back as a known device.
		if got := knownHashes(t, c); !slices.Equal(got, []string{"phone"}) {
			t.Errorf("known devices = %v, want only phone", got)
		}
	})

	t.Run("every grant but the laptop", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, knownDevicesObjects()...)
		if _, err := newTestRevoker(c).LogoutExceptGrant(t.Context(), jane, "rt-laptop", "op-1"); err != nil {
			t.Fatalf("LogoutExceptGrant: %v", err)
		}
		if got := knownHashes(t, c); !slices.Equal(got, []string{"laptop"}) {
			t.Errorf("known devices = %v, want only laptop", got)
		}
	})

	t.Run("one grant of a browser still signed in", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, knownDevicesObjects()...)
		if _, err := newTestRevoker(c).LogoutGrant(t.Context(), jane, "rt-laptop", "op-1"); err != nil {
			t.Fatalf("LogoutGrant: %v", err)
		}
		// The laptop keeps its browser session: the browser is forgotten with its last sign-in.
		if got := knownHashes(t, c); len(got) != 3 {
			t.Errorf("known devices = %v, want all three", got)
		}
	})

	t.Run("an old grant of the browser the user works in", func(t *testing.T) {
		t.Parallel()
		// A console token left from an earlier session of the laptop, which has signed in again since.
		objs := withGrant(knownDevicesObjects(), "rt-laptop-old", "console", "sid-gone", "laptop")
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, objs...)
		if _, err := newTestRevoker(c).LogoutGrant(t.Context(), jane, "rt-laptop-old", "op-1"); err != nil {
			t.Fatalf("LogoutGrant: %v", err)
		}
		if got := knownHashes(t, c); len(got) != 3 {
			t.Errorf("known devices = %v, want all three", got)
		}
	})

	t.Run("the last grant of a browser", func(t *testing.T) {
		t.Parallel()
		objs := withGrant(knownDevicesObjects(), "rt-old", "kubeconfig", "", "old")
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, objs...)
		if _, err := newTestRevoker(c).LogoutGrant(t.Context(), jane, "rt-old", "op-1"); err != nil {
			t.Fatalf("LogoutGrant: %v", err)
		}
		if got := knownHashes(t, c); !slices.Equal(got, []string{"laptop", "phone"}) {
			t.Errorf("known devices = %v, want laptop and phone", got)
		}
	})

	t.Run("an expired session does not hold its browser", func(t *testing.T) {
		t.Parallel()
		objs := knownDevicesObjects()
		for _, obj := range objs {
			if u, ok := obj.(*unstructured.Unstructured); ok && u.GetKind() == controller.AuthSessionGVK.Kind && u.GetName() == "sid-phone" {
				// Dex has not deleted the session yet, but it ended an hour before the operation.
				u.Object["idleExpiry"] = revokeTime.Add(-time.Hour).Format(time.RFC3339)
			}
		}
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, objs...)
		if _, err := newTestRevoker(c).LogoutGrant(t.Context(), jane, "rt-phone", "op-1"); err != nil {
			t.Fatalf("LogoutGrant: %v", err)
		}
		if got := knownHashes(t, c); !slices.Equal(got, []string{"laptop", "old"}) {
			t.Errorf("known devices = %v, want laptop and old", got)
		}
	})

	t.Run("the pair forgets every browser", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, knownDevicesObjects()...)
		if _, err := newTestRevoker(c).LogoutPair(t.Context(), jane, revocation.ReasonLogout, "op-1"); err != nil {
			t.Fatalf("LogoutPair: %v", err)
		}
		if got := knownHashes(t, c); len(got) != 0 {
			t.Errorf("known devices = %v, want none", got)
		}
	})
}
