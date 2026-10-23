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
	"context"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

var (
	jane     = Pair{UserID: "jane", ConnectorID: naming.LocalConnectorID, Claims: useridentity.Claims{UserID: "jane", Email: "jane@example.com"}}
	janePair = naming.OfflineTokenName("jane", naming.LocalConnectorID)
)

// janeObjects is a local user signed in to three clients: console and grafana through browser
// sessions, kubeconfig-generator through a token without one. A fourth sign-in has finished in
// Dex and waits for the client: an auth request and the code issued for it.
func janeObjects() []client.Object {
	return []client.Object{
		codeObj("code-a", naming.LocalConnectorID, "jane", 2),
		requestObj("req-a", naming.LocalConnectorID, "jane", 2),
		identityObj(naming.LocalConnectorID, "jane", 2),
		offlineObj(naming.LocalConnectorID, "jane", map[string][2]string{
			"console":              {"rt-console", "sid-a"},
			"kubeconfig-generator": {"rt-kubeconfig", ""},
		}),
		tokenObj("rt-console", naming.LocalConnectorID, "jane", "console", 2),
		tokenObj("rt-kubeconfig", naming.LocalConnectorID, "jane", "kubeconfig-generator", 0),
		sessionObj("sid-a", naming.LocalConnectorID, "jane", 2, "console"),
		sessionObj("sid-b", naming.LocalConnectorID, "jane", 0, "grafana"),
	}
}

// The order is the contract: the generation first (Dex refuses everything older from then on), the
// Revocation next (the subscribers hear of it before the long deletions), then the sessions, each
// after its own Revocation, with the sign-ins in progress, then the grants.
func TestLogoutPair_Order(t *testing.T) {
	t.Parallel()

	rec := &calls{}
	c := newTestClient(t, rec, interceptor.Funcs{}, janeObjects()...)
	if _, err := newTestRevoker(c).LogoutPair(t.Context(), jane, revocation.ReasonLock, "op-1"); err != nil {
		t.Fatalf("LogoutPair: %v", err)
	}

	expected := []string{
		"update UserIdentity " + janePair,
		"create Revocation " + revocation.PairName(janePair, 3),
		"create Revocation " + revocation.SessionName("sid-a"),
		"delete AuthSession sid-a",
		"create Revocation " + revocation.SessionName("sid-b"),
		"delete AuthSession sid-b",
		"delete AuthCode code-a",
		"delete AuthRequest req-a",
		"delete RefreshToken rt-console",
		"delete RefreshToken rt-kubeconfig",
		"patch OfflineSessions " + janePair,
		"patch OfflineSessions " + janePair,
	}
	if got := rec.list(); !slices.Equal(got, expected) {
		t.Errorf("calls:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}
}

func TestLogoutPair_WritesThePairRevocation(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, janeObjects()...)
	before := metricValue(t, writtenCounter.WithLabelValues(revocation.TypePair, revocation.ReasonResetPassword))
	res, err := newTestRevoker(c).LogoutPair(t.Context(), jane, revocation.ReasonResetPassword, "op-1")
	if err != nil {
		t.Fatalf("LogoutPair: %v", err)
	}

	expectedName := revocation.PairName(janePair, 3)
	if res.Generation != 3 || res.Name != expectedName || res.RefreshTokensDeleted != 2 || res.AuthSessionsDeleted != 2 {
		t.Errorf("result = %+v", res)
	}
	if g := generationOf(t, c, naming.LocalConnectorID, "jane"); g != 3 {
		t.Errorf("stored generation = %d, want 3", g)
	}
	rev := getRevocation(t, c, expectedName)
	expected := revocation.Revocation{
		Name:         expectedName,
		Type:         revocation.TypePair,
		ConnectorID:  naming.LocalConnectorID,
		UserID:       "jane",
		Subject:      "CgRqYW5lEgVsb2NhbA",
		Generation:   3,
		Clients:      []string{"console", "grafana", "kubeconfig-generator"},
		Reason:       revocation.ReasonResetPassword,
		Source:       revocation.SourceController,
		CreatedAt:    revokeTime,
		OperationUID: "op-1",
	}
	if !equalRevocation(rev, expected) {
		t.Errorf("revocation = %+v\nwant %+v", rev, expected)
	}
	if after := metricValue(t, writtenCounter.WithLabelValues(revocation.TypePair, revocation.ReasonResetPassword)); after-before != 1 {
		t.Errorf("written counter grew by %v, want 1", after-before)
	}

	for _, gone := range []struct {
		gvk  schema.GroupVersionKind
		name string
	}{
		{controller.AuthSessionGVK, "sid-a"},
		{controller.AuthSessionGVK, "sid-b"},
		{controller.AuthCodeGVK, "code-a"},
		{controller.AuthRequestGVK, "req-a"},
		{controller.RefreshTokenGVK, "rt-console"},
		{controller.RefreshTokenGVK, "rt-kubeconfig"},
	} {
		if exists(t, c, gone.gvk, gone.name) {
			t.Errorf("%s %s is still there", gone.gvk.Kind, gone.name)
		}
	}
	if refs := refsOf(t, c, naming.LocalConnectorID, "jane"); len(refs) != 0 {
		t.Errorf("references left: %v", refs)
	}
	// OfflineSessions stays: the transitional Dex still reads the TOTP secret and the lock there.
	if !exists(t, c, controller.OfflineSessionsGVK, janePair) {
		t.Error("OfflineSessions was deleted")
	}
}

// A pair without a UserIdentity gets one at generation 1: from then on Dex refuses the tokens
// issued without a generation.
func TestLogoutPair_CreatesTheUserIdentity(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, tokenObj("rt-1", "corp-ldap", "jdoe", "console", 0))
	p := Pair{UserID: "jdoe", ConnectorID: "corp-ldap", Claims: useridentity.Claims{UserID: "jdoe", Email: "jdoe@example.com"}}
	res, err := newTestRevoker(c).LogoutPair(t.Context(), p, revocation.ReasonLock, "")
	if err != nil {
		t.Fatalf("LogoutPair: %v", err)
	}
	if res.Generation != 1 || generationOf(t, c, "corp-ldap", "jdoe") != 1 {
		t.Errorf("generation = %d, want 1", res.Generation)
	}
	if rev := getRevocation(t, c, res.Name); rev.OperationUID != "" || !slices.Equal(rev.Clients, []string{"console"}) {
		t.Errorf("revocation = %+v", rev)
	}
	if exists(t, c, controller.RefreshTokenGVK, "rt-1") {
		t.Error("the refresh token is still there")
	}
}

