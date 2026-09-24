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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/require"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuralcel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

// The validating hook is Bash, not one jq program, so its tests run it the way shell-operator
// does. The unit test image carries bash but no jq, so a jq built from gojq stands in when the
// real one is absent; a present jq is preferred, to keep local runs on what production uses.
func TestMain(m *testing.M) {
	os.Exit(runAuthenticatorTests(m))
}

// hookIPCheck is what is_ip_address runs under python3. The stand-in under testdata emulates
// exactly this, so it is only put on PATH while the hook still asks for it.
const hookIPCheck = "ipaddress.ip_address(sys.argv[1])"

// The tools the hooks need. Each has a stand-in under testdata, built only when the image carries
// no real one; a present tool is preferred, to keep local runs on what production uses.
var authenticatorTools = []string{"jq", "python3"}

func runAuthenticatorTests(m *testing.M) int {
	if _, err := exec.LookPath("bash"); err != nil {
		fmt.Fprintln(os.Stderr, "bash is required: these tests run the webhooks under ../webhooks")

		return 1
	}

	dir, err := os.MkdirTemp("", "hookshims")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}
	defer os.RemoveAll(dir)

	for _, tool := range authenticatorTools {
		if _, err := exec.LookPath(tool); err == nil {
			continue
		}
		if tool == "python3" {
			if err := checkIPCheckUnchanged(); err != nil {
				fmt.Fprintln(os.Stderr, err)

				return 1
			}
		}

		source := filepath.Join("testdata", tool)
		build := exec.Command("go", "build", "-o", filepath.Join(dir, tool), "./"+source)
		if out, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "building the %s stand-in: %v\n%s", tool, err, out)

			return 1
		}
		fmt.Fprintf(os.Stderr, "no %s found, standing in with %s\n", tool, source)
	}

	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	return m.Run()
}

// checkIPCheckUnchanged refuses to emulate Python the hook no longer runs. The hook reads only the
// exit status of python3 and discards its output, so the stand-in itself cannot report this.
func checkIPCheckUnchanged() error {
	hook, err := os.ReadFile(filepath.Join("..", "webhooks", "validating", "dex_authenticator"))
	if err != nil {
		return fmt.Errorf("reading the validating hook: %w", err)
	}

	if !strings.Contains(string(hook), hookIPCheck) {
		return fmt.Errorf("is_ip_address no longer runs %s, which testdata/python3 emulates: "+
			"revisit testdata/python3, or install python3 to run the hook as it is", hookIPCheck)
	}

	return nil
}

