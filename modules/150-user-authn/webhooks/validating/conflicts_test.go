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

// The hook tells the two shapes apart by apiVersion alone, so the versions worth covering are
// v2alpha1 and one of the others.
var hookVersions = []string{"v1", "v2alpha1"}

var applicationPositions = []struct {
	name                                   string
	existingAdditional, incomingAdditional bool
}{
	{"primary-primary", false, false}, {"primary-additional", false, true},
	{"additional-primary", true, false}, {"additional-additional", true, true},
}

func TestGatewayConflicts(t *testing.T) {
	shared := listenerSetReference("shared", "d8-alb", "https")
	for _, tc := range []struct {
		name                                 string
		existingRef, incomingRef             map[string]any
		existingNamespace, incomingNamespace string
		incomingDomain                       string
		allowed                              bool
	}{
		{"shared ListenerSet across namespaces", shared, shared, "team-a", "team-b", "app.example.com", false},
		{"different domain", shared, shared, "team-a", "team-b", "other.example.com", true},
		{"different ListenerSet", shared, listenerSetReference("other", "d8-alb", "https"), "team-a", "team-b", "app.example.com", true},
		{"different ListenerSet namespace", shared, listenerSetReference("shared", "other", "https"), "team-a", "team-b", "app.example.com", true},
		{"different section", shared, listenerSetReference("shared", "d8-alb", "other"), "team-a", "team-b", "app.example.com", true},
		{"existing all sections", listenerSetReference("shared", "d8-alb", ""), shared, "team-a", "team-b", "app.example.com", false},
		{"incoming all sections", shared, listenerSetReference("shared", "d8-alb", ""), "team-a", "team-b", "app.example.com", false},
		{"both all sections", listenerSetReference("shared", "d8-alb", ""), listenerSetReference("shared", "d8-alb", ""), "team-a", "team-b", "app.example.com", false},
		{"existing default namespace", listenerSetReference("shared", "", "https"), listenerSetReference("shared", "team-a", "https"), "team-a", "team-b", "app.example.com", false},
		{"incoming default namespace", listenerSetReference("shared", "team-b", "https"), listenerSetReference("shared", "", "https"), "team-a", "team-b", "app.example.com", false},
		{"same default namespace", listenerSetReference("shared", "", "https"), listenerSetReference("shared", "", "https"), "team-a", "team-a", "app.example.com", false},
		{"different default namespaces", listenerSetReference("shared", "", "https"), listenerSetReference("shared", "", "https"), "team-a", "team-b", "app.example.com", true},
	} {
		for _, version := range hookVersions {
			for _, position := range applicationPositions {
				t.Run(version+"/"+position.name+"/"+tc.name, func(t *testing.T) {
					existing := withApplication("v1", tc.existingNamespace, "test",
						map[string]any{"domain": "app.example.com", "gatewayAPI": tc.existingRef}, position.existingAdditional)
					incomingApp := map[string]any{"domain": tc.incomingDomain, "gatewayAPI": tc.incomingRef}
					incoming := withApplication(version, tc.incomingNamespace, "new", incomingApp, position.incomingAdditional)

					result := runHook(t, review(incoming, snapshot(t, existing)))
					require.Equal(t, tc.allowed, result["allowed"])
					if !tc.allowed {
						require.Contains(t, result["message"], "conflicts")
						require.Contains(t, result["message"], tc.existingNamespace+"/test")
						require.Contains(t, result["message"], "app.example.com")
						require.Contains(t, result["message"], "ListenerSet")
					}
				})
			}
		}
	}
}

// An Ingress conflict and a ListenerSet conflict are reported separately, so the message says
// which one it is.
func TestGatewayConflictWithIngress(t *testing.T) {
	for _, classes := range []struct{ name, existing, incoming string }{
		{"existing mixed", "nginx", ""}, {"incoming mixed", "", "nginx"}, {"both mixed", "first", "second"},
		{"ingress conflict with distinct ListenerSets", "nginx", "nginx"},
	} {
		t.Run(classes.name, func(t *testing.T) {
			app := func(class string) map[string]any {
				fields := map[string]any{"domain": "app.example.com", "gatewayAPI": listenerSetReference("shared", "d8-alb", "https")}
				if class != "" {
					fields["ingressClassName"] = class
				}

				return fields
			}

			ingressConflict := classes.existing != "" && classes.existing == classes.incoming
			incomingApp := app(classes.incoming)
			if ingressConflict {
				incomingApp["gatewayAPI"] = listenerSetReference("other", "d8-alb", "https")
			}
			existing := withApplication("v1", "team-a", "test", app(classes.existing), false)
			incoming := withApplication("v2alpha1", "team-b", "test", incomingApp, false)

			result := runHook(t, review(incoming, snapshot(t, existing)))
			require.Equal(t, false, result["allowed"])
			if ingressConflict {
				require.Contains(t, result["message"], "conflicts")
				require.NotContains(t, result["message"], "ListenerSet")
			} else {
				require.Equal(t, "Desired DexAuthenticator 'team-b/test' conflicts with the existing DexAuthenticator "+
					"'team-a/test' for domain 'app.example.com' on ListenerSet 'd8-alb/shared'", result["message"])
			}
		})
	}
}

