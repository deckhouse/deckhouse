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
	"testing"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// officeObjects is the console and the kubeconfig generator, each with tokens of two users of one provider and of a
// local user.
func officeObjects() []client.Object {
	return []client.Object{
		offlineObj("ldap", "ann", map[string][2]string{"console": {"rt-ann-console", ""}, "kubeconfig-generator": {"rt-ann-kube", ""}}),
		tokenObj("rt-ann-console", "ldap", "ann", "console", 0),
		tokenObj("rt-ann-kube", "ldap", "ann", "kubeconfig-generator", 0),
		offlineObj("oidc", "bob", map[string][2]string{"console": {"rt-bob-console", ""}}),
		tokenObj("rt-bob-console", "oidc", "bob", "console", 0),
		offlineObj(naming.LocalConnectorID, "jane", map[string][2]string{"console": {"rt-jane-console", ""}}),
		tokenObj("rt-jane-console", naming.LocalConnectorID, "jane", "console", 0),
		sessionObj("sid-ann", "ldap", "ann", 0, "console"),
	}
}

func TestDeleteClientTokensOutside_DeletesTheTokensOfTheClientOutsideTheList(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, officeObjects()...)
	deleted, err := newTestRevoker(c).DeleteClientTokensOutside(t.Context(), "console", []string{"local", "ldap"})
	if err != nil {
		t.Fatalf("DeleteClientTokensOutside: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted %d tokens, want 1", deleted)
	}
	if exists(t, c, controller.RefreshTokenGVK, "rt-bob-console") {
		t.Error("the token of the console through the provider outside the list is still there")
	}
	for _, name := range []string{"rt-ann-console", "rt-ann-kube", "rt-jane-console"} {
		if !exists(t, c, controller.RefreshTokenGVK, name) {
			t.Errorf("the token %s was deleted", name)
		}
	}
	if refresh := refreshOfObj(t, c, "oidc", "bob"); len(refresh) != 0 {
		t.Errorf("the OfflineSessions of bob still refer to %v", refresh)
	}
	if refresh := refreshOfObj(t, c, "ldap", "ann"); len(refresh) != 2 {
		t.Errorf("the OfflineSessions of ann lost a reference: %v", refresh)
	}
}

func TestDeleteClientTokensOutside_LeavesOtherClientsAlone(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, officeObjects()...)
	deleted, err := newTestRevoker(c).DeleteClientTokensOutside(t.Context(), "kubeconfig-generator", []string{"oidc"})
	if err != nil {
		t.Fatalf("DeleteClientTokensOutside: %v", err)
	}
	if deleted != 1 || exists(t, c, controller.RefreshTokenGVK, "rt-ann-kube") {
		t.Fatalf("the token of the kubeconfig generator through ldap is still there (deleted %d)", deleted)
	}
	for _, name := range []string{"rt-ann-console", "rt-bob-console", "rt-jane-console"} {
		if !exists(t, c, controller.RefreshTokenGVK, name) {
			t.Errorf("the token %s of another client was deleted", name)
		}
	}
}

func TestDeleteClientTokensOutside_AnEmptyListAllowsEveryConnector(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, officeObjects()...)
	deleted, err := newTestRevoker(c).DeleteClientTokensOutside(t.Context(), "console", nil)
	if err != nil || deleted != 0 {
		t.Fatalf("deleted %d, error %v", deleted, err)
	}
	if !exists(t, c, controller.RefreshTokenGVK, "rt-bob-console") {
		t.Error("a token was deleted for an empty list")
	}
}

func TestDeleteClientTokensOutside_CanBeRepeatedAndKeepsTheSessions(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, officeObjects()...)
	r := newTestRevoker(c)
	if _, err := r.DeleteClientTokensOutside(t.Context(), "console", []string{"oidc"}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	deleted, err := r.DeleteClientTokensOutside(t.Context(), "console", []string{"oidc"})
	if err != nil || deleted != 0 {
		t.Fatalf("second run deleted %d, error %v", deleted, err)
	}
	obj := controller.Object(controller.AuthSessionGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: "sid-ann"}, obj); err != nil {
		t.Fatalf("the browser session is gone: %v", err)
	}
	_ = obj
}
