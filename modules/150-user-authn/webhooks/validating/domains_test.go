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

package validating_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// An application published only through Gateway API is checked like any other: it used to fall out
// of the loop that rejects an IP address in place of a domain.
func TestDomainValidationWithoutIngressClass(t *testing.T) {
	for _, version := range hookVersions {
		for _, domain := range []string{"app.example.com", "192.0.2.1", "2001:db8::1"} {
			for _, additional := range []bool{false, true} {
				app := map[string]any{"domain": domain, "gatewayAPI": listenerSetReference("app", "", "")}
				object := withApplication(version, "test", "test", app, additional)

				result := runHook(t, review(object))
				require.Equal(t, domain == "app.example.com", result["allowed"],
					"%s %s additional=%v", version, domain, additional)
			}
		}
	}
}

func TestIngressConflict(t *testing.T) {
	existing := map[string]any{"filterResult": map[string]any{
		"name": "existing", "namespace": "test",
		"applicationDomain": "app.example.com", "ingressClass": "nginx",
		"additionalDomains": []any{},
	}}

	for _, version := range hookVersions {
		t.Run(version, func(t *testing.T) {
			object := authenticator(version, "test", "test",
				map[string]any{"domain": "app.example.com", "ingressClassName": "nginx"})

			result := runHook(t, review(object, existing))
			require.Equal(t, false, result["allowed"])
			require.Contains(t, result["message"], "conflicts")
		})
	}
}

// Applications after the first take a resource-name suffix derived from sha256(domain), so two of
// them sharing a domain would name two objects of one kind alike. The first application carries no
// suffix, and an Ingress may share a name with an HTTPRoute, so neither of those repeats collides.
func TestDuplicateDomainWithinObject(t *testing.T) {
	gateway := listenerSetReference("app-listeners", "", "")
	nginx := func(domain string) any {
		return map[string]any{"domain": domain, "ingressClassName": "nginx"}
	}
	route := func(domain string) any {
		return map[string]any{"domain": domain, "gatewayAPI": gateway}
	}

	for _, tc := range []struct {
		name      string
		apps      []any
		duplicate string
	}{
		{"distinct domains", []any{nginx("first.example.com"), nginx("second.example.com")}, ""},
		// The first application has no name suffix, so these render as two distinct objects.
		{"first application repeated, same class", []any{nginx("app.example.com"), nginx("app.example.com")}, ""},
		{"first application repeated, another class", []any{
			nginx("app.example.com"),
			map[string]any{"domain": "app.example.com", "ingressClassName": "nginx-internal"},
		}, ""},
		{"first application repeated through Gateway API", []any{nginx("app.example.com"), route("app.example.com")}, ""},
		// One names an Ingress, the other an HTTPRoute: different kinds may share a name.
		{"additional applications split across publication paths", []any{
			nginx("first.example.com"), nginx("same.example.com"), route("same.example.com"),
		}, ""},
		// Both suffixes are sha256("same.example.com") and both name an Ingress.
		{"two additional applications share a domain", []any{
			nginx("first.example.com"), nginx("same.example.com"), nginx("same.example.com"),
		}, "Ingress"},
		{"colliding additional applications differ in class", []any{
			nginx("first.example.com"),
			nginx("same.example.com"),
			map[string]any{"domain": "same.example.com", "ingressClassName": "nginx-internal"},
		}, "Ingress"},
		{"two additional applications share a domain through Gateway API", []any{
			nginx("first.example.com"), route("same.example.com"), route("same.example.com"),
		}, "HTTPRoute"},
		// Publishing both ways collides on whichever kind the earlier application already named.
		{"additional application repeats both publication paths", []any{
			nginx("first.example.com"),
			map[string]any{"domain": "same.example.com", "ingressClassName": "nginx", "gatewayAPI": gateway},
			route("same.example.com"),
		}, "HTTPRoute"},
	} {
		for _, version := range hookVersions {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				object := authenticator(version, "test", "test", tc.apps...)

				result := runHook(t, review(object))
				require.Equal(t, tc.duplicate == "", result["allowed"], "%v", result["message"])
				if tc.duplicate != "" {
					require.Contains(t, result["message"], "repeats domain 'same.example.com'")
					require.Contains(t, result["message"], "a second "+tc.duplicate)
				}
			})
		}
	}
}