func authenticatorSchemas(t *testing.T) map[string]*apiextensions.JSONSchemaProps {
	t.Helper()
	data, err := os.ReadFile("dex-authenticator.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.UnmarshalStrict(data, &document))
	var crd apiextensionsv1.CustomResourceDefinition
	require.NoError(t, yaml.Unmarshal(data, &crd))
	schemas := make(map[string]*apiextensions.JSONSchemaProps)
	for _, version := range crd.Spec.Versions {
		schema := new(apiextensions.JSONSchemaProps)
		require.NoError(t, apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(version.Schema.OpenAPIV3Schema, schema, nil))
		structural, err := structuralschema.NewStructural(schema)
		require.NoError(t, err)
		require.Empty(t, structuralschema.ValidateStructural(nil, structural), version.Name)
		schemas[version.Name] = schema
	}
	return schemas
}

func authenticatorObject(version string, apps []any) map[string]any {
	return map[string]any{
		"apiVersion": "deckhouse.io/" + version,
		"kind":       "DexAuthenticator",
		"metadata":   map[string]any{"name": "test", "namespace": "test"},
		"spec":       map[string]any{"applications": apps},
	}
}

func validateAuthenticator(t *testing.T, schema *apiextensions.JSONSchemaProps, object map[string]any, valid bool) field.ErrorList {
	t.Helper()
	// Match API server decoding: non-nullable optional fields with null values are removed.
	structural, err := structuralschema.NewStructural(schema)
	require.NoError(t, err)
	defaulting.PruneNonNullableNullsWithoutDefaults(object, structural)
	validator, _, err := validation.NewSchemaValidator(schema)
	require.NoError(t, err)
	errors := validation.ValidateCustomResource(nil, object, validator)
	celValidator := structuralcel.NewValidator(structural, true, celconfig.PerCallLimit)
	require.NotNil(t, celValidator)
	celErrors, _ := celValidator.Validate(context.Background(), nil, structural, object, nil, celconfig.RuntimeCELCostBudget)
	errors = append(errors, celErrors...)
	if valid {
		require.Empty(t, errors)
	} else {
		require.NotEmpty(t, errors)
	}
	return errors
}

func TestDexAuthenticatorRoutingSchema(t *testing.T) {
	for version, schema := range authenticatorSchemas(t) {
		for _, additional := range []bool{false, true} {
			for _, tc := range []struct {
				name   string
				fields string
				valid  bool
			}{
				{"ingress", `"ingressClassName":"nginx"`, true},
				{"gateway", `"gatewayAPI":{"httpRouteListenerSetName":"app"}`, true},
				{"both", `"ingressClassName":"nginx","gatewayAPI":{"httpRouteListenerSetName":"app"}`, true},
				{"neither", ``, false},
				{"empty class", `"ingressClassName":"","gatewayAPI":{"httpRouteListenerSetName":"app"}`, false},
				{"empty gateway", `"gatewayAPI":{}`, false},
				{"invalid gateway with ingress", `"ingressClassName":"nginx","gatewayAPI":{}`, false},
				{"empty listener", `"gatewayAPI":{"httpRouteListenerSetName":""}`, false},
			} {
				location := "primary"
				if additional {
					location = "additional"
				}
				t.Run(version+"/"+location+"/"+tc.name, func(t *testing.T) {
					app := map[string]any{"domain": "app.example.com"}
					fields := map[string]any{}
					require.NoError(t, json.Unmarshal([]byte("{"+tc.fields+"}"), &fields))
					for k, v := range fields {
						app[k] = v
					}
					object := authenticatorObject(version, []any{app})
					if version == "v2alpha1" {
						if additional {
							object["spec"].(map[string]any)["applications"] = []any{map[string]any{"domain": "first.example.com", "ingressClassName": "nginx"}, app}
						}
					} else if additional {
						object["spec"] = map[string]any{"applicationDomain": "first.example.com", "applicationIngressClassName": "nginx", "additionalApplications": []any{app}}
					} else {
						spec := map[string]any{"applicationDomain": app["domain"]}
						if value, ok := app["ingressClassName"]; ok {
							spec["applicationIngressClassName"] = value
						}
						if value, ok := app["gatewayAPI"]; ok {
							gateway := map[string]any{}
							for k, v := range value.(map[string]any) {
								gateway[strings.Replace(k, "httpRoute", "applicationHTTPRoute", 1)] = v
							}
							spec["gatewayAPI"] = gateway
						}
						object["spec"] = spec
					}
					errors := validateAuthenticator(t, schema, object, tc.valid)
					if tc.name == "neither" {
						classField := "ingressClassName"
						if version != "v2alpha1" && !additional {
							classField = "applicationIngressClassName"
						}
						require.Len(t, errors, 1)
						require.Equal(t, "Specify "+classField+" or gatewayAPI to publish the application.", errors[0].Detail)
					}
				})
			}
		}
	}
}

// Execute the actual Bash hook, substituting only shell-operator context and response plumbing.
func runAuthenticatorHook(t *testing.T, path, function string, context map[string]any) map[string]any {
	t.Helper()
	script, err := os.ReadFile(filepath.Join("..", "webhooks", path))
	require.NoError(t, err)
	dir := t.TempDir()
	input, err := json.Marshal(context)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "context.json"), input, 0600))
	source := strings.Replace(string(script), "source /shell_lib.sh", `function context::jq() { jq "$@" "$TEST_CONTEXT"; }
function hook::run() { :; }`, 1)
	cmd := exec.Command("bash", "-e", "-c", source+"\n"+function)
	response := filepath.Join(dir, "response.json")
	cmd.Env = append(os.Environ(), "TEST_CONTEXT="+filepath.Join(dir, "context.json"), "CONVERSION_RESPONSE_PATH="+response, "VALIDATING_RESPONSE_PATH="+response)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	data, err := os.ReadFile(response)
	require.NoError(t, err, "%s", output)
	result := map[string]any{}
	require.NoError(t, json.Unmarshal(data, &result))
	return result
}

