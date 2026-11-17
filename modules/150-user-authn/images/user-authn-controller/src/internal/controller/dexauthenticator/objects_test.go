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

package dexauthenticator

import (
	"reflect"
	"testing"
)

// TestClientContent_OneAddressPerDomain checks that the Dex client of an object names every
// address once: two applications of an object may publish one domain, each through its own
// Ingress class, and the address to return to is the same for both.
func TestClientContent_OneAddressPerDomain(t *testing.T) {
	t.Parallel()

	a := testAuthenticator("ns", "both", 0, "app.example.com", "other.example.com", "app.example.com")
	a.Applications[2].IngressClassName = "nginx-internal"

	tables := build(t, testInput(a))
	entry := tables.entries[a.Key()]
	if entry.Record == nil || entry.Reason != "" {
		t.Fatalf("the object is not served: %+v", entry)
	}

	content := clientContent(a, *entry.Record, testSettings, nil)
	for key, want := range map[string][]any{
		// In the order of the applications, the way the module chart wrote them.
		"redirectURIs":           {"https://app.example.com/dex-authenticator/callback", "https://other.example.com/dex-authenticator/callback"},
		"postLogoutRedirectURIs": {"https://app.example.com/", "https://other.example.com/"},
	} {
		if got := content[key]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}

	// Both applications keep their routes: only the addresses of the client are named once.
	if routes := desiredIngresses(a, entry.Record, nil, testSettings); len(routes) != 3 {
		t.Errorf("the object has %d Ingress objects, want one per application: 3", len(routes))
	}
}
