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
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

var managedLabels = map[string]string{heritageLabel: heritageValue, moduleLabel: moduleValue, appLabel: appValue}

func TestReconcile_DeletedUserLosesItsDexRecord(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	ghostPair := useridentity.Name("ghost", naming.LocalConnectorID)
	r := newTestReconciler(t, now,
		namespaceObj(),
		passwordObj("managed-ghost", map[string]any{
			"email": "ghost@example.com", "username": "ghost", "userID": "ghost", "hash": "aGFzaA==",
		}, managedLabels, nil),
		localIdentityObj("ghost"),
		dexSessionObj(ghostPair, "ghost", naming.LocalConnectorID),
		// Another provider's user with the same name is not the deleted user.
		localIdentityObjFor("ghost", "corp-ldap"),
		dexSessionObj(useridentity.Name("ghost", "corp-ldap"), "ghost", "corp-ldap"),
	)

	if _, err := r.Reconcile(t.Context(), userReq("ghost")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	assertGone(t, r, controller.UserIdentityGVK, ghostPair)
	assertGone(t, r, controller.OfflineSessionsGVK, ghostPair)
	assertKept(t, r, controller.UserIdentityGVK, useridentity.Name("ghost", "corp-ldap"))
	assertKept(t, r, controller.OfflineSessionsGVK, useridentity.Name("ghost", "corp-ldap"))
}

// The cache index still lists the Password the reconcile has just deleted. The record must go in
// that same reconcile: a User created again right after takes the path that keeps the record.
func TestReconcile_DeletedUserLosesItsDexRecordWithAStaleCache(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	ghostPair := useridentity.Name("ghost", naming.LocalConnectorID)
	password := passwordObj("managed-ghost", map[string]any{
		"email": "ghost@example.com", "username": "ghost", "userID": "ghost", "hash": "aGFzaA==",
	}, managedLabels, nil)
	api := newFakeClientBuilder(t, namespaceObj(), password.DeepCopy(), localIdentityObj("ghost"), dexSessionObj(ghostPair, "ghost", naming.LocalConnectorID)).
		Build()
	var served atomic.Bool
	staleCache := interceptor.NewClient(api, interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			var lo client.ListOptions
			lo.ApplyOptions(opts)
			u, ok := list.(*unstructured.UnstructuredList)
			byPair := ok && lo.FieldSelector != nil && !lo.FieldSelector.Empty()
			if err := c.List(ctx, list, opts...); err != nil || !byPair || served.Swap(true) {
				return err
			}
			for i := range u.Items {
				if u.Items[i].GetName() == password.GetName() {
					return nil
				}
			}
			u.Items = append(u.Items, *password.DeepCopy())
			return nil
		},
	})
	r := New(staleCache, api, logr.Discard(), func() time.Time { return now })

	if _, err := r.Reconcile(t.Context(), userReq("ghost")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	assertGone(t, r, controller.PasswordGVK, password.GetName())
	assertGone(t, r, controller.UserIdentityGVK, ghostPair)
	assertGone(t, r, controller.OfflineSessionsGVK, ghostPair)

	// ghost is created again.
	if err := api.Create(t.Context(), userObj("ghost", "ghost@example.com", adminPassword, "")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(t.Context(), userReq("ghost")); err != nil {
		t.Fatalf("Reconcile after the re-create: %v", err)
	}

	// The new ghost starts at the generation of the revocation of the old one, without its record.
	id := getLocalIdentity(t, r, "ghost")
	if g, _ := id.Generation(); g != 1 {
		t.Errorf("generation of the new user = %d, want 1", g)
	}
	if totp, _ := id.TOTP(); totp != nil {
		t.Error("the new user inherited the TOTP secret")
	}
	assertGone(t, r, controller.OfflineSessionsGVK, ghostPair)
}

func TestReconcile_ExistingUserKeepsItsDexRecord(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	pair := useridentity.Name(adminUserName, naming.LocalConnectorID)
	r := newTestReconciler(t, now,
		namespaceObj(),
		userObj(adminUserName, adminEmail, adminPassword, ""),
		localIdentityObj(adminUserName),
		dexSessionObj(pair, adminUserName, naming.LocalConnectorID),
	)

	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	assertKept(t, r, controller.UserIdentityGVK, pair)
	assertKept(t, r, controller.OfflineSessionsGVK, pair)
}

func TestReconcile_UnmanagedPasswordKeepsTheDexRecord(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	pair := useridentity.Name("external", naming.LocalConnectorID)
	r := newTestReconciler(t, now,
		namespaceObj(),
		passwordObj("unmanaged-external", map[string]any{
			"email": "external@example.com", "username": "external", "userID": "external", "hash": "aGFzaA==",
		}, nil, nil),
		localIdentityObj("external"),
	)

	if _, err := r.Reconcile(t.Context(), userReq("external")); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	assertKept(t, r, controller.UserIdentityGVK, pair)
}

func TestMapUserIdentity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	r := newTestReconciler(t, now, namespaceObj(), userObj(adminUserName, adminEmail, adminPassword, ""))

	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		expected []reconcile.Request
	}{
		{name: "local identity of a deleted user", obj: localIdentityObj("ghost"), expected: []reconcile.Request{userReq("ghost")}},
		{name: "local identity of an existing user", obj: localIdentityObj(adminUserName), expected: []reconcile.Request{userReq(adminUserName)}},
		{name: "external identity", obj: localIdentityObjFor("ghost", "corp-ldap"), expected: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := r.mapUserIdentity(t.Context(), tt.obj)
			if len(got) != len(tt.expected) || (len(got) == 1 && got[0] != tt.expected[0]) {
				t.Errorf("requests = %v, want %v", got, tt.expected)
			}
		})
	}
}

func localIdentityObj(userID string) *unstructured.Unstructured {
	return localIdentityObjFor(userID, naming.LocalConnectorID)
}

func localIdentityObjFor(userID, connID string) *unstructured.Unstructured {
	id := useridentity.New(userID, connID)
	id.SetTOTP(useridentity.MFASecret{Secret: "otpauth://totp/x?secret=AAAA", Confirmed: true})
	return id.Object()
}

func dexSessionObj(name, userID, connID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.OfflineSessionsGVK.GroupVersion().String(),
		"kind":       controller.OfflineSessionsGVK.Kind,
		"metadata":   map[string]any{"name": name, "namespace": naming.DexNamespace},
		"userID":     userID,
		"connID":     connID,
		"totp":       "otpauth://totp/x?secret=AAAA",
	}}
}

func assertGone(t *testing.T, r *Reconciler, gvk schema.GroupVersionKind, name string) {
	t.Helper()
	err := r.client.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, controller.Object(gvk))
	if !apierrors.IsNotFound(err) {
		t.Errorf("%s %s: err = %v, want NotFound", gvk.Kind, name, err)
	}
}

func assertKept(t *testing.T, r *Reconciler, gvk schema.GroupVersionKind, name string) {
	t.Helper()
	if err := r.client.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, controller.Object(gvk)); err != nil {
		t.Errorf("%s %s: %v, want it kept", gvk.Kind, name, err)
	}
}

func getLocalIdentity(t *testing.T, r *Reconciler, userID string) *useridentity.Identity {
	t.Helper()
	obj := controller.Object(controller.UserIdentityGVK)
	key := types.NamespacedName{Namespace: naming.DexNamespace, Name: useridentity.Name(userID, naming.LocalConnectorID)}
	if err := r.client.Get(t.Context(), key, obj); err != nil {
		t.Fatalf("get useridentity of %s: %v", userID, err)
	}
	return useridentity.FromUnstructured(obj)
}