// A conversion function is one jq program wrapped in shell plumbing, so the program runs here
// directly. That keeps the conversion tests free of an external jq, and mirrors how the projects
// hook of multitenancy-manager is tested.
func convertAuthenticator(t *testing.T, object map[string]any, from, to string) map[string]any {
	t.Helper()
	query, err := gojq.Parse(conversionProgram(t, "__on_conversion::"+from+"_to_"+to))
	require.NoError(t, err)

	review := map[string]any{"review": map[string]any{"request": map[string]any{"objects": []any{object}}}}
	result, ok := query.Run(review).Next()
	require.True(t, ok, "%s to %s produced nothing", from, to)
	if failure, isErr := result.(error); isErr {
		t.Fatalf("converting %s to %s: %v", from, to, failure)
	}

	converted, ok := result.([]any)
	require.True(t, ok, "%s to %s produced %v", from, to, result)
	require.Len(t, converted, 1)

	return converted[0].(map[string]any)
}

// conversionProgram lifts the jq source of one conversion function out of the shell hook. The
// programs are single-quoted bash strings, which cannot contain a single quote, so the quotes
// delimit them unambiguously.
func conversionProgram(t *testing.T, function string) string {
	t.Helper()
	hook, err := os.ReadFile(filepath.Join("..", "webhooks", "conversion", "dex-authenticator"))
	require.NoError(t, err)

	body := string(hook)
	start := strings.Index(body, "function "+function+"()")
	require.GreaterOrEqual(t, start, 0, "the hook has no %s", function)

	body = body[start:]
	open := strings.Index(body, "'")
	require.GreaterOrEqual(t, open, 0, "%s runs no jq program", function)

	body = body[open+1:]
	end := strings.Index(body, "'")
	require.GreaterOrEqual(t, end, 0, "the jq program of %s is not closed", function)

	return body[:end]
}

func TestDexAuthenticatorConversionPreservesRouting(t *testing.T) {
	schemas := authenticatorSchemas(t)
	for _, fixture := range []string{
		`[{"domain":"app.example.com","ingressClassName":"nginx"}]`,
		`[{"domain":"app.example.com","gatewayAPI":{"httpRouteListenerSetName":"app"}}]`,
		`[{"domain":"app.example.com","gatewayAPI":{"httpRouteListenerSetName":"app","httpRouteListenerSetNamespace":"gateway","httpRouteListenerSetSectionName":"https"},"signOutURL":"/logout"},{"domain":"second.example.com","gatewayAPI":{"httpRouteListenerSetName":"second","httpRouteListenerSetNamespace":"gateway","httpRouteListenerSetSectionName":"https"},"signOutURL":"/bye","whitelistSourceRanges":["192.0.2.0/24"]}]`,
		`[{"domain":"app.example.com","ingressClassName":"nginx","gatewayAPI":{"httpRouteListenerSetName":"app"}},{"domain":"second.example.com","gatewayAPI":{"httpRouteListenerSetName":"second"}},{"domain":"third.example.com","ingressClassName":"nginx"}]`,
	} {
		t.Run(fixture, func(t *testing.T) {
			var apps []any
			require.NoError(t, json.Unmarshal([]byte(fixture), &apps))
			object := authenticatorObject("v2alpha1", apps)
			stored := convertAuthenticator(t, object, "v2alpha1", "v1")
			first := apps[0].(map[string]any)
			storedSpec := stored["spec"].(map[string]any)
			_, hasClass := first["ingressClassName"]
			_, storedClass := storedSpec["applicationIngressClassName"]
			require.Equal(t, hasClass, storedClass)
			_, hasGateway := first["gatewayAPI"]
			_, storedGateway := storedSpec["gatewayAPI"]
			require.Equal(t, hasGateway, storedGateway)
			structural, err := structuralschema.NewStructural(schemas["v1"])
			require.NoError(t, err)
			pruning.Prune(stored, structural, true)
			validateAuthenticator(t, schemas["v1"], stored, true)
			legacy := convertAuthenticator(t, stored, "v1", "v1alpha1")
			validateAuthenticator(t, schemas["v1alpha1"], legacy, true)
			stored = convertAuthenticator(t, legacy, "v1alpha1", "v1")
			restored := convertAuthenticator(t, stored, "v1", "v2alpha1")
			validateAuthenticator(t, schemas["v2alpha1"], restored, true)
			require.Equal(t, apps, restored["spec"].(map[string]any)["applications"])
		})
	}
}