// Each ended browser session gets a Revocation with its sid and its clients: the pair revocation
// names neither, and Dex sends the back-channel logout of the session from it.
func TestLogoutPair_WritesTheSessionRevocations(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, janeObjects()...)
	res, err := newTestRevoker(c).LogoutPair(t.Context(), jane, revocation.ReasonLock, "op-1")
	if err != nil {
		t.Fatalf("LogoutPair: %v", err)
	}
	if res.Name != revocation.PairName(janePair, 3) || res.RevocationsWritten != 3 || res.AuthSessionsDeleted != 2 {
		t.Errorf("result = %+v", res)
	}
	for sid, clients := range map[string][]string{"sid-a": {"console"}, "sid-b": {"grafana"}} {
		rev := getRevocation(t, c, revocation.SessionName(sid))
		ok := rev.Type == revocation.TypeSession &&
			rev.SessionID == sid &&
			slices.Equal(rev.Clients, clients) &&
			rev.UserID == "jane" &&
			rev.ConnectorID == naming.LocalConnectorID &&
			rev.Subject == revocation.Subject("jane", naming.LocalConnectorID) &&
			rev.Reason == revocation.ReasonLock &&
			rev.Source == revocation.SourceController &&
			rev.OperationUID == "op-1"
		if !ok {
			t.Errorf("revocation of %s = %+v", sid, rev)
		}
	}
}

// A retry of the operation finds its Revocation by the operation UID and does not raise the
// generation again.
func TestLogoutPair_IdempotentByOperationUID(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, janeObjects()...)
	r := newTestRevoker(c)
	first, err := r.LogoutPair(t.Context(), jane, revocation.ReasonLock, "op-1")
	if err != nil {
		t.Fatalf("first LogoutPair: %v", err)
	}
	second, err := r.LogoutPair(t.Context(), jane, revocation.ReasonLock, "op-1")
	if err != nil {
		t.Fatalf("second LogoutPair: %v", err)
	}
	if first.Generation != 3 || second.Generation != 3 || second.Name != first.Name {
		t.Errorf("first = %+v, second = %+v; want one generation", first, second)
	}
	if g := generationOf(t, c, naming.LocalConnectorID, "jane"); g != 3 {
		t.Errorf("stored generation = %d, want 3", g)
	}
	// One of the pair and one of each session.
	if revs := listRevocations(t, c); len(revs) != 3 {
		t.Errorf("revocations = %d, want 3", len(revs))
	}

	// Another operation raises it again.
	third, err := r.LogoutPair(t.Context(), jane, revocation.ReasonLogout, "op-2")
	if err != nil {
		t.Fatalf("third LogoutPair: %v", err)
	}
	if third.Generation != 4 {
		t.Errorf("generation of another operation = %d, want 4", third.Generation)
	}
}

