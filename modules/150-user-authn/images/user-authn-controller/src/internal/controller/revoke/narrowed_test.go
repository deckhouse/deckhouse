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
	"errors"
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
)

func TestLogoutSession(t *testing.T) {
	t.Parallel()

	t.Run("own session", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, janeObjects()...)
		res, err := newTestRevoker(c).LogoutSession(t.Context(), jane, "sid-a", "op-1")
		if err != nil {
			t.Fatalf("LogoutSession: %v", err)
		}
		if res.Name != revocation.SessionName("sid-a") || res.AuthSessionsDeleted != 1 || res.RefreshTokensDeleted != 1 {
			t.Errorf("result = %+v", res)
		}
		rev := getRevocation(t, c, revocation.SessionName("sid-a"))
		if rev.Type != revocation.TypeSession || rev.SessionID != "sid-a" || !slices.Equal(rev.Clients, []string{"console"}) ||
			rev.Reason != revocation.ReasonLogoutSession {
			t.Errorf("revocation = %+v", rev)
		}
		if exists(t, c, controller.AuthSessionGVK, "sid-a") || exists(t, c, controller.RefreshTokenGVK, "rt-console") {
			t.Error("the session or its grant is still there")
		}
		if !exists(t, c, controller.AuthSessionGVK, "sid-b") || !exists(t, c, controller.RefreshTokenGVK, "rt-kubeconfig") {
			t.Error("another session or a grant outside the session was deleted")
		}
		if g := generationOf(t, c, naming.LocalConnectorID, "jane"); g != 2 {
			t.Errorf("generation = %d, want it unchanged", g)
		}
	})

	for _, tt := range []struct {
		name string
		sid  string
	}{
		{name: "missing session", sid: "sid-missing"},
		{name: "session of another pair", sid: "sid-ldap"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newTestClient(t, &calls{}, interceptor.Funcs{},
				append(janeObjects(), sessionObj("sid-ldap", "corp-ldap", "jane", 0, "console"))...)
			_, err := newTestRevoker(c).LogoutSession(t.Context(), jane, tt.sid, "")
			if !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("err = %v, want ErrSessionNotFound", err)
			}
			if len(listRevocations(t, c)) != 0 || !exists(t, c, controller.AuthSessionGVK, "sid-ldap") {
				t.Error("a failed session logout wrote or deleted something")
			}
		})
	}
}

func TestLogoutExceptSession(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{},
		append(janeObjects(), tokenObj("rt-orphan", naming.LocalConnectorID, "jane", "grafana", 2))...)
	res, err := newTestRevoker(c).LogoutExceptSession(t.Context(), jane, "sid-b", "op-1")
	if err != nil {
		t.Fatalf("LogoutExceptSession: %v", err)
	}
	if res.AuthSessionsDeleted != 1 || res.RefreshTokensDeleted != 3 {
		t.Errorf("result = %+v, want sid-a and rt-console, rt-kubeconfig, rt-orphan", res)
	}
	if !exists(t, c, controller.AuthSessionGVK, "sid-b") || exists(t, c, controller.AuthSessionGVK, "sid-a") {
		t.Error("want sid-b kept and sid-a ended")
	}
	names := []string{}
	for _, rev := range listRevocations(t, c) {
		names = append(names, rev.Name)
	}
	slices.Sort(names)
	expected := []string{
		revocation.GrantName("rt-kubeconfig"),
		revocation.GrantName("rt-orphan"),
		revocation.SessionName("sid-a"),
	}
	if !slices.Equal(names, expected) {
		t.Errorf("revocations = %v, want %v", names, expected)
	}
	if refs := refsOf(t, c, naming.LocalConnectorID, "jane"); len(refs) != 0 {
		t.Errorf("references left: %v", refs)
	}

	t.Run("kept session of another pair", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, janeObjects()...)
		if _, err := newTestRevoker(c).LogoutExceptSession(t.Context(), jane, "sid-missing", ""); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("err = %v, want ErrSessionNotFound", err)
		}
	})
}

// A grant kept by the kept session survives; one issued under it by reference only.
func TestLogoutExceptSession_KeepsTheGrantsOfTheSession(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, janeObjects()...)
	if _, err := newTestRevoker(c).LogoutExceptSession(t.Context(), jane, "sid-a", ""); err != nil {
		t.Fatalf("LogoutExceptSession: %v", err)
	}
	if !exists(t, c, controller.RefreshTokenGVK, "rt-console") {
		t.Error("the grant of the kept session was deleted")
	}
	refs := refsOf(t, c, naming.LocalConnectorID, "jane")
	if _, ok := refs["console"]; !ok || len(refs) != 1 {
		t.Errorf("references = %v, want only the console one", refs)
	}
}

