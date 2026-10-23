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

package user

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

var revokeNow = time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)

func refreshTokenObj(name, connID, userID, clientID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":  controller.RefreshTokenGVK.GroupVersion().String(),
		"kind":        controller.RefreshTokenGVK.Kind,
		"metadata":    map[string]any{"name": name, "namespace": naming.DexNamespace},
		"clientID":    clientID,
		"connectorID": connID,
		"claims":      map[string]any{"userID": userID},
	}}
}

func authSessionObj(sid, connID, userID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":   controller.AuthSessionGVK.GroupVersion().String(),
		"kind":         controller.AuthSessionGVK.Kind,
		"metadata":     map[string]any{"name": sid, "namespace": naming.DexNamespace},
		"userID":       userID,
		"connectorID":  connID,
		"clientStates": map[string]any{"console": map[string]any{}},
	}}
}

// signInObj is an AuthCode or an AuthRequest of (userID, connID) that Dex stamped with generation.
func signInObj(gvk schema.GroupVersionKind, name, connID, userID string, generation int64) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":  gvk.GroupVersion().String(),
		"kind":        gvk.Kind,
		"metadata":    map[string]any{"name": name, "namespace": naming.DexNamespace},
		"clientID":    "console",
		"connectorID": connID,
		"claims":      map[string]any{"userID": userID, "username": userID},
	}}
	if generation > 0 {
		obj.Object["generation"] = generation
	}
	return obj
}