// A retry after a failure past the Revocation resumes the deletions at the generation it recorded.
func TestLogoutPair_RetryResumesTheDeletions(t *testing.T) {
	t.Parallel()

	var failed atomic.Bool
	c := newTestClient(t, &calls{}, interceptor.Funcs{}, janeObjects()...)
	broken := interceptor.NewClient(c, interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if kind(obj) == "RefreshToken" && !failed.Swap(true) {
				return apierrors.NewServiceUnavailable("etcd is away")
			}
			return cl.Delete(ctx, obj, opts...)
		},
	})
	r := newTestRevoker(broken)
	if _, err := r.LogoutPair(t.Context(), jane, revocation.ReasonLock, "op-1"); err == nil {
		t.Fatal("LogoutPair succeeded through a failed delete")
	}
	res, err := r.LogoutPair(t.Context(), jane, revocation.ReasonLock, "op-1")
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if res.Generation != 3 || generationOf(t, c, naming.LocalConnectorID, "jane") != 3 {
		t.Errorf("generation = %d after the retry, want 3", res.Generation)
	}
	if exists(t, c, controller.RefreshTokenGVK, "rt-console") || exists(t, c, controller.RefreshTokenGVK, "rt-kubeconfig") {
		t.Error("the retry left refresh tokens")
	}
}

// What Dex issued at the new generation or later, a sign-in that raced the operation, stays.
func TestLogoutPair_KeepsWhatWasIssuedAfter(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{},
		identityObj(naming.LocalConnectorID, "jane", 2),
		offlineObj(naming.LocalConnectorID, "jane", map[string][2]string{"console": {"rt-new", "sid-new"}}),
		tokenObj("rt-new", naming.LocalConnectorID, "jane", "console", 3),
		sessionObj("sid-new", naming.LocalConnectorID, "jane", 3, "console"),
	)
	res, err := newTestRevoker(c).LogoutPair(t.Context(), jane, revocation.ReasonLogout, "")
	if err != nil {
		t.Fatalf("LogoutPair: %v", err)
	}
	if res.RefreshTokensDeleted != 0 || res.AuthSessionsDeleted != 0 {
		t.Errorf("result = %+v, want nothing deleted", res)
	}
	if !exists(t, c, controller.RefreshTokenGVK, "rt-new") || !exists(t, c, controller.AuthSessionGVK, "sid-new") {
		t.Error("a sign-in after the revocation lost its token or session")
	}
	if refs := refsOf(t, c, naming.LocalConnectorID, "jane"); len(refs) != 1 {
		t.Errorf("references = %v, want the new one kept", refs)
	}
}

// F1: the sign-ins of the pair that Dex finished before the revocation and the client has not
// exchanged yet are deleted - codes and auth requests at any older generation, generation 0
// included. A sign-in at the new generation, a request nobody has signed in to yet, and the
// sign-ins of another pair stay.
func TestLogoutPair_DeletesThePendingSignIns(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{},
		identityObj("corp-ldap", "jdoe", 0),
		codeObj("code-gen0", "corp-ldap", "jdoe", 0),
		codeObj("code-new", "corp-ldap", "jdoe", 1),
		requestObj("req-gen0", "corp-ldap", "jdoe", 0),
		requestObj("req-anonymous", "", "", 0),
		codeObj("code-local", naming.LocalConnectorID, "jdoe", 0),
		requestObj("req-other", "corp-ldap", "jane", 0),
	)
	p := Pair{UserID: "jdoe", ConnectorID: "corp-ldap", Claims: useridentity.Claims{UserID: "jdoe"}}
	res, err := newTestRevoker(c).LogoutPair(t.Context(), p, revocation.ReasonLogout, "")
	if err != nil {
		t.Fatalf("LogoutPair: %v", err)
	}
	if res.Generation != 1 {
		t.Fatalf("generation = %d, want 1", res.Generation)
	}
	for _, gone := range []struct {
		gvk  schema.GroupVersionKind
		name string
	}{
		{controller.AuthCodeGVK, "code-gen0"},
		{controller.AuthRequestGVK, "req-gen0"},
	} {
		if exists(t, c, gone.gvk, gone.name) {
			t.Errorf("%s %s of the revoked generation is still there", gone.gvk.Kind, gone.name)
		}
	}
	for _, kept := range []struct {
		gvk  schema.GroupVersionKind
		name string
	}{
		{controller.AuthCodeGVK, "code-new"},
		{controller.AuthRequestGVK, "req-anonymous"},
		{controller.AuthCodeGVK, "code-local"},
		{controller.AuthRequestGVK, "req-other"},
	} {
		if !exists(t, c, kept.gvk, kept.name) {
			t.Errorf("%s %s was deleted", kept.gvk.Kind, kept.name)
		}
	}
}

