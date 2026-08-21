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
	"encoding/json"
	"sync/atomic"
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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	adminEmail      = "admin@example.com"
	adminUserName   = "admin"
	adminPassword   = "$2a$10$E/MjyzFi6GZkta9GHd8zCeuYigbLenXv18jkxOZ6vhoWsKnaxNJou"
	adminEncodedPW  = "mfsg22loibsxqylnobwgkltdn5w4x4u44scceizf"
	adminEncodedB64 = "JDJhJDEwJEUvTWp5ekZpNkdaa3RhOUdIZDh6Q2V1WWlnYkxlblh2MThqa3hPWjZ2aG9Xc0tuYXhOSm91"
	liveHash        = "dXNlckNoYW5nZWRIYXNo"
)

func TestReconcile_CreatePassword(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	r := newTestReconciler(t, now, namespaceObj(), userObj(adminUserName, adminEmail, adminPassword, "30m"))

	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	pw := getPassword(t, r, adminEncodedPW)
	if got := strField(pw, "email"); got != adminEmail {
		t.Errorf("email = %q", got)
	}
	if got := strField(pw, "username"); got != adminUserName {
		t.Errorf("username = %q", got)
	}
	if got := strField(pw, "userID"); got != adminUserName {
		t.Errorf("userID = %q", got)
	}
	if got := strField(pw, "hash"); got != adminEncodedB64 {
		t.Errorf("hash = %q, want bcrypt base64", got)
	}
	if pw.GetLabels()[heritageLabel] != heritageValue || pw.GetLabels()[moduleLabel] != moduleValue || pw.GetLabels()[appLabel] != appValue {
		t.Errorf("labels = %v", pw.GetLabels())
	}
	if pw.GetAnnotations()[helmResourcePolicyAnnotation] != helmResourcePolicyKeep {
		t.Errorf("keep annotation = %q", pw.GetAnnotations()[helmResourcePolicyAnnotation])
	}
	if got := strField(pw, "hashUpdatedAt"); got != now.UTC().Format(time.RFC3339) {
		t.Errorf("hashUpdatedAt = %q", got)
	}

	expireAt, found, err := unstructured.NestedString(getUser(t, r, adminUserName).Object, "status", "expireAt")
	if err != nil || !found {
		t.Fatalf("status.expireAt: found=%v err=%v", found, err)
	}
	wantExpire := now.Add(30 * time.Minute).UTC().Format(time.RFC3339)
	if expireAt != wantExpire {
		t.Errorf("expireAt = %q, want %q", expireAt, wantExpire)
	}

	assertNoUserAccount(t, r, adminEmail)
}

