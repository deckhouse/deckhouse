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

package crds_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func authenticatorSnapshot(t *testing.T, object map[string]any) map[string]any {
	t.Helper()
	// The watch uses v1, even when the user submitted a v2alpha1 resource.
	if object["apiVersion"] == "deckhouse.io/v2alpha1" {
		object = convertAuthenticator(t, object, "v2alpha1", "v1")
	}
	script, err := os.ReadFile("../webhooks/validating/dex_authenticator")
	require.NoError(t, err)
	source := strings.Replace(string(script), "source /shell_lib.sh", "function hook::run() { :; }", 1)
	configYAML, err := exec.Command("bash", "-e", "-c", source+"\n__config__").CombinedOutput()
	require.NoError(t, err, "%s", configYAML)
	var config struct {
		Kubernetes []struct {
			JQFilter string `json:"jqFilter"`
		} `json:"kubernetes"`
	}
	require.NoError(t, yaml.Unmarshal(configYAML, &config))
	require.Len(t, config.Kubernetes, 1)
	require.NotEmpty(t, config.Kubernetes[0].JQFilter)
	input, err := json.Marshal(object)
	require.NoError(t, err)
	cmd := exec.Command("jq", config.Kubernetes[0].JQFilter)
	cmd.Stdin = strings.NewReader(string(input))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	var filtered map[string]any
	require.NoError(t, json.Unmarshal(output, &filtered))
	return map[string]any{"filterResult": filtered}
}

func listenerSetReference(name, namespace, section string) map[string]any {
	ref := map[string]any{"httpRouteListenerSetName": name}
	if namespace != "" {
		ref["httpRouteListenerSetNamespace"] = namespace
	}
	if section != "" {
		ref["httpRouteListenerSetSectionName"] = section
	}
	return ref
}

func authenticatorWithApplication(namespace string, app map[string]any, additional bool) map[string]any {
	apps := []any{app}
	if additional {
		apps = append([]any{map[string]any{"domain": "unrelated.example.com", "ingressClassName": namespace}}, apps...)
	}
	object := authenticatorObject("v2alpha1", apps)
	object["metadata"].(map[string]any)["namespace"] = namespace
	return object
}

