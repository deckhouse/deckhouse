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

package migration

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	// legacyID is the old id of DexAuthenticator billing in prod-team; currentID the one it has now.
	legacyID  = "billing-prod-team-dex-authenticator"
	currentID = "dex-authenticator-billing@prod-team"
)

func TestLegacyDexAuthenticatorClientID(t *testing.T) {
	if got := LegacyDexAuthenticatorClientID("billing", "prod-team"); got != legacyID {
		t.Fatalf("LegacyDexAuthenticatorClientID = %q, want %q", got, legacyID)
	}
}

func TestDexAuthenticatorTokensDeletesWhatDexKeepsForLegacyIDs(t *testing.T) {
	c := newTestClient(t,
		dexAuthenticator("billing", "prod-team"),
		oauth2Client("current", currentID),
		token("t-legacy", legacyID),
		token("t-current", currentID),
		token("t-console", "console"),
		offlineSessions("pair-a",
			map[string]any{legacyID: ref("t-legacy", ""), "console": ref("t-console", "")},
			map[string]any{"t-legacy": ref("t-legacy", legacyID), "t-legacy-older": ref("t-legacy-older", legacyID), "t-console": ref("t-console", "console")},
		),
		authSession("sid-1", legacyID, "console"),
	)
	m := NewDexAuthenticatorTokens(c, logr.Discard(), time.Millisecond)

	pending, err := m.Run(t.Context())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending = %v, want none", pending)
	}

	assertGone(t, c, controller.RefreshTokenGVK, "t-legacy")
	assertExists(t, c, controller.RefreshTokenGVK, "t-current")
	assertExists(t, c, controller.RefreshTokenGVK, "t-console")
	assertKeys(t, c, controller.OfflineSessionsGVK, "pair-a", "refresh", "console")
	assertKeys(t, c, controller.OfflineSessionsGVK, "pair-a", "grants", "t-console")
	assertKeys(t, c, controller.AuthSessionGVK, "sid-1", "clientStates", "console")

	// A second pass finds nothing left and changes nothing.
	if pending, err := m.Run(t.Context()); err != nil || len(pending) != 0 {
		t.Fatalf("second Run = %v, %v", pending, err)
	}
	assertKeys(t, c, controller.OfflineSessionsGVK, "pair-a", "refresh", "console")
}

func TestDexAuthenticatorTokensWaitsForTheLegacyOAuth2Client(t *testing.T) {
	c := newTestClient(t,
		dexAuthenticator("billing", "prod-team"),
		oauth2Client("legacy", legacyID),
		token("t-legacy", legacyID),
		offlineSessions("pair-a", map[string]any{legacyID: ref("t-legacy", "")}, nil),
	)
	m := NewDexAuthenticatorTokens(c, logr.Discard(), time.Millisecond)

	pending, err := m.Run(t.Context())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(pending) != 1 || pending[0] != legacyID {
		t.Fatalf("pending = %v, want [%s]", pending, legacyID)
	}
	assertExists(t, c, controller.RefreshTokenGVK, "t-legacy")
	assertKeys(t, c, controller.OfflineSessionsGVK, "pair-a", "refresh", legacyID)

	// Helm prunes the old client; the next pass deletes the token and its reference.
	if err := c.Delete(t.Context(), oauth2Client("legacy", legacyID)); err != nil {
		t.Fatalf("delete legacy client: %v", err)
	}
	if pending, err := m.Run(t.Context()); err != nil || len(pending) != 0 {
		t.Fatalf("Run after prune = %v, %v", pending, err)
	}
	assertGone(t, c, controller.RefreshTokenGVK, "t-legacy")
	assertKeys(t, c, controller.OfflineSessionsGVK, "pair-a", "refresh")
}

// A controller that stopped after deleting the tokens left their references behind; the next start
// removes them.
func TestDexAuthenticatorTokensFinishesAfterARestart(t *testing.T) {
	c := newTestClient(t,
		dexAuthenticator("billing", "prod-team"),
		offlineSessions("pair-a",
			map[string]any{legacyID: ref("t-gone", "")},
			map[string]any{"t-gone": ref("t-gone", legacyID)},
		),
	)
	m := NewDexAuthenticatorTokens(c, logr.Discard(), time.Millisecond)
	if _, err := m.Run(t.Context()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertKeys(t, c, controller.OfflineSessionsGVK, "pair-a", "refresh")
	assertKeys(t, c, controller.OfflineSessionsGVK, "pair-a", "grants")
}

func TestDexAuthenticatorTokensStartReturnsWhenDone(t *testing.T) {
	c := newTestClient(t, dexAuthenticator("billing", "prod-team"), token("t-legacy", legacyID))
	m := NewDexAuthenticatorTokens(c, logr.Discard(), time.Millisecond)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("Start returned only when the context ended")
	}
	assertGone(t, c, controller.RefreshTokenGVK, "t-legacy")
	if !m.NeedLeaderElection() {
		t.Fatal("the migration must run on the leader only")
	}
}