func TestReconcile_SkipIfEqual(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	var patches atomic.Int32
	var statusPatches atomic.Int32

	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(newTestMapper()).
		WithStatusSubresource(userStatusObject()).
		WithObjects(namespaceObj(), userObj(adminUserName, adminEmail, adminPassword, "")).
		WithInterceptorFuncs(interceptor.Funcs{
			Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
				patches.Add(1)
				return cl.Patch(ctx, obj, patch, opts...)
			},
			SubResourcePatch: func(ctx context.Context, cl client.Client, subResourceName string, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
				if subResourceName == "status" {
					statusPatches.Add(1)
				}
				return cl.SubResource(subResourceName).Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()

	r := New(c, c, logr.Discard(), func() time.Time { return now })
	req := userReq(adminUserName)
	if _, err := r.Reconcile(t.Context(), req); err != nil {
		t.Fatalf("first Reconcile: %v", err)
	}
	if statusPatches.Load() != 1 {
		t.Fatalf("status patches after create = %d, want 1", statusPatches.Load())
	}

	patches.Store(0)
	statusPatches.Store(0)
	if _, err := r.Reconcile(t.Context(), req); err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if patches.Load() != 0 {
		t.Errorf("password patches after skip-if-equal = %d, want 0", patches.Load())
	}
	if statusPatches.Load() != 0 {
		t.Errorf("status patches after skip-if-equal = %d, want 0", statusPatches.Load())
	}
}

func TestReconcile_EmailRenamePreservesHash(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	until := now.Add(24 * time.Hour).UTC().Format(time.RFC3339)
	oldPW := passwordObj("oldemailencodedname", map[string]any{
		"email":                           "old@example.com",
		"username":                        adminUserName,
		"userID":                          adminUserName,
		"hash":                            liveHash,
		"hashUpdatedAt":                   "2024-01-02T03:04:05Z",
		"incorrectPasswordLoginAttempts":  int64(3),
		"lockedUntil":                     until,
		"previousHashes":                  []any{"b2xkSGFzaA=="},
		"requireResetHashOnNextSuccLogin": true,
	}, map[string]string{heritageLabel: heritageValue, moduleLabel: moduleValue, appLabel: appValue}, nil)

	r := newTestReconciler(t, now, namespaceObj(), userObj(adminUserName, adminEmail, "$2a$10$thisIsTheOriginalUserSpecHashMustNotOverwriteLiveHashAA", ""), oldPW)
	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if passwordExists(t, r, "oldemailencodedname") {
		t.Fatal("old-name password still exists")
	}
	pw := getPassword(t, r, adminEncodedPW)
	if got := strField(pw, "hash"); got != liveHash {
		t.Errorf("hash = %q, want live hash preserved", got)
	}
	if got := strField(pw, "hashUpdatedAt"); got != "2024-01-02T03:04:05Z" {
		t.Errorf("hashUpdatedAt = %q", got)
	}
	if got, _, _ := unstructured.NestedInt64(pw.Object, "incorrectPasswordLoginAttempts"); got != 3 {
		t.Errorf("attempts = %d", got)
	}
	if got := strField(pw, "lockedUntil"); got != until {
		t.Errorf("lockedUntil = %q", got)
	}
	if got, _, _ := unstructured.NestedBool(pw.Object, "requireResetHashOnNextSuccLogin"); !got {
		t.Error("requireResetHashOnNextSuccLogin = false")
	}
	prev, _, _ := unstructured.NestedStringSlice(pw.Object, "previousHashes")
	if len(prev) != 1 || prev[0] != "b2xkSGFzaA==" {
		t.Errorf("previousHashes = %v", prev)
	}
	if got := strField(pw, "email"); got != adminEmail {
		t.Errorf("email = %q", got)
	}
}

func TestReconcile_OrphanDeleteOnlyHeritageDeckhouse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	managed := passwordObj("managed-ghost", map[string]any{
		"email":    "ghost@example.com",
		"username": "ghost",
		"userID":   "ghost",
		"hash":     "aGFzaA==",
	}, map[string]string{heritageLabel: heritageValue, moduleLabel: moduleValue, appLabel: appValue}, nil)
	nameless := passwordObj("managed-nameless", map[string]any{
		"email": "nameless@example.com",
		"hash":  "aGFzaA==",
	}, map[string]string{heritageLabel: heritageValue, moduleLabel: moduleValue, appLabel: appValue}, nil)
	unmanaged := passwordObj("unmanaged-ghost", map[string]any{
		"email":    "external@example.com",
		"username": "external",
		"userID":   "external",
		"hash":     "aGFzaA==",
	}, nil, nil)

	r := newTestReconciler(t, now, namespaceObj(), userObj(adminUserName, adminEmail, adminPassword, ""), managed, nameless, unmanaged)
	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
		t.Fatalf("Reconcile admin: %v", err)
	}
	if _, err := r.Reconcile(t.Context(), userReq("ghost")); err != nil {
		t.Fatalf("Reconcile ghost: %v", err)
	}

	if passwordExists(t, r, "managed-ghost") {
		t.Error("managed orphan password was not deleted")
	}
	if passwordExists(t, r, "managed-nameless") {
		t.Error("managed password with empty username was not deleted")
	}
	if !passwordExists(t, r, "unmanaged-ghost") {
		t.Error("unmanaged password was deleted")
	}
}

func TestReconcile_InvalidTTLDoesNotFail(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	r := newTestReconciler(t, now, namespaceObj(), userObj(adminUserName, adminEmail, adminPassword, "9999999h"))
	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
		t.Fatalf("Reconcile with invalid TTL: %v", err)
	}
	_ = getPassword(t, r, adminEncodedPW)
	_, found, err := unstructured.NestedString(getUser(t, r, adminUserName).Object, "status", "expireAt")
	if err != nil {
		t.Fatalf("status.expireAt: %v", err)
	}
	if found {
		t.Error("expireAt set for invalid TTL")
	}
}