func pairRevocationObj(t *testing.T, userID string, generation uint64) *unstructured.Unstructured {
	t.Helper()
	pair := naming.OfflineTokenName(userID, naming.LocalConnectorID)
	obj, err := revocation.Revocation{
		Name:        revocation.PairName(pair, generation),
		Type:        revocation.TypePair,
		ConnectorID: naming.LocalConnectorID,
		UserID:      userID,
		Subject:     revocation.Subject(userID, naming.LocalConnectorID),
		Generation:  generation,
		Reason:      revocation.ReasonUserDeleted,
		Source:      revocation.SourceController,
		CreatedAt:   revokeNow.Add(-time.Minute),
	}.Object()
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

func listRevocationNames(t *testing.T, c client.Reader) []string {
	t.Helper()
	list := controller.List(controller.RevocationGVK)
	if err := c.List(t.Context(), list); err != nil {
		t.Fatal(err)
	}
	var names []string
	for i := range list.Items {
		names = append(names, list.Items[i].GetName())
	}
	slices.Sort(names)
	return names
}

// A deleted User: the pair is revoked first, then its OfflineSessions and UserIdentity go. A user of
// another provider with the same ID is another pair (A1).
func TestReconcile_DeletedUserIsRevoked(t *testing.T) {
	t.Parallel()

	ghostPair := useridentity.Name("ghost", naming.LocalConnectorID)
	identity := localIdentityObj("ghost")
	useridentity.FromUnstructured(identity).SetGeneration(2)
	var mu sync.Mutex
	var calls []string
	record := func(verb string, obj client.Object) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, verb+" "+obj.GetObjectKind().GroupVersionKind().Kind+" "+obj.GetName())
	}
	c := newFakeClientBuilder(t,
		namespaceObj(),
		identity,
		dexSessionObj(ghostPair, "ghost", naming.LocalConnectorID),
		refreshTokenObj("rt-ghost", naming.LocalConnectorID, "ghost", "console"),
		authSessionObj("sid-ghost", naming.LocalConnectorID, "ghost"),
		localIdentityObjFor("ghost", "corp-ldap"),
		refreshTokenObj("rt-ldap", "corp-ldap", "ghost", "console"),
		authSessionObj("sid-ldap", "corp-ldap", "ghost"),
	).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			record("create", obj)
			return cl.Create(ctx, obj, opts...)
		},
		Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			record("update", obj)
			return cl.Update(ctx, obj, opts...)
		},
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			record("delete", obj)
			return cl.Delete(ctx, obj, opts...)
		},
	}).Build()
	r := New(c, c, logr.Discard(), func() time.Time { return revokeNow })

	if _, err := r.Reconcile(t.Context(), userReq("ghost")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	expected := []string{
		"update UserIdentity " + ghostPair,
		"create Revocation " + revocation.PairName(ghostPair, 3),
		"create Revocation " + revocation.SessionName("sid-ghost"),
		"delete AuthSession sid-ghost",
		"delete RefreshToken rt-ghost",
		"delete OfflineSessions " + ghostPair,
		"delete UserIdentity " + ghostPair,
	}
	mu.Lock()
	got := slices.Clone(calls)
	mu.Unlock()
	if !slices.Equal(got, expected) {
		t.Errorf("calls:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}

	rev, err := revocation.FromObject(getDexObject(t, r, controller.RevocationGVK, revocation.PairName(ghostPair, 3)))
	if err != nil {
		t.Fatal(err)
	}
	if rev.Reason != revocation.ReasonUserDeleted || rev.Generation != 3 || !slices.Equal(rev.Clients, []string{"console"}) {
		t.Errorf("revocation = %+v", rev)
	}
	assertKept(t, r, controller.UserIdentityGVK, useridentity.Name("ghost", "corp-ldap"))
	assertKept(t, r, controller.RefreshTokenGVK, "rt-ldap")
	assertKept(t, r, controller.AuthSessionGVK, "sid-ldap")
}

// F1: a pair never revoked before is at generation 0. Deleting the User raises it to 1 and then
// deletes the UserIdentity, and Dex reads the missing record as generation 0 again: a code, or a
// finished auth request, issued before the deletion would match it. The revocation deletes them
// before the UserIdentity goes.
func TestReconcile_DeletedUserLosesTheSignInsInProgress(t *testing.T) {
	t.Parallel()

	ghostPair := useridentity.Name("ghost", naming.LocalConnectorID)
	var mu sync.Mutex
	var calls []string
	c := newFakeClientBuilder(t,
		namespaceObj(),
		localIdentityObj("ghost"),
		signInObj(controller.AuthCodeGVK, "code-ghost", naming.LocalConnectorID, "ghost", 0),
		signInObj(controller.AuthRequestGVK, "req-ghost", naming.LocalConnectorID, "ghost", 0),
		signInObj(controller.AuthCodeGVK, "code-ldap", "corp-ldap", "ghost", 0),
	).WithInterceptorFuncs(interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			mu.Lock()
			calls = append(calls, obj.GetObjectKind().GroupVersionKind().Kind+" "+obj.GetName())
			mu.Unlock()
			return cl.Delete(ctx, obj, opts...)
		},
	}).Build()
	r := New(c, c, logr.Discard(), func() time.Time { return revokeNow })

	if _, err := r.Reconcile(t.Context(), userReq("ghost")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	expected := []string{
		"AuthCode code-ghost",
		"AuthRequest req-ghost",
		"UserIdentity " + ghostPair,
	}
	mu.Lock()
	got := slices.Clone(calls)
	mu.Unlock()
	if !slices.Equal(got, expected) {
		t.Errorf("deletions:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}
	if names := listRevocationNames(t, r.client); !slices.Equal(names, []string{revocation.PairName(ghostPair, 1)}) {
		t.Errorf("revocations = %v", names)
	}
	assertKept(t, r, controller.AuthCodeGVK, "code-ldap")

	// The next reconcile finds nothing left and does nothing.
	if _, err := r.Reconcile(t.Context(), userReq("ghost")); err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if names := listRevocationNames(t, r.client); len(names) != 1 {
		t.Errorf("revocations after the second reconcile = %v, want one", names)
	}
}

// A pair that never had a generation (no UserIdentity, tokens of the old Dex) is revoked too:
// its tokens carry no generation, and the Revocation at generation 1 makes the subscribers refuse
// them.
func TestReconcile_DeletedUserWithoutUserIdentity(t *testing.T) {
	t.Parallel()

	ghostPair := useridentity.Name("ghost", naming.LocalConnectorID)
	r := newTestReconciler(t, revokeNow, namespaceObj(), refreshTokenObj("rt-ghost", naming.LocalConnectorID, "ghost", "console"))
	if _, err := r.Reconcile(t.Context(), userReq("ghost")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if names := listRevocationNames(t, r.client); !slices.Equal(names, []string{revocation.PairName(ghostPair, 1)}) {
		t.Errorf("revocations = %v", names)
	}
	assertGone(t, r, controller.RefreshTokenGVK, "rt-ghost")
	assertGone(t, r, controller.UserIdentityGVK, ghostPair)
}

// The reconcile runs again on the changes it made, while the cache still holds the token it has
// deleted. Only what the API server still has counts: a second revocation would create the
// UserIdentity again.
func TestReconcile_DeletedUserIsRevokedOnce(t *testing.T) {
	t.Parallel()

	ghostPair := useridentity.Name("ghost", naming.LocalConnectorID)
	stale := refreshTokenObj("rt-ghost", naming.LocalConnectorID, "ghost", "console")
	api := newFakeClientBuilder(t, namespaceObj(), stale.DeepCopy(), localIdentityObj("ghost")).Build()
	staleCache := interceptor.NewClient(api, interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if err := c.List(ctx, list, opts...); err != nil {
				return err
			}
			u, ok := list.(*unstructured.UnstructuredList)
			if !ok || u.GetKind() != "RefreshTokenList" {
				return nil
			}
			for i := range u.Items {
				if u.Items[i].GetName() == stale.GetName() {
					return nil
				}
			}
			u.Items = append(u.Items, *stale.DeepCopy())
			return nil
		},
	})
	r := New(staleCache, api, logr.Discard(), func() time.Time { return revokeNow })

	for range 2 {
		if _, err := r.Reconcile(t.Context(), userReq("ghost")); err != nil {
			t.Fatalf("Reconcile: %v", err)
		}
	}
	if names := listRevocationNames(t, api); !slices.Equal(names, []string{revocation.PairName(ghostPair, 1)}) {
		t.Errorf("revocations = %v, want one", names)
	}
	assertGone(t, r, controller.UserIdentityGVK, ghostPair)
}

// A deleted User that left nothing in Dex: no revocation, no UserIdentity.
func TestReconcile_DeletedUserWithNothingLeft(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, revokeNow, namespaceObj())
	if _, err := r.Reconcile(t.Context(), userReq("ghost")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if names := listRevocationNames(t, r.client); len(names) != 0 {
		t.Errorf("revocations = %v, want none", names)
	}
	assertGone(t, r, controller.UserIdentityGVK, useridentity.Name("ghost", naming.LocalConnectorID))
}

// A User created with the name of a deleted one starts at the highest revoked generation of the
// pair, and the tokens and sessions the old user left are deleted before the Password exists.
func TestReconcile_NewUserInheritsTheRevokedGeneration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		identity *uint64
		expected uint64
	}{
		{name: "no user identity", expected: 5},
		{name: "a lower generation", identity: new(uint64(2)), expected: 5},
		{name: "a higher generation", identity: new(uint64(7)), expected: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			objs := []client.Object{
				namespaceObj(),
				userObj(adminUserName, adminEmail, adminPassword, ""),
				pairRevocationObj(t, adminUserName, 3),
				pairRevocationObj(t, adminUserName, 5),
				refreshTokenObj("rt-old", naming.LocalConnectorID, adminUserName, "console"),
				authSessionObj("sid-old", naming.LocalConnectorID, adminUserName),
				refreshTokenObj("rt-ldap", "corp-ldap", adminUserName, "console"),
			}
			if tt.identity != nil {
				id := useridentity.New(adminUserName, naming.LocalConnectorID)
				id.SetGeneration(*tt.identity)
				objs = append(objs, id.Object())
			}
			r := newTestReconciler(t, revokeNow, objs...)
			if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}

			id := getLocalIdentity(t, r, adminUserName)
			if g, _ := id.Generation(); g != tt.expected {
				t.Errorf("generation = %d, want %d", g, tt.expected)
			}
			if tt.identity == nil {
				if !id.Migrated() {
					t.Error("the seeded UserIdentity has no record: the transitional Dex would read old places that do not exist")
				}
				if claims, _ := id.Claims(); claims.Email != adminEmail || claims.Username != adminUserName {
					t.Errorf("claims = %+v", claims)
				}
			}
			assertGone(t, r, controller.RefreshTokenGVK, "rt-old")
			assertGone(t, r, controller.AuthSessionGVK, "sid-old")
			assertKept(t, r, controller.RefreshTokenGVK, "rt-ldap")
			if !passwordExists(t, r, adminEncodedPW) {
				t.Error("the Password was not created")
			}
		})
	}
}