// A legacy id can belong to another client for good: DexAuthenticator kubeconfig in foo has the
// legacy id of the kubeconfig cluster foo-dex-authenticator. The migration cleans the ids that are
// free once, leaves that one alone, and keeps waiting without listing the Dex objects again.
func TestDexAuthenticatorTokensLeavesAnIDAnotherClientHas(t *testing.T) {
	const taken = "kubeconfig-foo-dex-authenticator"
	var scans int
	c := newTestClientWith(t, interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			switch list.GetObjectKind().GroupVersionKind().Kind {
			case "RefreshTokenList", "OfflineSessionsList", "AuthSessionList":
				scans++
			}
			return c.List(ctx, list, opts...)
		},
	},
		dexAuthenticator("billing", "prod-team"),
		dexAuthenticator("kubeconfig", "foo"),
		oauth2Client("kubeconfig-cluster", taken),
		token("t-legacy", legacyID),
		token("t-kubeconfig", taken),
	)
	m := NewDexAuthenticatorTokens(c, logr.Discard(), time.Millisecond)

	pending, err := m.Run(t.Context())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(pending) != 1 || pending[0] != taken {
		t.Fatalf("pending = %v, want [%s]", pending, taken)
	}
	assertGone(t, c, controller.RefreshTokenGVK, "t-legacy")
	assertExists(t, c, controller.RefreshTokenGVK, "t-kubeconfig")
	firstPass := scans
	if firstPass == 0 {
		t.Fatal("the first pass must list the Dex objects")
	}

	// Start keeps waiting while the id is taken; the leader stopping ends it.
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if ctx.Err() == nil {
		t.Fatal("Start must keep waiting while an OAuth2Client has the legacy id")
	}
	if scans != firstPass {
		t.Fatalf("the Dex objects were listed %d more times while nothing new was free", scans-firstPass)
	}
	assertExists(t, c, controller.RefreshTokenGVK, "t-kubeconfig")
}

func newTestClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return newTestClientWith(t, interceptor.Funcs{}, objs...)
}

func newTestClientWith(t *testing.T, funcs interceptor.Funcs, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(controller.DexAuthenticatorGVK, &metav1.PartialObjectMetadata{})
	scheme.AddKnownTypeWithName(controller.DexAuthenticatorGVK.GroupVersion().WithKind(controller.DexAuthenticatorGVK.Kind+"List"), &metav1.PartialObjectMetadataList{})
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "deckhouse.io", Version: "v1"}, {Group: "dex.coreos.com", Version: "v1"}})
	for _, gvk := range []schema.GroupVersionKind{
		controller.DexAuthenticatorGVK,
		controller.OAuth2ClientGVK,
		controller.RefreshTokenGVK,
		controller.OfflineSessionsGVK,
		controller.AuthSessionGVK,
	} {
		mapper.Add(gvk, meta.RESTScopeNamespace)
	}
	builder := fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(mapper).WithObjects(objs...).WithInterceptorFuncs(funcs)
	for _, index := range controller.Indexes() {
		builder = builder.WithIndex(index.Object, index.Field, index.Extract)
	}
	return builder.Build()
}

func dexAuthenticator(name, namespace string) *metav1.PartialObjectMetadata {
	a := &metav1.PartialObjectMetadata{}
	a.SetGroupVersionKind(controller.DexAuthenticatorGVK)
	a.SetName(name)
	a.SetNamespace(namespace)
	return a
}

func dexObject(gvk schema.GroupVersionKind, name string, fields map[string]any) *unstructured.Unstructured {
	obj := controller.Object(gvk)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(name)
	for k, v := range fields {
		obj.Object[k] = v
	}
	return obj
}

func oauth2Client(name, id string) *unstructured.Unstructured {
	return dexObject(controller.OAuth2ClientGVK, name, map[string]any{"id": id})
}

func token(name, clientID string) *unstructured.Unstructured {
	return dexObject(controller.RefreshTokenGVK, name, map[string]any{
		"clientID":    clientID,
		"connectorID": "crowd",
		"claims":      map[string]any{"userID": "jane", "username": "jane", "email": "jane@example.com"},
	})
}

func ref(id, clientID string) map[string]any {
	r := map[string]any{"ID": id}
	if clientID != "" {
		r["ClientID"] = clientID
	}
	return r
}

func offlineSessions(name string, refresh, grants map[string]any) *unstructured.Unstructured {
	fields := map[string]any{"userID": "jane", "connID": "crowd", "refresh": refresh}
	if grants != nil {
		fields["grants"] = grants
	}
	return dexObject(controller.OfflineSessionsGVK, name, fields)
}

func authSession(sid string, clients ...string) *unstructured.Unstructured {
	states := map[string]any{}
	for _, c := range clients {
		states[c] = map[string]any{"AuthenticatedAt": "2026-10-23T09:00:00Z"}
	}
	return dexObject(controller.AuthSessionGVK, sid, map[string]any{"userID": "jane", "connectorID": "crowd", "clientStates": states})
}

func get(t *testing.T, c client.Client, gvk schema.GroupVersionKind, name string) (*unstructured.Unstructured, error) {
	t.Helper()
	obj := controller.Object(gvk)
	err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj)
	return obj, err
}

func assertGone(t *testing.T, c client.Client, gvk schema.GroupVersionKind, name string) {
	t.Helper()
	if _, err := get(t, c, gvk, name); !apierrors.IsNotFound(err) {
		t.Fatalf("%s %s: want NotFound, got %v", gvk.Kind, name, err)
	}
}

func assertExists(t *testing.T, c client.Client, gvk schema.GroupVersionKind, name string) {
	t.Helper()
	if _, err := get(t, c, gvk, name); err != nil {
		t.Fatalf("%s %s: %v", gvk.Kind, name, err)
	}
}

func assertKeys(t *testing.T, c client.Client, gvk schema.GroupVersionKind, name, field string, want ...string) {
	t.Helper()
	obj, err := get(t, c, gvk, name)
	if err != nil {
		t.Fatalf("%s %s: %v", gvk.Kind, name, err)
	}
	m, _, _ := unstructured.NestedMap(obj.Object, field)
	got := sortedKeys(m)
	if len(got) != len(want) {
		t.Fatalf("%s %s %s = %v, want %v", gvk.Kind, name, field, got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("%s %s %s = %v, want %v", gvk.Kind, name, field, got, want)
		}
	}
}