func TestReconcile_StatusLockFromLockedUntilAndAdminAnnotation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	until := now.Add(2 * time.Hour).UTC().Format(time.RFC3339)

	tests := []struct {
		name        string
		lockedUntil string
		annots      map[string]string
		wantState   bool
		wantReason  string
		wantMessage string
		wantUntil   string
	}{
		{
			name:        "policy lockout",
			lockedUntil: until,
			wantState:   true,
			wantReason:  lockReasonPasswordPolicy,
			wantMessage: lockMessagePasswordPolicy,
			wantUntil:   until,
		},
		{
			name:        "admin annotation while lock active",
			lockedUntil: until,
			annots:      map[string]string{lockedByAdministratorAnnot: ""},
			wantState:   true,
			wantReason:  lockReasonAdministrator,
			wantMessage: lockMessageAdministrator,
			wantUntil:   until,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pw := passwordObj(adminEncodedPW, map[string]any{
				"email":       adminEmail,
				"username":    adminUserName,
				"userID":      adminUserName,
				"hash":        liveHash,
				"lockedUntil": tt.lockedUntil,
			}, map[string]string{heritageLabel: heritageValue}, tt.annots)
			r := newTestReconciler(t, now, namespaceObj(), userObj(adminUserName, adminEmail, adminPassword, ""), pw)
			res, err := r.Reconcile(t.Context(), userReq(adminUserName))
			if err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			if res.RequeueAfter != 2*time.Hour {
				t.Errorf("RequeueAfter = %v, want 2h so status unlocks when lockedUntil expires", res.RequeueAfter)
			}
			lock := userStatusLock(t, getUser(t, r, adminUserName))
			if lock.State != tt.wantState {
				t.Errorf("state = %v, want %v", lock.State, tt.wantState)
			}
			if lock.Reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", lock.Reason, tt.wantReason)
			}
			if lock.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", lock.Message, tt.wantMessage)
			}
			if lock.Until != tt.wantUntil {
				t.Errorf("until = %q, want %q", lock.Until, tt.wantUntil)
			}
			assertNoUserAccount(t, r, adminEmail)
		})
	}
}

func TestReconcile_ExpiredAdminAnnotationStrippedOnExisting(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour).UTC().Format(time.RFC3339)
	pw := passwordObj(adminEncodedPW, map[string]any{
		"email":       adminEmail,
		"username":    adminUserName,
		"userID":      adminUserName,
		"hash":        liveHash,
		"lockedUntil": expired,
	}, map[string]string{heritageLabel: heritageValue}, map[string]string{
		lockedByAdministratorAnnot:   "",
		helmResourcePolicyAnnotation: helmResourcePolicyKeep,
	})

	stale := userObj(adminUserName, adminEmail, adminPassword, "")
	stale.Object["status"] = map[string]any{
		"groups": []any{},
		"lock": map[string]any{
			"state":   true,
			"reason":  lockReasonAdministrator,
			"message": lockMessageAdministrator,
			"until":   expired,
		},
	}

	r := newTestReconciler(t, now, namespaceObj(), stale, pw)
	res, err := r.Reconcile(t.Context(), userReq(adminUserName))
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if res.RequeueAfter != 0 {
		t.Errorf("RequeueAfter = %v, want 0 after lock expiry", res.RequeueAfter)
	}

	got := getPassword(t, r, adminEncodedPW)
	if _, ok := got.GetAnnotations()[lockedByAdministratorAnnot]; ok {
		t.Error("admin lock annotation still present after expiry")
	}
	lock := userStatusLock(t, getUser(t, r, adminUserName))
	if lock.State {
		t.Error("status.lock.state = true, want false after expired lock")
	}
	if lock.Reason != "" || lock.Message != "" || lock.Until != "" {
		t.Errorf("stale lock fields remain: %+v", lock)
	}
}

func TestReconcile_NamelessCanonicalPasswordNotStolen(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	bobEmail := "bob@example.com"
	bobName := "bob"
	bobPWName := passwordName(bobEmail)
	nameless := passwordObj(bobPWName, map[string]any{
		"email":          bobEmail,
		"hash":           liveHash,
		"previousHashes": []any{"b2xkSGFzaA=="},
	}, map[string]string{heritageLabel: heritageValue, moduleLabel: moduleValue, appLabel: appValue}, nil)

	r := newTestReconciler(t, now,
		userObj(adminUserName, adminEmail, adminPassword, ""),
		userObj(bobName, bobEmail, adminPassword, ""),
		nameless,
	)
	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
		t.Fatalf("Reconcile admin: %v", err)
	}
	if !passwordExists(t, r, bobPWName) {
		t.Fatal("bob's nameless canonical password was deleted by admin reconcile")
	}

	if _, err := r.Reconcile(t.Context(), userReq(bobName)); err != nil {
		t.Fatalf("Reconcile bob: %v", err)
	}
	pw := getPassword(t, r, bobPWName)
	if got := strField(pw, "hash"); got != liveHash {
		t.Errorf("hash = %q, want live hash preserved", got)
	}
	if got := strField(pw, "username"); got != bobName {
		t.Errorf("username = %q, want adopted %q", got, bobName)
	}
}