// One device of the console goes, the other device of the same client and the kubeconfig stay.
func TestLogoutGrant(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, devicesObjects()...)
	res, err := newTestRevoker(c).LogoutGrant(t.Context(), jane, "rt-laptop", "op-1")
	if err != nil {
		t.Fatalf("LogoutGrant: %v", err)
	}
	if res.RefreshTokensDeleted != 1 || exists(t, c, controller.RefreshTokenGVK, "rt-laptop") {
		t.Errorf("result = %+v, want rt-laptop deleted", res)
	}
	for _, kept := range []string{"rt-phone", "rt-kubeconfig"} {
		if !exists(t, c, controller.RefreshTokenGVK, kept) {
			t.Errorf("%s was deleted", kept)
		}
	}
	grants := grantsOfObj(t, c, naming.LocalConnectorID, "jane")
	if _, ok := grants["rt-laptop"]; ok || len(grants) != 2 {
		t.Errorf("grants = %v, want rt-phone and rt-kubeconfig", grants)
	}
	if refs := refsOf(t, c, naming.LocalConnectorID, "jane"); len(refs) != 2 {
		t.Errorf("refresh = %v, want console (rt-phone) and kubeconfig kept", refs)
	}
	if rev := getRevocation(t, c, revocation.GrantName("rt-laptop")); rev.Reason != revocation.ReasonLogoutGrant {
		t.Errorf("revocation = %+v", rev)
	}
	// The browser session of the laptop stays, without the console it was signed out of.
	obj := controller.Object(controller.AuthSessionGVK)
	if err := c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: "sid-laptop"}, obj); err != nil {
		t.Fatalf("get session sid-laptop: %v", err)
	}
	if states, _, _ := unstructured.NestedMap(obj.Object, "clientStates"); states["console"] != nil {
		t.Error("the session of the revoked grant still holds the console state")
	}

	t.Run("grant of another pair", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, devicesObjects()...)
		if _, err := newTestRevoker(c).LogoutGrant(t.Context(), jane, "rt-missing", ""); !errors.Is(err, ErrGrantNotFound) {
			t.Fatalf("err = %v, want ErrGrantNotFound", err)
		}
	})
}

// Signing out on the other devices from the phone: the laptop session and its grant, and the
// kubeconfig, go; the phone stays.
func TestLogoutExceptGrant(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, devicesObjects()...)
	res, err := newTestRevoker(c).LogoutExceptGrant(t.Context(), jane, "rt-phone", "op-1")
	if err != nil {
		t.Fatalf("LogoutExceptGrant: %v", err)
	}
	if res.AuthSessionsDeleted != 1 || res.RefreshTokensDeleted != 2 {
		t.Errorf("result = %+v, want sid-laptop, rt-laptop and rt-kubeconfig", res)
	}
	if !exists(t, c, controller.RefreshTokenGVK, "rt-phone") {
		t.Error("the kept grant was deleted")
	}
	grants := grantsOfObj(t, c, naming.LocalConnectorID, "jane")
	if _, ok := grants["rt-phone"]; !ok || len(grants) != 1 {
		t.Errorf("grants = %v, want only rt-phone", grants)
	}

	t.Run("kept grant of another pair", func(t *testing.T) {
		t.Parallel()
		c := newTestClient(t, &calls{}, interceptor.Funcs{}, devicesObjects()...)
		if _, err := newTestRevoker(c).LogoutExceptGrant(t.Context(), jane, "rt-missing", ""); !errors.Is(err, ErrGrantNotFound) {
			t.Fatalf("err = %v, want ErrGrantNotFound", err)
		}
	})
}

// The pair logout removes the references of grants too.
func TestLogoutPair_RemovesGrants(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, &calls{}, interceptor.Funcs{}, devicesObjects()...)
	if _, err := newTestRevoker(c).LogoutPair(t.Context(), jane, revocation.ReasonLogout, ""); err != nil {
		t.Fatalf("LogoutPair: %v", err)
	}
	if grants := grantsOfObj(t, c, naming.LocalConnectorID, "jane"); len(grants) != 0 {
		t.Errorf("grants left: %v", grants)
	}
	if refs := refsOf(t, c, naming.LocalConnectorID, "jane"); len(refs) != 0 {
		t.Errorf("references left: %v", refs)
	}
}
