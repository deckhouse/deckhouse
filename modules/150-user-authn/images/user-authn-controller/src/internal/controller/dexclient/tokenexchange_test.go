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

package dexclient

import (
	"slices"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	moscowUID = types.UID("0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8f9")
	berlinUID = types.UID("9f8e7d6c-5b4a-3928-1706-f5e4d3c2b1a0")
)

var testClientRef = types.NamespacedName{Namespace: testNamespace, Name: testName}

// tokenExchangeProvider is a TokenExchangeProvider that lists the DexClients refs as its clients,
// with its first connector ID in the status, as its controller writes it.
func tokenExchangeProvider(name string, uid types.UID, refs ...types.NamespacedName) *unstructured.Unstructured {
	u := controller.Object(controller.TokenExchangeProviderGVK)
	u.SetName(name)
	u.SetUID(uid)
	clients := make([]any, 0, len(refs))
	for _, ref := range refs {
		clients = append(clients, map[string]any{"dexClient": map[string]any{"name": ref.Name, "namespace": ref.Namespace}})
	}
	u.Object["spec"] = map[string]any{"clients": clients}
	setStatusConnectorID(u, naming.TokenExchangeConnectorID(name, string(uid), 0))
	return u
}

func setStatusConnectorID(u *unstructured.Unstructured, id string) {
	u.Object["status"] = map[string]any{"connectorID": id}
}

// Dex checks allowedConnectors of the client for a token exchange too, so a client that limits its
// providers must allow the connector of every TokenExchangeProvider that lists it.
func TestReconcile_RestrictedClientAllowsTheConnectorOfItsTokenExchangeProvider(t *testing.T) {
	t.Parallel()

	dc := dexClient(map[string]any{"allowedProviders": []any{"corp-ldap"}}, nil)
	tep := tokenExchangeProvider("moscow", moscowUID, testClientRef)
	r, c := newTestReconciler(t, dc, tep)

	if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	assertAllowedConnectors(t, c, []string{"corp-ldap", "tep-moscow-0a1b2c3d"})
}

func TestReconcile_TokenExchangeConnectors(t *testing.T) {
	t.Parallel()

	other := types.NamespacedName{Namespace: testNamespace, Name: "prometheus"}
	revoked := tokenExchangeProvider("moscow", moscowUID, testClientRef)
	setStatusConnectorID(revoked, naming.TokenExchangeConnectorID("moscow", string(moscowUID), 1))
	foreign := tokenExchangeProvider("moscow", moscowUID, testClientRef)
	setStatusConnectorID(foreign, "tep-berlin-9f8e7d6c")
	pending := tokenExchangeProvider("moscow", moscowUID, testClientRef)
	delete(pending.Object, "status")
	deleting := tokenExchangeProvider("berlin", berlinUID, testClientRef)
	deleting.SetFinalizers([]string{"user-authn.deckhouse.io/token-exchange-provider"})
	deleting.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})

	tests := []struct {
		name             string
		allowedProviders []any
		providers        []*unstructured.Unstructured
		// want is nil when the OAuth2Client must have no allowedConnectors.
		want []string
	}{
		{
			name:             "restricted client listed in one provider",
			allowedProviders: []any{"corp-ldap"},
			providers:        []*unstructured.Unstructured{tokenExchangeProvider("moscow", moscowUID, testClientRef)},
			want:             []string{"corp-ldap", "tep-moscow-0a1b2c3d"},
		},
		{
			name:             "restricted client listed in two providers",
			allowedProviders: []any{"local", "corp-ldap"},
			providers: []*unstructured.Unstructured{
				tokenExchangeProvider("moscow", moscowUID, testClientRef),
				tokenExchangeProvider("berlin", berlinUID, other, testClientRef),
			},
			want: []string{"local", "corp-ldap", "tep-berlin-9f8e7d6c", "tep-moscow-0a1b2c3d"},
		},
		{
			name:             "restricted client not listed",
			allowedProviders: []any{"corp-ldap"},
			providers:        []*unstructured.Unstructured{tokenExchangeProvider("moscow", moscowUID, other)},
			want:             []string{"corp-ldap"},
		},
		{
			name:      "open client listed stays open",
			providers: []*unstructured.Unstructured{tokenExchangeProvider("moscow", moscowUID, testClientRef)},
		},
		{
			name:             "hand-written connector ID is not repeated",
			allowedProviders: []any{"tep-moscow-0a1b2c3d", "corp-ldap"},
			providers:        []*unstructured.Unstructured{tokenExchangeProvider("moscow", moscowUID, testClientRef)},
			want:             []string{"tep-moscow-0a1b2c3d", "corp-ldap"},
		},
		{
			name:             "connector of the status after a revocation",
			allowedProviders: []any{"corp-ldap"},
			providers:        []*unstructured.Unstructured{revoked},
			want:             []string{"corp-ldap", naming.TokenExchangeConnectorID("moscow", string(moscowUID), 1)},
		},
		{
			name:             "connector of another provider in the status is ignored",
			allowedProviders: []any{"corp-ldap"},
			providers:        []*unstructured.Unstructured{foreign},
			want:             []string{"corp-ldap"},
		},
		{
			name:             "provider without a connector in the status adds nothing",
			allowedProviders: []any{"corp-ldap"},
			providers:        []*unstructured.Unstructured{pending},
			want:             []string{"corp-ldap"},
		},
		{
			name:             "provider being deleted is left out",
			allowedProviders: []any{"corp-ldap"},
			providers:        []*unstructured.Unstructured{deleting},
			want:             []string{"corp-ldap"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			spec := map[string]any{}
			if tt.allowedProviders != nil {
				spec["allowedProviders"] = tt.allowedProviders
			}
			objs := []client.Object{dexClient(spec, nil)}
			for _, p := range tt.providers {
				objs = append(objs, p.DeepCopy())
			}
			r, c := newTestReconciler(t, objs...)

			if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			assertAllowedConnectors(t, c, tt.want)
		})
	}
}