func TestReconcile_DoesNotWriteUserAccount(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	r := newTestReconciler(t, now, namespaceObj(), userObj(adminUserName, adminEmail, adminPassword, ""))
	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertNoUserAccount(t, r, adminEmail)
}

func TestReconcile_GroupMembershipOnPasswordAndStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	g1 := groupObj("group-1", "group-1", []groupMember{{Kind: memberKindUser, Name: adminUserName}})
	g2 := groupObj("group-2", "group-2", []groupMember{{Kind: memberKindGroup, Name: "group-1"}})
	r := newTestReconciler(t, now, namespaceObj(), userObj(adminUserName, adminEmail, adminPassword, ""), g1, g2)
	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	pw := getPassword(t, r, adminEncodedPW)
	gotGroups, _, _ := unstructured.NestedStringSlice(pw.Object, "groups")
	if !equalStringSets(gotGroups, []string{"group-1", "group-2"}) {
		t.Errorf("password groups = %v", gotGroups)
	}
	statusGroups, _, _ := unstructured.NestedStringSlice(getUser(t, r, adminUserName).Object, "status", "groups")
	if !equalStringSets(statusGroups, []string{"group-1", "group-2"}) {
		t.Errorf("status.groups = %v", statusGroups)
	}
}

func TestReconcile_HonorsCanceledContext(t *testing.T) {
	t.Parallel()

	r := newTestReconciler(t, time.Now())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := r.Reconcile(ctx, userReq("x"))
	if err == nil {
		t.Fatal("Reconcile with canceled context: want error")
	}
}

func TestMapPassword(t *testing.T) {
	t.Parallel()

	r := &Reconciler{log: logr.Discard(), now: time.Now}

	if reqs := r.mapPassword(t.Context(), nil); reqs != nil {
		t.Errorf("nil object mapped: %v", reqs)
	}

	pw := passwordObj("pw", map[string]any{"username": adminUserName, "email": adminEmail}, nil, nil)
	reqs := r.mapPassword(t.Context(), pw)
	if len(reqs) != 1 || reqs[0].Name != adminUserName {
		t.Errorf("mapPassword = %v, want %s", reqs, adminUserName)
	}

	empty := passwordObj("pw-empty", map[string]any{"email": adminEmail}, nil, nil)
	if reqs := r.mapPassword(t.Context(), empty); reqs != nil {
		t.Errorf("password without username mapped: %v", reqs)
	}
}

func TestEncodePasswordHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "bcrypt prefix is base64-encoded", raw: adminPassword, want: adminEncodedB64},
		{name: "non-bcrypt stored verbatim", raw: "password", want: "password"},
		{name: "empty stored verbatim", raw: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := encodePasswordHash(tt.raw); got != tt.want {
				t.Errorf("encodePasswordHash(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestPasswordNameIsFnvNotLocalPrefixed(t *testing.T) {
	t.Parallel()

	got := passwordName(adminEmail)
	if got != adminEncodedPW {
		t.Errorf("passwordName = %q, want %q", got, adminEncodedPW)
	}
	if len(got) >= 6 && got[:6] == "local-" {
		t.Fatal("password name must not use LocalName prefix")
	}
}

func newTestReconciler(t *testing.T, now time.Time, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(newTestMapper()).
		WithStatusSubresource(userStatusObject()).
		WithObjects(objs...).
		Build()
	return New(c, c, logr.Discard(), func() time.Time { return now })
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	return scheme
}

func newTestMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "", Version: "v1"},
		{Group: "dex.coreos.com", Version: "v1"},
		{Group: "deckhouse.io", Version: "v1"},
		{Group: "deckhouse.io", Version: "v1alpha1"},
		v1alpha1.SchemeGroupVersion,
	})
	m.Add(controller.NamespaceGVK, meta.RESTScopeRoot)
	m.Add(controller.PasswordGVK, meta.RESTScopeNamespace)
	m.Add(controller.UserGVK, meta.RESTScopeRoot)
	m.Add(controller.GroupGVK, meta.RESTScopeRoot)
	m.Add(v1alpha1.SchemeGroupVersion.WithKind("UserAccount"), meta.RESTScopeRoot)
	return m
}

