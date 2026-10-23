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