// An authenticator must not conflict with itself, or no one could ever edit one.
func TestConflictOnUpdate(t *testing.T) {
	for _, mode := range []string{"ingress", "gateway"} {
		for _, conflict := range []bool{false, true} {
			name := "self"
			if conflict {
				name = "other"
			}
			t.Run(mode+"/"+name, func(t *testing.T) {
				app := map[string]any{"domain": "app.example.com"}
				if mode == "ingress" {
					app["ingressClassName"] = "nginx"
				} else {
					app["gatewayAPI"] = listenerSetReference("shared", "d8-alb", "https")
				}

				incoming := withApplication("v1", "team-a", "test", app, false)
				snapshots := []any{snapshot(t, incoming)}
				if conflict {
					snapshots = append(snapshots, snapshot(t, withApplication("v1", "team-b", "test", app, false)))
				}

				result := runHook(t, review(incoming, snapshots...))
				require.Equal(t, !conflict, result["allowed"])
			})
		}
	}
}

// A shared domain alone does not imply a conflict between Ingress and Gateway API.
func TestSameDomainAcrossRoutingModes(t *testing.T) {
	for _, version := range hookVersions {
		for _, position := range applicationPositions {
			for _, existingMode := range []string{"ingress", "gateway"} {
				t.Run(version+"/"+position.name+"/existing-"+existingMode, func(t *testing.T) {
					existingApp := map[string]any{"domain": "app.example.com", "ingressClassName": "nginx"}
					incomingApp := map[string]any{"domain": "app.example.com", "gatewayAPI": listenerSetReference("shared", "d8-alb", "https")}
					if existingMode == "gateway" {
						existingApp, incomingApp = incomingApp, existingApp
					}

					existing := withApplication("v1", "team-a", "test", existingApp, position.existingAdditional)
					incoming := withApplication(version, "team-b", "new", incomingApp, position.incomingAdditional)

					result := runHook(t, review(incoming, snapshot(t, existing)))
					require.Equal(t, true, result["allowed"])
				})
			}
		}
	}
}

// Pairs sharing a domain could be created before these checks existed. Refusing every edit to them
// would leave them stuck, so only what the object did not already claim is checked.
func TestConflictIsNotRecheckedOnUpdate(t *testing.T) {
	for _, mode := range []string{"ingress", "gateway"} {
		claim := func(domain string) map[string]any {
			app := map[string]any{"domain": domain}
			if mode == "ingress" {
				app["ingressClassName"] = "nginx"
			} else {
				app["gatewayAPI"] = listenerSetReference("shared", "d8-alb", "https")
			}

			return app
		}
		existing := snapshot(t, withApplication("v1", "team-a", "first", claim("app.example.com"), false))

		for _, tc := range []struct {
			name             string
			oldDomain        string
			deleting, denied bool
		}{
			{name: "claim carried over from the old object", oldDomain: "app.example.com"},
			{name: "claim newly pointed at the taken domain", oldDomain: "other.example.com", denied: true},
			{name: "object being deleted", oldDomain: "other.example.com", deleting: true},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				object := withApplication("v1", "team-b", "second", claim("app.example.com"), false)
				old := withApplication("v1", "team-b", "second", claim(tc.oldDomain), false)
				if tc.deleting {
					object["metadata"].(map[string]any)["deletionTimestamp"] = "2026-09-25T10:00:00Z"
				}

				result := runHook(t, map[string]any{
					"review": map[string]any{"request": map[string]any{
						"operation": "UPDATE", "object": object, "oldObject": old,
					}},
					"snapshots": map[string]any{"dexauthenticators": []any{existing}},
				})
				require.Equal(t, !tc.denied, result["allowed"], "%v", result["message"])
			})
		}

		// Without an old object the claim is new, whatever it repeats.
		t.Run(mode+"/created rather than updated", func(t *testing.T) {
			object := withApplication("v1", "team-b", "second", claim("app.example.com"), false)
			result := runHook(t, review(object, existing))
			require.Equal(t, false, result["allowed"])
		})
	}
}