func userStatusObject() client.Object {
	u := controller.Object(controller.UserGVK)
	return u
}

func userReq(name string) reconcile.Request {
	return reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
}

func namespaceObj() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]any{"name": naming.DexNamespace},
	}}
}

func groupObj(name, specName string, members []groupMember) *unstructured.Unstructured {
	items := make([]any, 0, len(members))
	for _, m := range members {
		items = append(items, map[string]any{"kind": m.Kind, "name": m.Name})
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.GroupGVK.GroupVersion().String(),
		"kind":       controller.GroupGVK.Kind,
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{
			"name":    specName,
			"members": items,
		},
	}}
}

func userObj(name, email, password, ttl string) *unstructured.Unstructured {
	spec := map[string]any{
		"email":    email,
		"password": password,
		"userID":   name,
	}
	if ttl != "" {
		spec["ttl"] = ttl
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": controller.UserGVK.GroupVersion().String(),
		"kind":       controller.UserGVK.Kind,
		"metadata":   map[string]any{"name": name},
		"spec":       spec,
	}}
}

func passwordObj(name string, fields map[string]any, labels, annots map[string]string) *unstructured.Unstructured {
	meta := map[string]any{
		"name":      name,
		"namespace": naming.DexNamespace,
	}
	if len(labels) > 0 {
		lbl := make(map[string]any, len(labels))
		for k, v := range labels {
			lbl[k] = v
		}
		meta["labels"] = lbl
	}
	if len(annots) > 0 {
		a := make(map[string]any, len(annots))
		for k, v := range annots {
			a[k] = v
		}
		meta["annotations"] = a
	}
	obj := map[string]any{
		"apiVersion": controller.PasswordGVK.GroupVersion().String(),
		"kind":       controller.PasswordGVK.Kind,
		"metadata":   meta,
	}
	for k, v := range fields {
		obj[k] = v
	}
	return &unstructured.Unstructured{Object: obj}
}

func getPassword(t *testing.T, r *Reconciler, name string) *unstructured.Unstructured {
	t.Helper()
	obj := controller.Object(controller.PasswordGVK)
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name, Namespace: naming.DexNamespace}, obj); err != nil {
		t.Fatalf("get password %s: %v", name, err)
	}
	return obj
}

func passwordExists(t *testing.T, r *Reconciler, name string) bool {
	t.Helper()
	obj := controller.Object(controller.PasswordGVK)
	err := r.client.Get(t.Context(), types.NamespacedName{Name: name, Namespace: naming.DexNamespace}, obj)
	if err == nil {
		return true
	}
	if apierrors.IsNotFound(err) {
		return false
	}
	t.Fatalf("get password %s: %v", name, err)
	return false
}

func getUser(t *testing.T, r *Reconciler, name string) *unstructured.Unstructured {
	t.Helper()
	obj := controller.Object(controller.UserGVK)
	if err := r.client.Get(t.Context(), types.NamespacedName{Name: name}, obj); err != nil {
		t.Fatalf("get user %s: %v", name, err)
	}
	return obj
}

func strField(obj *unstructured.Unstructured, field string) string {
	s, _, _ := unstructured.NestedString(obj.Object, field)
	return s
}

func userStatusLock(t *testing.T, user *unstructured.Unstructured) userLock {
	t.Helper()
	raw, err := json.Marshal(user.Object["status"])
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	var status userStatusFields
	if err := json.Unmarshal(raw, &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	return userLock{
		State:   status.Lock.State,
		Reason:  status.Lock.Reason,
		Message: status.Lock.Message,
		Until:   status.Lock.Until,
	}
}

func assertNoUserAccount(t *testing.T, r *Reconciler, email string) {
	t.Helper()
	ua := &v1alpha1.UserAccount{}
	err := r.client.Get(t.Context(), types.NamespacedName{Name: v1alpha1LocalName(email)}, ua)
	if err == nil {
		t.Fatalf("UserAccount %s exists; user reconciler must not write UserAccount", ua.Name)
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("get useraccount: %v", err)
	}
}

func v1alpha1LocalName(email string) string {
	return "local-" + passwordName(email)
}
