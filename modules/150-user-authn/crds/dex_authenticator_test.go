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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuralcel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel/model"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/common"
	"sigs.k8s.io/yaml"
)

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

// validateAuthenticatorUpdate validates an update the way the API server does, with ratcheting: a value
// the old object already had passes a rule it breaks, as long as the update leaves it as it was.
func validateAuthenticatorUpdate(t *testing.T, schema *apiextensions.JSONSchemaProps, object, old map[string]any) field.ErrorList {
	t.Helper()
	structural, err := structuralschema.NewStructural(schema)
	require.NoError(t, err)
	validator, _, err := validation.NewSchemaValidator(schema)
	require.NoError(t, err)
	correlated := common.NewCorrelatedObject(object, old, &model.Structural{Structural: structural})
	errors := validation.ValidateCustomResourceUpdate(nil, object, old, validator, validation.WithRatcheting(correlated))
	celValidator := structuralcel.NewValidator(structural, true, celconfig.PerCallLimit)
	celErrors, _ := celValidator.Validate(context.Background(), nil, structural, object, old, celconfig.RuntimeCELCostBudget,
		structuralcel.WithRatcheting(correlated))
	return append(errors, celErrors...)
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

// The name of a DexAuthenticator is a label value of its objects and a part of the name of its Service,
// so it has to be a DNS-1035 label: a name the apiserver refuses there would fail the whole release.
func TestDexAuthenticatorNameIsADNSLabel(t *testing.T) {
	names := map[string]bool{
		"app":                   true,
		"a1":                    true,
		"my-app":                true,
		strings.Repeat("a", 63): true,
		"1app":                  false,
		"123":                   false,
		"0x1":                   false,
		"my.app":                false,
		strings.Repeat("a", 64): false,
	}
	for version, schema := range authenticatorSchemas(t) {
		for name, valid := range names {
			t.Run(version+"/"+name, func(t *testing.T) {
				validateAuthenticator(t, schema, namedAuthenticator(version, name, "app.example.com"), valid)
			})
		}
	}
}

// An authenticator created before the name rules, with a name they refuse, can still be updated: the
// rules sit on metadata.name, which an update does not change, so the API server ratchets them.
func TestDexAuthenticatorNameRulesRatchet(t *testing.T) {
	for version, schema := range authenticatorSchemas(t) {
		for _, name := range []string{"1app", "my.app", strings.Repeat("a", 64)} {
			t.Run(version+"/"+name, func(t *testing.T) {
				old := namedAuthenticator(version, name, "app.example.com")
				updated := namedAuthenticator(version, name, "other.example.com")
				require.Empty(t, validateAuthenticatorUpdate(t, schema, updated, old))
			})
		}
	}
}

// namedAuthenticator is a valid authenticator of the version with the name and the application domain.
func namedAuthenticator(version, name, domain string) map[string]any {
	var object map[string]any
	if version == "v2alpha1" {
		object = authenticatorObject(version, []any{map[string]any{"domain": domain, "ingressClassName": "nginx"}})
	} else {
		object = authenticatorObject(version, nil)
		object["spec"] = map[string]any{"applicationDomain": domain, "applicationIngressClassName": "nginx"}
	}
	object["metadata"].(map[string]any)["name"] = name
	return object
}

// The templates parse keepUsersLoggedInFor as a duration, and a value they cannot parse fails the release
// of the whole module. The schema takes the format of idTokenTTL in the module settings and bounds the
// number of digits of each unit, so that the sum of the largest values still fits time.Duration.
func TestDexAuthenticatorKeepUsersLoggedInFor(t *testing.T) {
	values := map[string]bool{
		"168h":                     true,
		"30m":                      true,
		"20s":                      true,
		"2h30m10s":                 true,
		"1h0s":                     true,
		"":                         true,
		"99999h9999999m999999999s": true,
		"7d":                       false,
		"1.5h":                     false,
		"10 m":                     false,
		"1H":                       false,
		"-1h":                      false,
		"30m1h":                    false,
		"forever":                  false,
		"100000h":                  false,
		"2562048h":                 false,
		"3000000h":                 false,
		"10000000m":                false,
		"1000000000s":              false,
		"99999999999999999999s":    false,
	}
	for version, schema := range authenticatorSchemas(t) {
		for value, valid := range values {
			t.Run(version+"/"+value, func(t *testing.T) {
				object := namedAuthenticator(version, "app", "app.example.com")
				object["spec"].(map[string]any)["keepUsersLoggedInFor"] = value
				validateAuthenticator(t, schema, object, valid)
				if valid && value != "" {
					_, err := time.ParseDuration(value)
					require.NoError(t, err, "the schema admits a value the templates cannot parse")
				}
			})
		}
	}
}