// A retry after a failure in the middle of the sign-ins resumes them at the recorded generation;
// one more run of the same operation finds nothing to delete and succeeds.
func TestLogoutPair_RetryResumesThePendingSignIns(t *testing.T) {
	t.Parallel()

	var failed atomic.Bool
	c := newTestClient(t, &calls{}, interceptor.Funcs{}, janeObjects()...)
	broken := interceptor.NewClient(c, interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if kind(obj) == "AuthRequest" && !failed.Swap(true) {
				return apierrors.NewServiceUnavailable("etcd is away")
			}
			return cl.Delete(ctx, obj, opts...)
		},
	})
	r := newTestRevoker(broken)
	if _, err := r.LogoutPair(t.Context(), jane, revocation.ReasonLock, "op-1"); err == nil {
		t.Fatal("LogoutPair succeeded through a failed delete")
	}
	if !exists(t, c, controller.RefreshTokenGVK, "rt-console") {
		t.Fatal("the grants went before the sign-ins in progress")
	}
	for range 2 {
		res, err := r.LogoutPair(t.Context(), jane, revocation.ReasonLock, "op-1")
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		if res.Generation != 3 {
			t.Errorf("generation = %d after the retry, want 3", res.Generation)
		}
	}
	if exists(t, c, controller.AuthCodeGVK, "code-a") || exists(t, c, controller.AuthRequestGVK, "req-a") {
		t.Error("the retry left a sign-in in progress")
	}
	if g := generationOf(t, c, naming.LocalConnectorID, "jane"); g != 3 {
		t.Errorf("stored generation = %d, want 3", g)
	}
}

// The sign-ins are not cached: a failure to list them through the API server fails the
// revocation, and it is retried, instead of leaving a code that could pass later.
func TestLogoutPair_FailsWhenTheSignInsCannotBeListed(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, janeObjects()...)
	forbidden := interceptor.NewClient(c, interceptor.Funcs{
		List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if list.GetObjectKind().GroupVersionKind().Kind == "AuthCodeList" {
				return apierrors.NewForbidden(schema.GroupResource{Group: "dex.coreos.com", Resource: "authcodes"}, "", nil)
			}
			return cl.List(ctx, list, opts...)
		},
	})
	_, err := newTestRevoker(forbidden).LogoutPair(t.Context(), jane, revocation.ReasonLock, "op-1")
	if !apierrors.IsForbidden(err) {
		t.Fatalf("LogoutPair error = %v, want the forbidden list", err)
	}
	if !exists(t, c, controller.AuthCodeGVK, "code-a") || !exists(t, c, controller.RefreshTokenGVK, "rt-console") {
		t.Error("the revocation went on past the failed list")
	}
}

// A sign-in replaces the reference between the read and the patch: the test of the JSON Patch fails
// and the new reference stays.
func TestLogoutPair_KeepsAReferenceReplacedInBetween(t *testing.T) {
	t.Parallel()

	var replaced atomic.Bool
	c := newTestClient(t, &calls{}, interceptor.Funcs{
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if kind(obj) != "OfflineSessions" || replaced.Swap(true) {
				return nil
			}
			current := controller.Object(controller.OfflineSessionsGVK)
			if err := cl.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: obj.GetName()}, current); err != nil {
				return err
			}
			if err := unstructured.SetNestedField(current.Object, "rt-fresh", "refresh", "console", "ID"); err != nil {
				return err
			}
			return cl.Update(ctx, current)
		},
	}, janeObjects()...)
	if _, err := newTestRevoker(c).LogoutPair(t.Context(), jane, revocation.ReasonLogout, ""); err != nil {
		t.Fatalf("LogoutPair: %v", err)
	}
	refs := refsOf(t, c, naming.LocalConnectorID, "jane")
	console, ok := refs["console"].(map[string]any)
	if !ok || console["ID"] != "rt-fresh" {
		t.Errorf("references = %v, want the fresh console reference kept", refs)
	}
	if _, ok := refs["kubeconfig-generator"]; ok {
		t.Errorf("references = %v, want the old kubeconfig reference removed", refs)
	}
}

