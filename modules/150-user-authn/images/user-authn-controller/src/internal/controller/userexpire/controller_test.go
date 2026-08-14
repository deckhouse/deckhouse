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

package userexpire

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
)

func TestReconcile_DeletesExpiredUserAndStripsGroupMembers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	r := newTestReconciler(t, now,
		userObj("admin", "2020-02-02T22:22:22Z"),
		userObj("expired-two", "2020-02-02T22:22:22Z"),
		userObj("future", "2150-10-10T10:10:10Z"),
		userObj("without-ttl", ""),
		groupObj("admins", []groupMember{
			{Kind: "User", Name: "admin"},
			{Kind: "User", Name: "expired-two"},
			{Kind: "User", Name: "future"},
		}),
		groupObj("everyone", []groupMember{
			{Kind: "User", Name: "admin"},
			{Kind: "User", Name: "expired-two"},
			{Kind: "Group", Name: "admins"},
		}),
	)

	if _, err := r.Reconcile(t.Context(), userRequest("admin")); err != nil {
		t.Fatalf("reconcile admin: %v", err)
	}
	if _, err := r.Reconcile(t.Context(), userRequest("expired-two")); err != nil {
		t.Fatalf("reconcile expired-two: %v", err)
	}

	assertUserGone(t, r, "admin")
	assertUserGone(t, r, "expired-two")

	admins := getGroup(t, r, "admins")
	assertMembersEqual(t, admins, []groupMember{{Kind: "User", Name: "future"}})

	everyone := getGroup(t, r, "everyone")
	assertMembersEqual(t, everyone, []groupMember{{Kind: "Group", Name: "admins"}})
}

func TestReconcile_KeepsFutureAndUnttlUsers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	r := newTestReconciler(t, now,
		userObj("future", "2150-10-10T10:10:10Z"),
		userObj("without-ttl", ""),
	)

	futureRes, err := r.Reconcile(t.Context(), userRequest("future"))
	if err != nil {
		t.Fatalf("reconcile future: %v", err)
	}
	if futureRes.RequeueAfter <= 0 {
		t.Errorf("future user RequeueAfter = %v, want > 0", futureRes.RequeueAfter)
	}
	getUser(t, r, "future")

	untimed, err := r.Reconcile(t.Context(), userRequest("without-ttl"))
	if err != nil {
		t.Fatalf("reconcile without-ttl: %v", err)
	}
	if untimed.RequeueAfter != 0 {
		t.Errorf("without-ttl RequeueAfter = %v, want 0", untimed.RequeueAfter)
	}
	getUser(t, r, "without-ttl")
}

func TestReconcile_InvalidExpireAtDoesNotDelete(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
		userObj("bad", "not-a-time"),
	)
	res, err := r.Reconcile(t.Context(), userRequest("bad"))
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if res.RequeueAfter != 0 {
		t.Errorf("RequeueAfter = %v, want 0", res.RequeueAfter)
	}
	getUser(t, r, "bad")
}

func TestReconcile_MissingUserIsNoOp(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, time.Now())
	if _, err := r.Reconcile(t.Context(), userRequest("gone")); err != nil {
		t.Fatalf("Reconcile missing user: %v", err)
	}
}

func TestReconcile_HonorsCanceledContext(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, time.Now())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := r.Reconcile(ctx, userRequest("x"))
	if err == nil {
		t.Fatal("Reconcile with canceled context: want error")
	}
}

func TestRemoveUsersFromGroupMembers(t *testing.T) {
	t.Parallel()

	members := []groupMember{
		{Kind: "User", Name: "admin"},
		{Kind: "User", Name: "future"},
		{Kind: "Group", Name: "admins"},
	}
	kept, removed := removeUsersFromGroupMembers(members, map[string]struct{}{"admin": {}})
	if len(removed) != 1 || removed[0] != "admin" {
		t.Errorf("removed = %v, want [admin]", removed)
	}
	if len(kept) != 2 {
		t.Fatalf("kept = %v", kept)
	}
	if kept[0] != (groupMember{Kind: "User", Name: "future"}) {
		t.Errorf("kept[0] = %+v", kept[0])
	}
	if kept[1] != (groupMember{Kind: "Group", Name: "admins"}) {
		t.Errorf("kept[1] = %+v", kept[1])
	}
}

func newTestReconciler(t *testing.T, now time.Time, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(newTestMapper()).
		WithObjects(objs...).
		Build()
	return New(c, logr.Discard(), func() time.Time { return now })
}

func newTestMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "deckhouse.io", Version: "v1"},
		{Group: "deckhouse.io", Version: "v1alpha1"},
	})
	m.Add(controller.UserGVK, meta.RESTScopeRoot)
	m.Add(controller.GroupGVK, meta.RESTScopeRoot)
	return m
}

func userRequest(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func userObj(name, expireAt string) *unstructured.Unstructured {
	status := map[string]any{}
	if expireAt != "" {
		status["expireAt"] = expireAt
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.UserGVK.GroupVersion().String(),
		"kind":       controller.UserGVK.Kind,
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"email":    name + "@example.com",
			"password": "password",
		},
		"status": status,
	}}
}

func groupObj(name string, members []groupMember) *unstructured.Unstructured {
	items := make([]any, 0, len(members))
	for _, m := range members {
		items = append(items, map[string]any{"kind": m.Kind, "name": m.Name})
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.GroupGVK.GroupVersion().String(),
		"kind":       controller.GroupGVK.Kind,
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"name":    name,
			"members": items,
		},
	}}
}

func getUser(t *testing.T, r *Reconciler, name string) *unstructured.Unstructured {
	t.Helper()
	obj := controller.Object(controller.UserGVK)
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, obj); err != nil {
		t.Fatalf("get user %s: %v", name, err)
	}
	return obj
}

func getGroup(t *testing.T, r *Reconciler, name string) *unstructured.Unstructured {
	t.Helper()
	obj := controller.Object(controller.GroupGVK)
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, obj); err != nil {
		t.Fatalf("get group %s: %v", name, err)
	}
	return obj
}

func assertUserGone(t *testing.T, r *Reconciler, name string) {
	t.Helper()
	obj := controller.Object(controller.UserGVK)
	err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, obj)
	if err == nil {
		t.Fatalf("user %s still exists", name)
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("get user %s: %v", name, err)
	}
}

func assertMembersEqual(t *testing.T, group *unstructured.Unstructured, want []groupMember) {
	t.Helper()
	got, err := groupMembers(group)
	if err != nil {
		t.Fatalf("group members: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("members = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("members[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