func TestDexAuthenticatorDomainValidationWithoutIngress(t *testing.T) {
	for _, version := range []string{"v1", "v1alpha1", "v2alpha1"} {
		for _, domain := range []string{"app.example.com", "192.0.2.1", "2001:db8::1"} {
			for _, additional := range []bool{false, true} {
				object := authenticatorObject("v2alpha1", []any{map[string]any{"domain": domain, "gatewayAPI": map[string]any{"httpRouteListenerSetName": "app"}}})
				if additional {
					apps := object["spec"].(map[string]any)["applications"].([]any)
					object["spec"].(map[string]any)["applications"] = append([]any{map[string]any{"domain": "first.example.com", "ingressClassName": "nginx"}}, apps...)
				}
				if version != "v2alpha1" {
					object = convertAuthenticator(t, object, "v2alpha1", "v1")
					object["apiVersion"] = "deckhouse.io/" + version
				}
				result := runAuthenticatorHook(t, "validating/dex_authenticator", "__main__", map[string]any{
					"review":    map[string]any{"request": map[string]any{"object": object}},
					"snapshots": map[string]any{"dexauthenticators": []any{}},
				})
				require.Equal(t, domain == "app.example.com", result["allowed"], "%s %s additional=%v", version, domain, additional)
			}
		}
	}
}

func TestDexAuthenticatorIngressConflict(t *testing.T) {
	object := authenticatorObject("v2alpha1", []any{map[string]any{"domain": "app.example.com", "ingressClassName": "nginx"}})
	for _, version := range []string{"v2alpha1", "v1", "v1alpha1"} {
		if version == "v1" {
			object = convertAuthenticator(t, object, "v2alpha1", "v1")
		}
		object["apiVersion"] = "deckhouse.io/" + version
		result := runAuthenticatorHook(t, "validating/dex_authenticator", "__main__", map[string]any{
			"review":    map[string]any{"request": map[string]any{"object": object}},
			"snapshots": map[string]any{"dexauthenticators": []any{map[string]any{"filterResult": map[string]any{"name": "existing", "namespace": "test", "applicationDomain": "app.example.com", "ingressClass": "nginx", "additionalDomains": []any{}}}}},
		})
		require.Equal(t, false, result["allowed"], version)
		require.Contains(t, result["message"], "conflicts")
	}
}

// Older converters emitted a gatewayAPI object full of nulls even for Ingress-only applications.
func TestDexAuthenticatorLegacyEmptyGateway(t *testing.T) {
	schemas := authenticatorSchemas(t)
	for _, gateway := range []any{nil, map[string]any{}, map[string]any{
		"applicationHTTPRouteListenerSetName":        nil,
		"applicationHTTPRouteListenerSetNamespace":   nil,
		"applicationHTTPRouteListenerSetSectionName": nil,
	}} {
		object := authenticatorObject("v1", nil)
		object["spec"] = map[string]any{"applicationDomain": "app.example.com", "applicationIngressClassName": "nginx", "gatewayAPI": gateway}
		converted := convertAuthenticator(t, object, "v1", "v2alpha1")
		apps := converted["spec"].(map[string]any)["applications"].([]any)
		require.NotContains(t, apps[0], "gatewayAPI")
		validateAuthenticator(t, schemas["v2alpha1"], converted, true)
	}
}

// Applications after the first take a resource-name suffix derived from sha256(domain), so two of
// them sharing a domain would name two objects of one kind alike. The first application carries no
// suffix, and an Ingress may share a name with an HTTPRoute, so neither of those repeats collides.
func TestDexAuthenticatorDuplicateDomainWithinObject(t *testing.T) {
	gateway := map[string]any{"httpRouteListenerSetName": "app-listeners"}
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
		for _, version := range []string{"v2alpha1", "v1", "v1alpha1"} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				object := authenticatorObject("v2alpha1", tc.apps)
				if version != "v2alpha1" {
					object = convertAuthenticator(t, object, "v2alpha1", "v1")
					object["apiVersion"] = "deckhouse.io/" + version
				}
				result := runAuthenticatorHook(t, "validating/dex_authenticator", "__main__", map[string]any{
					"review":    map[string]any{"request": map[string]any{"object": object}},
					"snapshots": map[string]any{"dexauthenticators": []any{}},
				})
				require.Equal(t, tc.duplicate == "", result["allowed"], "%v", result["message"])
				if tc.duplicate != "" {
					require.Contains(t, result["message"], "repeats domain 'same.example.com'")
					require.Contains(t, result["message"], "a second "+tc.duplicate)
				}
			})
		}
	}
}