// Dex raced the controller on the UserIdentity: the write is read again and the generation is
// raised by one once.
func TestLogoutPair_ConflictOnTheGeneration(t *testing.T) {
	t.Parallel()

	var conflicts atomic.Int32
	c := newTestClient(t, &calls{}, interceptor.Funcs{
		Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if kind(obj) == "UserIdentity" && conflicts.Add(1) == 1 {
				return apierrors.NewConflict(schema.GroupResource{Group: "dex.coreos.com", Resource: "useridentities"}, obj.GetName(), nil)
			}
			return cl.Update(ctx, obj, opts...)
		},
	}, janeObjects()...)
	res, err := newTestRevoker(c).LogoutPair(t.Context(), jane, revocation.ReasonLock, "")
	if err != nil {
		t.Fatalf("LogoutPair: %v", err)
	}
	if res.Generation != 3 || generationOf(t, c, naming.LocalConnectorID, "jane") != 3 {
		t.Errorf("generation = %d, want 3", res.Generation)
	}
	if conflicts.Load() != 2 {
		t.Errorf("updates = %d, want the conflict and one retry", conflicts.Load())
	}
}

// A1: a user of another provider with the same user ID is another pair and keeps everything.
func TestLogoutPair_TouchesThePairOnly(t *testing.T) {
	t.Parallel()

	ldapPair := naming.OfflineTokenName("jane", "corp-ldap")
	objs := append(janeObjects(),
		codeObj("code-ldap", "corp-ldap", "jane", 0),
		requestObj("req-ldap", "corp-ldap", "jane", 0),
		identityObj("corp-ldap", "jane", 5),
		offlineObj("corp-ldap", "jane", map[string][2]string{"console": {"rt-ldap", "sid-ldap"}}),
		tokenObj("rt-ldap", "corp-ldap", "jane", "console", 5),
		sessionObj("sid-ldap", "corp-ldap", "jane", 5, "console"),
	)
	c := newTestClient(t, &calls{}, interceptor.Funcs{}, objs...)
	if _, err := newTestRevoker(c).LogoutPair(t.Context(), jane, revocation.ReasonLock, ""); err != nil {
		t.Fatalf("LogoutPair: %v", err)
	}
	if g := generationOf(t, c, "corp-ldap", "jane"); g != 5 {
		t.Errorf("generation of the LDAP pair = %d, want 5", g)
	}
	if !exists(t, c, controller.RefreshTokenGVK, "rt-ldap") || !exists(t, c, controller.AuthSessionGVK, "sid-ldap") {
		t.Error("the LDAP pair lost its token or session")
	}
	if !exists(t, c, controller.AuthCodeGVK, "code-ldap") || !exists(t, c, controller.AuthRequestGVK, "req-ldap") {
		t.Error("the LDAP pair lost a sign-in in progress")
	}
	if refs := refsOf(t, c, "corp-ldap", "jane"); len(refs) != 1 {
		t.Errorf("LDAP references = %v", refs)
	}
	for _, rev := range listRevocations(t, c) {
		if rev.Pair() == ldapPair {
			t.Errorf("revocation %s names the LDAP pair", rev.Name)
		}
	}
}

func TestEscapePointer(t *testing.T) {
	t.Parallel()

	if got := escapePointer("a/b~c"); got != "a~1b~0c" {
		t.Errorf("escapePointer = %q", got)
	}
}

func equalRevocation(got, expected revocation.Revocation) bool {
	got.Created = expected.Created
	return got.Name == expected.Name &&
		got.Type == expected.Type &&
		got.ConnectorID == expected.ConnectorID &&
		got.UserID == expected.UserID &&
		got.Subject == expected.Subject &&
		got.Generation == expected.Generation &&
		got.SessionID == expected.SessionID &&
		got.GrantID == expected.GrantID &&
		got.ClientID == expected.ClientID &&
		slices.Equal(got.Clients, expected.Clients) &&
		got.Reason == expected.Reason &&
		got.Source == expected.Source &&
		got.CreatedAt.Equal(expected.CreatedAt) &&
		got.OperationUID == expected.OperationUID
}