// The list follows the providers: a provider deleted, created again under the same name, or that
// drops the client from spec.clients takes its connector away, and a second reconcile changes nothing.
func TestReconcile_TokenExchangeConnectorsFollowTheProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change func(t *testing.T, c client.Client)
		want   []string
		// unchanged is true when the OAuth2Client must not be written again.
		unchanged bool
	}{
		{
			name: "provider deleted",
			change: func(t *testing.T, c client.Client) {
				t.Helper()
				if err := c.Delete(t.Context(), tokenExchangeProvider("moscow", moscowUID)); err != nil {
					t.Fatalf("delete provider: %v", err)
				}
			},
			want: []string{"corp-ldap"},
		},
		{
			name: "provider recreated under the same name",
			change: func(t *testing.T, c client.Client) {
				t.Helper()
				if err := c.Delete(t.Context(), tokenExchangeProvider("moscow", moscowUID)); err != nil {
					t.Fatalf("delete provider: %v", err)
				}
				if err := c.Create(t.Context(), tokenExchangeProvider("moscow", berlinUID, testClientRef)); err != nil {
					t.Fatalf("create provider: %v", err)
				}
			},
			want: []string{"corp-ldap", "tep-moscow-9f8e7d6c"},
		},
		{
			name: "connector ID changed in the status",
			change: func(t *testing.T, c client.Client) {
				t.Helper()
				provider := controller.Object(controller.TokenExchangeProviderGVK)
				if err := c.Get(t.Context(), types.NamespacedName{Name: "moscow"}, provider); err != nil {
					t.Fatalf("get provider: %v", err)
				}
				setStatusConnectorID(provider, naming.TokenExchangeConnectorID("moscow", string(moscowUID), 1))
				if err := c.Update(t.Context(), provider); err != nil {
					t.Fatalf("update provider: %v", err)
				}
			},
			want: []string{"corp-ldap", naming.TokenExchangeConnectorID("moscow", string(moscowUID), 1)},
		},
		{
			name: "client removed from spec.clients",
			change: func(t *testing.T, c client.Client) {
				t.Helper()
				provider := controller.Object(controller.TokenExchangeProviderGVK)
				if err := c.Get(t.Context(), types.NamespacedName{Name: "moscow"}, provider); err != nil {
					t.Fatalf("get provider: %v", err)
				}
				provider.Object["spec"] = tokenExchangeProvider("moscow", moscowUID,
					types.NamespacedName{Namespace: testNamespace, Name: "prometheus"}).Object["spec"]
				if err := c.Update(t.Context(), provider); err != nil {
					t.Fatalf("update provider: %v", err)
				}
			},
			want: []string{"corp-ldap"},
		},
		{
			name:      "second reconcile",
			change:    func(*testing.T, client.Client) {},
			want:      []string{"corp-ldap", "tep-moscow-0a1b2c3d"},
			unchanged: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dc := dexClient(map[string]any{"allowedProviders": []any{"corp-ldap"}}, nil)
			r, c := newTestReconciler(t, dc, tokenExchangeProvider("moscow", moscowUID, testClientRef))
			if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
				t.Fatalf("first Reconcile: %v", err)
			}
			before := getOAuth2Client(t, c, ClientID(testName, testNamespace)).GetResourceVersion()

			tt.change(t, c)
			if _, err := r.Reconcile(t.Context(), testRequest); err != nil {
				t.Fatalf("second Reconcile: %v", err)
			}
			assertAllowedConnectors(t, c, tt.want)
			after := getOAuth2Client(t, c, ClientID(testName, testNamespace)).GetResourceVersion()
			if (before == after) != tt.unchanged {
				t.Errorf("OAuth2Client resourceVersion %s -> %s, want it written again only when the list changed", before, after)
			}
		})
	}
}

func TestMapTokenExchangeProvider(t *testing.T) {
	t.Parallel()

	other := types.NamespacedName{Namespace: "apps", Name: "argocd"}
	got := mapTokenExchangeProvider(t.Context(), tokenExchangeProvider("moscow", moscowUID, testClientRef, other))
	want := []reconcile.Request{{NamespacedName: testClientRef}, {NamespacedName: other}}
	if !slices.Equal(got, want) {
		t.Errorf("mapTokenExchangeProvider = %v, want %v", got, want)
	}
	if got := mapTokenExchangeProvider(t.Context(), tokenExchangeProvider("empty", moscowUID)); len(got) != 0 {
		t.Errorf("mapTokenExchangeProvider of a provider without clients = %v, want none", got)
	}
}

func assertAllowedConnectors(t *testing.T, c client.Client, want []string) {
	t.Helper()
	for _, id := range []string{ClientID(testName, testNamespace), LegacyClientID(testName, testNamespace)} {
		obj := getOAuth2Client(t, c, id)
		got, found, _ := unstructured.NestedStringSlice(obj.Object, "allowedConnectors")
		if want == nil {
			if found {
				t.Errorf("%s allowedConnectors = %v, want none: the client allows every connector", id, got)
			}
			continue
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s allowedConnectors = %v, want %v", id, got, want)
		}
	}
}