// Without stored revocations a new User gets no UserIdentity from the controller.
func TestReconcile_NewUserWithoutRevocations(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, revokeNow, namespaceObj(), userObj(adminUserName, adminEmail, adminPassword, ""))
	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertGone(t, r, controller.UserIdentityGVK, useridentity.Name(adminUserName, naming.LocalConnectorID))
	if !passwordExists(t, r, adminEncodedPW) {
		t.Error("the Password was not created")
	}
}

// When the revocations cannot be read, the Password is not created: a sign-in would start at a
// generation the subscribers refuse, or let the old tokens back in.
func TestReconcile_NewUserFailsClosed(t *testing.T) {
	t.Parallel()

	c := newFakeClientBuilder(t, namespaceObj(), userObj(adminUserName, adminEmail, adminPassword, "")).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if u, ok := list.(*unstructured.UnstructuredList); ok && u.GetKind() == "RevocationList" {
					return apierrors.NewForbidden(schema.GroupResource{Group: "dex.coreos.com", Resource: "revocations"}, "", nil)
				}
				return cl.List(ctx, list, opts...)
			},
		}).Build()
	r := New(c, c, logr.Discard(), func() time.Time { return revokeNow })
	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err == nil {
		t.Fatal("Reconcile succeeded without the revocations")
	}
	if passwordExists(t, r, adminEncodedPW) {
		t.Error("the Password was created")
	}
}

func getDexObject(t *testing.T, r *Reconciler, gvk schema.GroupVersionKind, name string) *unstructured.Unstructured {
	t.Helper()
	obj := controller.Object(gvk)
	if err := r.client.Get(t.Context(), client.ObjectKey{Namespace: naming.DexNamespace, Name: name}, obj); err != nil {
		t.Fatalf("get %s %s: %v", gvk.Kind, name, err)
	}
	return obj
}
