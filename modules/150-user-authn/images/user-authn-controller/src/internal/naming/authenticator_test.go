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

package naming

import (
	"strings"
	"testing"
)

func TestAuthenticatorInstance(t *testing.T) {
	t.Parallel()

	for namespace, want := range map[string]string{
		"d8-console":    SystemAuthenticatorInstance,
		"d8-user-authn": SystemAuthenticatorInstance,
		"shop":          UserAuthenticatorInstance,
		"d8":            UserAuthenticatorInstance,
		"default":       UserAuthenticatorInstance,
		"kube-system":   UserAuthenticatorInstance,
		"":              UserAuthenticatorInstance,
	} {
		if got := AuthenticatorInstance(namespace); got != want {
			t.Errorf("AuthenticatorInstance(%q) = %q, want %q", namespace, got, want)
		}
	}
}

func TestDexAuthenticatorClientID(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, namespace, id string }{
		// The id the module chart passes to user-api for the console (template_tests pin the same value).
		{"console", "d8-console", "dex-authenticator-console@d8-console"},
		{"grafana", "d8-monitoring", "dex-authenticator-grafana@d8-monitoring"},
		// A dash in both parts: "a" in "b-c" and "a-b" in "c" are different clients.
		{"a", "b-c", "dex-authenticator-a@b-c"},
		{"a-b", "c", "dex-authenticator-a-b@c"},
		{"billing", "prod-team", "dex-authenticator-billing@prod-team"},
		{"billing-prod", "team", "dex-authenticator-billing-prod@team"},
	}
	for _, tt := range tests {
		if got := DexAuthenticatorClientID(tt.name, tt.namespace); got != tt.id {
			t.Errorf("DexAuthenticatorClientID(%q, %q) = %q, want %q", tt.name, tt.namespace, got, tt.id)
		}
		name, namespace, ok := ParseDexAuthenticatorClientID(tt.id)
		if !ok || name != tt.name || namespace != tt.namespace {
			t.Errorf("ParseDexAuthenticatorClientID(%q) = %q, %q, %v", tt.id, name, namespace, ok)
		}
	}

	for _, id := range []string{"", "kubernetes", "dex-client-app@shop", "dex-authenticator-@shop", "dex-authenticator-app@", "dex-authenticator-app"} {
		if _, _, ok := ParseDexAuthenticatorClientID(id); ok {
			t.Errorf("ParseDexAuthenticatorClientID(%q) accepted", id)
		}
	}
}

// TestSafeObjectName pins the names to the ones the module chart gave the objects: the values are
// the output of SafeDNS1123Name of the hook the chart took them from.
func TestSafeObjectName(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 60)
	tests := []struct {
		full      string
		name      string
		truncated bool
		hash      string
	}{
		{full: "grafana-dex-authenticator", name: "grafana-dex-authenticator"},
		{full: strings.Repeat("b", 63), name: strings.Repeat("b", 63)},
		{
			full:      long + "-dex-authenticator",
			name:      strings.Repeat("a", 57) + "-04e8b",
			truncated: true,
			hash:      "04e8b",
		},
		{
			// The cut falls on a dash, which a name cannot end with before the hash.
			full:      strings.Repeat("c", 56) + "-dex-authenticator",
			name:      strings.Repeat("c", 56) + "-c164f",
			truncated: true,
			hash:      "c164f",
		},
	}
	for _, tt := range tests {
		name, truncated, hash := SafeObjectName(tt.full)
		if name != tt.name || truncated != tt.truncated || hash != tt.hash {
			t.Errorf("SafeObjectName(%q) = %q, %v, %q; want %q, %v, %q", tt.full, name, truncated, hash, tt.name, tt.truncated, tt.hash)
		}
		if len(name) > 63 {
			t.Errorf("SafeObjectName(%q) is %d characters long", tt.full, len(name))
		}
	}
}