func TestDexAuthenticatorGatewayConflicts(t *testing.T) {
	shared := listenerSetReference("shared", "d8-alb", "https")
	cases := []struct {
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
	}
	for _, version := range []string{"v1", "v1alpha1", "v2alpha1"} {
		for _, location := range []struct {
			name                                   string
			existingAdditional, incomingAdditional bool
		}{
			{"primary-primary", false, false}, {"primary-additional", false, true},
			{"additional-primary", true, false}, {"additional-additional", true, true},
		} {
			for _, tc := range cases {
				t.Run(version+"/"+location.name+"/"+tc.name, func(t *testing.T) {
					existing := authenticatorWithApplication(tc.existingNamespace, map[string]any{"domain": "app.example.com", "gatewayAPI": tc.existingRef}, location.existingAdditional)
					incoming := authenticatorWithApplication(tc.incomingNamespace, map[string]any{"domain": tc.incomingDomain, "gatewayAPI": tc.incomingRef}, location.incomingAdditional)
					incoming["metadata"].(map[string]any)["name"] = "new"
					if location.incomingAdditional {
						incoming["spec"].(map[string]any)["applications"].([]any)[0].(map[string]any)["domain"] = "incoming.example.com"
					}
					if version != "v2alpha1" {
						incoming = convertAuthenticator(t, incoming, "v2alpha1", "v1")
						incoming["apiVersion"] = "deckhouse.io/" + version
					}
					result := runAuthenticatorHook(t, "validating/dex_authenticator", "__main__", map[string]any{
						"review":    map[string]any{"request": map[string]any{"operation": "CREATE", "object": incoming}},
						"snapshots": map[string]any{"dexauthenticators": []any{authenticatorSnapshot(t, existing)}},
					})
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

func TestDexAuthenticatorGatewayConflictWithIngress(t *testing.T) {
	for _, classes := range []struct{ name, existing, incoming string }{
		{"existing mixed", "nginx", ""}, {"incoming mixed", "", "nginx"}, {"both mixed", "first", "second"}, {"ingress conflict with distinct ListenerSets", "nginx", "nginx"},
	} {
		t.Run(classes.name, func(t *testing.T) {
			app := func(class string) map[string]any {
				fields := map[string]any{"domain": "app.example.com", "gatewayAPI": listenerSetReference("shared", "d8-alb", "https")}
				if class != "" {
					fields["ingressClassName"] = class
				}
				return fields
			}
			existing := authenticatorWithApplication("team-a", app(classes.existing), false)
			incoming := authenticatorWithApplication("team-b", app(classes.incoming), false)
			ingressConflict := classes.existing != "" && classes.existing == classes.incoming
			if ingressConflict {
				incoming["spec"].(map[string]any)["applications"].([]any)[0].(map[string]any)["gatewayAPI"] = listenerSetReference("other", "d8-alb", "https")
			}
			result := runAuthenticatorHook(t, "validating/dex_authenticator", "__main__", map[string]any{
				"review":    map[string]any{"request": map[string]any{"object": incoming}},
				"snapshots": map[string]any{"dexauthenticators": []any{authenticatorSnapshot(t, existing)}},
			})
			require.Equal(t, false, result["allowed"])
			if ingressConflict {
				require.Contains(t, result["message"], "conflicts")
				require.NotContains(t, result["message"], "ListenerSet")
			} else {
				require.Equal(t, "Desired DexAuthenticator 'team-b/test' conflicts with the existing DexAuthenticator 'team-a/test' for domain 'app.example.com' on ListenerSet 'd8-alb/shared'", result["message"])
			}
		})
	}
}

func TestDexAuthenticatorConflictUpdate(t *testing.T) {
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
				incoming := authenticatorWithApplication("team-a", app, false)
				snapshots := []any{authenticatorSnapshot(t, incoming)}
				if conflict {
					snapshots = append(snapshots, authenticatorSnapshot(t, authenticatorWithApplication("team-b", app, false)))
				}
				result := runAuthenticatorHook(t, "validating/dex_authenticator", "__main__", map[string]any{
					"review":    map[string]any{"request": map[string]any{"operation": "UPDATE", "object": incoming}},
					"snapshots": map[string]any{"dexauthenticators": snapshots},
				})
				require.Equal(t, !conflict, result["allowed"])
			})
		}
	}
}

// A shared domain alone does not imply a conflict between Ingress and Gateway API.
func TestDexAuthenticatorAllowsSameDomainAcrossRoutingModes(t *testing.T) {
	for _, version := range []string{"v1", "v1alpha1", "v2alpha1"} {
		for _, location := range []struct {
			name                                   string
			existingAdditional, incomingAdditional bool
		}{
			{"primary-primary", false, false}, {"primary-additional", false, true},
			{"additional-primary", true, false}, {"additional-additional", true, true},
		} {
			for _, existingMode := range []string{"ingress", "gateway"} {
				t.Run(version+"/"+location.name+"/existing-"+existingMode, func(t *testing.T) {
					existingApp := map[string]any{"domain": "app.example.com", "ingressClassName": "nginx"}
					incomingApp := map[string]any{"domain": "app.example.com", "gatewayAPI": listenerSetReference("shared", "d8-alb", "https")}
					if existingMode == "gateway" {
						existingApp, incomingApp = incomingApp, existingApp
					}
					existing := authenticatorWithApplication("team-a", existingApp, location.existingAdditional)
					incoming := authenticatorWithApplication("team-b", incomingApp, location.incomingAdditional)
					if version != "v2alpha1" {
						incoming = convertAuthenticator(t, incoming, "v2alpha1", "v1")
						incoming["apiVersion"] = "deckhouse.io/" + version
					}
					result := runAuthenticatorHook(t, "validating/dex_authenticator", "__main__", map[string]any{
						"review":    map[string]any{"request": map[string]any{"operation": "CREATE", "object": incoming}},
						"snapshots": map[string]any{"dexauthenticators": []any{authenticatorSnapshot(t, existing)}},
					})
					require.Equal(t, true, result["allowed"])
				})
			}
		}
	}
}
