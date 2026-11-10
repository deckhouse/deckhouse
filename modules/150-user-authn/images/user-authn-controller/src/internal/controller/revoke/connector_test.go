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
	"encoding/json"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const corp = "corp-ldap"

// corpObjects is a provider whose two users hold tokens, next to a user of another provider and a local user.
func corpObjects() []client.Object {
	return []client.Object{
		offlineObj(corp, "ann", map[string][2]string{"console": {"rt-ann-console", ""}, "kubeconfig-generator": {"rt-ann-kube", ""}}),
		tokenObj("rt-ann-console", corp, "ann", "console", 0),
		tokenObj("rt-ann-kube", corp, "ann", "kubeconfig-generator", 0),
		offlineObj(corp, "bob", map[string][2]string{"console": {"rt-bob-console", ""}}),
		tokenObj("rt-bob-console", corp, "bob", "console", 0),
		offlineObj("other", "cid", map[string][2]string{"console": {"rt-cid-console", ""}}),
		tokenObj("rt-cid-console", "other", "cid", "console", 0),
		offlineObj(naming.LocalConnectorID, "jane", map[string][2]string{"console": {"rt-jane-console", ""}}),
		tokenObj("rt-jane-console", naming.LocalConnectorID, "jane", "console", 0),
		sessionObj("sid-ann", corp, "ann", 0, "console"),
		identityObj(corp, "ann", 3),
	}
}

func TestDeleteConnectorTokens_DeletesTheTokensOfTheConnectorOnly(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, corpObjects()...)
	deleted, err := newTestRevoker(c).DeleteConnectorTokens(t.Context(), corp, time.Time{})
	if err != nil {
		t.Fatalf("DeleteConnectorTokens: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted %d tokens, want 3", deleted)
	}
	for _, name := range []string{"rt-ann-console", "rt-ann-kube", "rt-bob-console"} {
		if exists(t, c, controller.RefreshTokenGVK, name) {
			t.Errorf("the token %s of the connector is still there", name)
		}
	}
	for _, name := range []string{"rt-cid-console", "rt-jane-console"} {
		if !exists(t, c, controller.RefreshTokenGVK, name) {
			t.Errorf("the token %s of another connector was deleted", name)
		}
	}
}

func TestDeleteConnectorTokens_RemovesTheReferences(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, corpObjects()...)
	if _, err := newTestRevoker(c).DeleteConnectorTokens(t.Context(), corp, time.Time{}); err != nil {
		t.Fatalf("DeleteConnectorTokens: %v", err)
	}
	for _, user := range []string{"ann", "bob"} {
		if refresh := refreshOfObj(t, c, corp, user); len(refresh) != 0 {
			t.Errorf("the OfflineSessions of %s still refer to %v", user, refresh)
		}
	}
	if refresh := refreshOfObj(t, c, "other", "cid"); len(refresh) != 1 {
		t.Errorf("the OfflineSessions of the other connector lost its reference: %v", refresh)
	}
}

// The browser sessions and the generation of the pair stay: they hold nothing that depends on the prefix.
func TestDeleteConnectorTokens_LeavesTheSessionsAndTheIdentity(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, corpObjects()...)
	if _, err := newTestRevoker(c).DeleteConnectorTokens(t.Context(), corp, time.Time{}); err != nil {
		t.Fatalf("DeleteConnectorTokens: %v", err)
	}
	if !exists(t, c, controller.AuthSessionGVK, "sid-ann") {
		t.Error("the browser session was deleted")
	}
	if !exists(t, c, controller.UserIdentityGVK, naming.OfflineTokenName("ann", corp)) {
		t.Error("the user identity was deleted")
	}
}

func TestDeleteConnectorTokens_CanBeRepeated(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, corpObjects()...)
	r := newTestRevoker(c)
	if _, err := r.DeleteConnectorTokens(t.Context(), corp, time.Time{}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	deleted, err := r.DeleteConnectorTokens(t.Context(), corp, time.Time{})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if deleted != 0 {
		t.Errorf("the second run deleted %d tokens", deleted)
	}
}

