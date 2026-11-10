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
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// corpPairObjects is a user of a provider with a token for two clients, a browser session and the OfflineSessions, next
// to another user of the provider and a user of another provider.
func corpPairObjects() []client.Object {
	return []client.Object{
		offlineObj(corp, "ann", map[string][2]string{"console": {"rt-ann-console", "sid-ann"}, "kubeconfig-generator": {"rt-ann-kube", ""}}),
		tokenObj("rt-ann-console", corp, "ann", "console", 0),
		tokenObj("rt-ann-kube", corp, "ann", "kubeconfig-generator", 0),
		sessionObj("sid-ann", corp, "ann", 0, "console"),
		identityObj(corp, "ann", 3),
		offlineObj(corp, "bob", map[string][2]string{"console": {"rt-bob-console", ""}}),
		tokenObj("rt-bob-console", corp, "bob", "console", 0),
		sessionObj("sid-bob", corp, "bob", 0, "console"),
		offlineObj("other", "ann", map[string][2]string{"console": {"rt-ann-other", ""}}),
		tokenObj("rt-ann-other", "other", "ann", "console", 0),
	}
}

func TestDeletePair_DeletesWhatDexKeepsOfThePairExceptItsIdentity(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, corpPairObjects()...)
	got, err := newTestRevoker(c).DeletePair(t.Context(), "ann", corp)
	if err != nil {
		t.Fatalf("DeletePair: %v", err)
	}
	if got.RefreshTokens != 2 || got.AuthSessions != 1 || !got.OfflineSessions {
		t.Fatalf("deleted %+v, want two tokens, a session and the OfflineSessions", got)
	}

	gone := []struct {
		gvk  string
		name string
	}{
		{"RefreshToken", "rt-ann-console"}, {"RefreshToken", "rt-ann-kube"}, {"AuthSession", "sid-ann"},
	}
	for _, g := range gone {
		gvk := controller.RefreshTokenGVK
		if g.gvk == "AuthSession" {
			gvk = controller.AuthSessionGVK
		}
		if exists(t, c, gvk, g.name) {
			t.Errorf("%s %s is still there", g.gvk, g.name)
		}
	}
	if exists(t, c, controller.OfflineSessionsGVK, naming.OfflineTokenName("ann", corp)) {
		t.Error("the OfflineSessions of the pair is still there")
	}

	// The identity is the caller's to delete, last; the other pairs are not touched.
	if !exists(t, c, controller.UserIdentityGVK, naming.OfflineTokenName("ann", corp)) {
		t.Error("the identity of the pair was deleted")
	}
	for _, name := range []string{"rt-bob-console", "rt-ann-other"} {
		if !exists(t, c, controller.RefreshTokenGVK, name) {
			t.Errorf("the token %s of another pair was deleted", name)
		}
	}
	if !exists(t, c, controller.AuthSessionGVK, "sid-bob") {
		t.Error("the session of another user was deleted")
	}
}

func TestDeletePair_WritesNoRevocationAndRaisesNoGeneration(t *testing.T) {
	t.Parallel()

	rec := &calls{}
	c := newTestClient(t, rec, interceptor.Funcs{}, corpPairObjects()...)
	if _, err := newTestRevoker(c).DeletePair(t.Context(), "ann", corp); err != nil {
		t.Fatalf("DeletePair: %v", err)
	}
	for _, call := range rec.list() {
		if call[:6] == "create" || call[:5] == "patch" || call[:6] == "update" {
			t.Errorf("a write besides the deletions: %s", call)
		}
	}
}

func TestDeletePair_CanBeRepeatedAndAnEmptyPairIsNotAnError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, corpPairObjects()...)
	r := newTestRevoker(c)
	if _, err := r.DeletePair(t.Context(), "ann", corp); err != nil {
		t.Fatal(err)
	}
	got, err := r.DeletePair(t.Context(), "ann", corp)
	if err != nil || got.RefreshTokens != 0 || got.AuthSessions != 0 || got.OfflineSessions {
		t.Fatalf("second run: %+v, %v", got, err)
	}
	if _, err := r.DeletePair(t.Context(), "nobody", corp); err != nil {
		t.Fatalf("a pair that holds nothing: %v", err)
	}
}

// The name of a pair hashes the user and the connector without a separator, so ("ann", "corp") and ("annc", "orp")
// share it. DeletePair of one leaves the tokens, the sessions and the OfflineSessions of the other.
func TestDeletePair_LeavesAnotherPairWithTheSameName(t *testing.T) {
	t.Parallel()

	if naming.OfflineTokenName("ann", "corp") != naming.OfflineTokenName("annc", "orp") {
		t.Skip("the two pairs no longer share a name")
	}
	c := newTestClient(t, &calls{}, interceptor.Funcs{},
		offlineObj("orp", "annc", map[string][2]string{"console": {"rt-annc", ""}}),
		tokenObj("rt-annc", "orp", "annc", "console", 0),
		sessionObj("sid-annc", "orp", "annc", 0, "console"),
		tokenObj("rt-ann", "corp", "ann", "console", 0),
	)
	got, err := newTestRevoker(c).DeletePair(t.Context(), "ann", "corp")
	if err != nil {
		t.Fatalf("DeletePair: %v", err)
	}
	if got.RefreshTokens != 1 || got.AuthSessions != 0 || got.OfflineSessions {
		t.Fatalf("deleted %+v, want only the token of ann@corp", got)
	}
	if !exists(t, c, controller.RefreshTokenGVK, "rt-annc") || !exists(t, c, controller.AuthSessionGVK, "sid-annc") {
		t.Error("a token or a session of annc@orp was deleted")
	}
	if !exists(t, c, controller.OfflineSessionsGVK, naming.OfflineTokenName("annc", "orp")) {
		t.Error("the OfflineSessions of annc@orp was deleted")
	}
}

// A conflict means a sign-in wrote the OfflineSessions between the read and the delete. DeletePair reports it
// instead of success: the cleanup then leaves the UserIdentity, and the next pass decides again.
func TestDeletePair_ConflictOnOfflineSessionsIsAnError(t *testing.T) {
	t.Parallel()

	name := naming.OfflineTokenName("ann", corp)
	c := newTestClient(t, &calls{}, interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if obj.GetName() == name {
				return apierrors.NewConflict(schema.GroupResource{Group: "dex.coreos.com", Resource: "offlinesessions"}, name, fmt.Errorf("resource version changed"))
			}
			return cl.Delete(ctx, obj, opts...)
		},
	}, corpPairObjects()...)

	_, err := newTestRevoker(c).DeletePair(t.Context(), "ann", corp)
	if err == nil || !apierrors.IsConflict(err) {
		t.Fatalf("DeletePair: got %v, want a conflict", err)
	}
	if !exists(t, c, controller.OfflineSessionsGVK, name) {
		t.Error("the OfflineSessions was deleted despite the conflict")
	}
}