func TestDeleteConnectorTokens_NoTokens(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{})
	deleted, err := newTestRevoker(c).DeleteConnectorTokens(t.Context(), corp, time.Time{})
	if err != nil || deleted != 0 {
		t.Fatalf("deleted %d, error %v", deleted, err)
	}
}

// issuedAt is a refresh token that says when it was issued, in the field Dex writes it in: CreatedAt, with the
// capital letter, since the Go struct of the kubernetes storage has no json tag on it.
func issuedAt(obj *unstructured.Unstructured, at time.Time) *unstructured.Unstructured {
	obj.Object["CreatedAt"] = at.Format(time.RFC3339Nano)
	return obj
}

// realTokenJSON is a RefreshToken as the API server serves it: the json of the kubernetes storage type of Dex.
const realTokenJSON = `{
  "apiVersion": "dex.coreos.com/v1",
  "kind": "RefreshToken",
  "metadata": {"name": "rt-real", "namespace": "d8-user-authn", "resourceVersion": "9"},
  "CreatedAt": "2026-11-04T10:07:00.123456789Z",
  "LastUsed": "2026-11-04T10:07:00.123456789Z",
  "clientID": "console",
  "scopes": ["openid", "offline_access"],
  "claims": {"userID": "ann", "username": "ann", "preferredUsername": "", "email": "ann@example.com", "emailVerified": true},
  "connectorID": "corp-ldap",
  "generation": 1
}`

// The cut-off goes by the time in the object as the API server serves it.
func TestDeleteConnectorTokens_CreatedBeforeReadsTheFieldDexWrites(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(realTokenJSON), &obj.Object); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		before     time.Time
		wantDelete bool
	}{
		{name: "issued before the cut-off", before: time.Date(2026, 11, 4, 10, 10, 0, 0, time.UTC), wantDelete: true},
		{name: "issued after the cut-off", before: time.Date(2026, 11, 4, 10, 5, 0, 0, time.UTC), wantDelete: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := newTestClient(t, &calls{}, interceptor.Funcs{}, obj.DeepCopy())
			deleted, err := newTestRevoker(c).DeleteConnectorTokens(t.Context(), "corp-ldap", tc.before)
			if err != nil {
				t.Fatal(err)
			}
			if (deleted == 1) != tc.wantDelete || exists(t, c, controller.RefreshTokenGVK, "rt-real") == tc.wantDelete {
				t.Fatalf("deleted %d, want delete=%v", deleted, tc.wantDelete)
			}
		})
	}
}

// The second pass over a connector spares the tokens issued after the change: they carry what the connector gives now.
func TestDeleteConnectorTokens_CreatedBefore(t *testing.T) {
	t.Parallel()

	changed := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	c := newTestClient(t, &calls{}, interceptor.Funcs{},
		offlineObj(corp, "ann", map[string][2]string{"console": {"rt-old", ""}}),
		issuedAt(tokenObj("rt-old", corp, "ann", "console", 0), changed.Add(-time.Hour)),
		issuedAt(tokenObj("rt-in-flight", corp, "bob", "console", 0), changed.Add(time.Minute)),
		issuedAt(tokenObj("rt-new", corp, "cid", "console", 0), changed.Add(10*time.Minute)),
		tokenObj("rt-undated", corp, "dan", "console", 0),
	)
	deleted, err := newTestRevoker(c).DeleteConnectorTokens(t.Context(), corp, changed.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("DeleteConnectorTokens: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted %d tokens, want 3", deleted)
	}
	for _, name := range []string{"rt-old", "rt-in-flight", "rt-undated"} {
		if exists(t, c, controller.RefreshTokenGVK, name) {
			t.Errorf("the token %s issued before the cut-off is still there", name)
		}
	}
	if !exists(t, c, controller.RefreshTokenGVK, "rt-new") {
		t.Error("the token issued after the cut-off was deleted")
	}
	if refresh := refreshOfObj(t, c, corp, "ann"); len(refresh) != 0 {
		t.Errorf("the OfflineSessions of ann still refer to %v", refresh)
	}
}
