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
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuralcel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

func tokenExchangeProviderSchema(t *testing.T) *apiextensions.JSONSchemaProps {
	t.Helper()
	data, err := os.ReadFile("token-exchange-provider.yaml")
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.UnmarshalStrict(data, &document))
	var crd apiextensionsv1.CustomResourceDefinition
	require.NoError(t, yaml.Unmarshal(data, &crd))
	require.Len(t, crd.Spec.Versions, 1)

	schema := new(apiextensions.JSONSchemaProps)
	require.NoError(t, apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(crd.Spec.Versions[0].Schema.OpenAPIV3Schema, schema, nil))
	structural, err := structuralschema.NewStructural(schema)
	require.NoError(t, err)
	require.Empty(t, structuralschema.ValidateStructural(nil, structural))
	return schema
}

// validateTokenExchangeProvider returns the schema and CEL errors of the API server for spec.
func validateTokenExchangeProvider(t *testing.T, schema *apiextensions.JSONSchemaProps, spec map[string]any) []string {
	t.Helper()
	return validateNamedTokenExchangeProvider(t, schema, "test", spec)
}

func validateNamedTokenExchangeProvider(t *testing.T, schema *apiextensions.JSONSchemaProps, name string, spec map[string]any) []string {
	t.Helper()
	object := map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "TokenExchangeProvider",
		"metadata":   map[string]any{"name": name},
		"spec":       spec,
	}
	structural, err := structuralschema.NewStructural(schema)
	require.NoError(t, err)
	defaulting.PruneNonNullableNullsWithoutDefaults(object, structural)
	defaulting.Default(object, structural)
	validator, _, err := validation.NewSchemaValidator(schema)
	require.NoError(t, err)
	errs := validation.ValidateCustomResource(nil, object, validator)
	celValidator := structuralcel.NewValidator(structural, true, celconfig.PerCallLimit)
	require.NotNil(t, celValidator)
	celErrs, _ := celValidator.Validate(context.Background(), nil, structural, object, nil, celconfig.RuntimeCELCostBudget)
	var messages []string
	for _, e := range append(errs, celErrs...) {
		messages = append(messages, e.Error())
	}
	return messages
}

func gitlabCISpec() map[string]any {
	return map[string]any{
		"issuer":    "https://gitlab.example.com",
		"keys":      map[string]any{"discovery": map[string]any{}},
		"audiences": []any{map[string]any{"exact": "https://dex.example.com"}},
		"claimValidation": []any{
			map[string]any{"claim": "namespace_path", "equals": "platform"},
		},
		"identity": map[string]any{"username": map[string]any{"claim": "sub", "prefix": "gitlab-ci:"}},
		"clients":  []any{map[string]any{"dexClient": map[string]any{"name": "deployer", "namespace": "d8-ci"}}},
	}
}

func TestTokenExchangeProviderSchema(t *testing.T) {
	t.Parallel()
	schema := tokenExchangeProviderSchema(t)

	for _, tc := range []struct {
		name    string
		mutate  func(spec map[string]any)
		errText string
	}{
		{name: "GitLab CI with discovery", mutate: func(map[string]any) {}},
		{
			name: "pinned public keys",
			mutate: func(spec map[string]any) {
				spec["keys"] = map[string]any{"jwks": map[string]any{"keys": []any{map[string]any{"kty": "RSA", "n": "AQAB", "e": "AQAB", "kid": "a"}}}}
			},
		},
		{
			name:    "no key source",
			mutate:  func(spec map[string]any) { spec["keys"] = map[string]any{} },
			errText: "Exactly one of spec.keys.discovery and spec.keys.jwks is required.",
		},
		{
			name: "two key sources",
			mutate: func(spec map[string]any) {
				spec["keys"] = map[string]any{"discovery": map[string]any{}, "jwks": map[string]any{"keys": []any{map[string]any{"kty": "RSA"}}}}
			},
			errText: "Exactly one of spec.keys.discovery and spec.keys.jwks is required.",
		},
		{
			name:    "audience with both forms",
			mutate:  func(spec map[string]any) { spec["audiences"] = []any{map[string]any{"exact": "a", "prefix": "b"}} },
			errText: "Exactly one of exact and prefix is required in an audience.",
		},
		{
			name:    "no audiences",
			mutate:  func(spec map[string]any) { spec["audiences"] = []any{} },
			errText: "spec.audiences",
		},
		{
			name: "system: user name prefix",
			mutate: func(spec map[string]any) {
				spec["identity"] = map[string]any{"username": map[string]any{"claim": "sub", "prefix": "system:ci:"}}
			},
			errText: "A prefix must not start with system:.",
		},
		{
			name: "system: group prefix",
			mutate: func(spec map[string]any) {
				spec["identity"] = map[string]any{
					"username": map[string]any{"claim": "sub"},
					"groups":   []any{map[string]any{"claim": "groups", "prefix": "system:"}},
				}
			},
			errText: "A prefix must not start with system:.",
		},
		{
			name: "user name prefix without a colon",
			mutate: func(spec map[string]any) {
				spec["identity"] = map[string]any{"username": map[string]any{"claim": "sub", "prefix": "gitlab-ci"}}
			},
			errText: "ends with a colon",
		},
		{
			name: "group prefix without a colon",
			mutate: func(spec map[string]any) {
				spec["identity"] = map[string]any{
					"username": map[string]any{"claim": "sub", "prefix": "ci:"},
					"groups":   []any{map[string]any{"claim": "groups", "prefix": "ci-"}},
				}
			},
			errText: "ends with a colon",
		},
		{
			name: "no prefix at all",
			mutate: func(spec map[string]any) {
				spec["identity"] = map[string]any{"username": map[string]any{"claim": "email"}}
			},
			errText: "A prefix is required",
		},
		{
			name: "empty user name prefix",
			mutate: func(spec map[string]any) {
				spec["identity"] = map[string]any{"username": map[string]any{"claim": "email", "prefix": ""}}
			},
			errText: "A prefix is required",
		},
		{
			name: "a colon alone",
			mutate: func(spec map[string]any) {
				spec["identity"] = map[string]any{"username": map[string]any{"claim": "email", "prefix": ":"}}
			},
			errText: "A prefix is required",
		},
		{
			name: "group without a prefix",
			mutate: func(spec map[string]any) {
				spec["identity"] = map[string]any{
					"username": map[string]any{"claim": "sub", "prefix": "ci:"},
					"groups":   []any{map[string]any{"claim": "groups"}},
				}
			},
			errText: "A prefix is required",
		},
		{
			name: "email without a prefix",
			mutate: func(spec map[string]any) {
				spec["identity"] = map[string]any{
					"username": map[string]any{"claim": "sub", "prefix": "ci:"},
					"email":    map[string]any{"claim": "user_email"},
				}
			},
			errText: "A prefix is required",
		},
		{
			name:    "plain http issuer",
			mutate:  func(spec map[string]any) { spec["issuer"] = "http://gitlab.example.com" },
			errText: "The issuer must be an https URL",
		},
		{
			name:    "issuer with a query",
			mutate:  func(spec map[string]any) { spec["issuer"] = "https://gitlab.example.com/?tenant=a" },
			errText: "The issuer must be an https URL",
		},
		{
			name:    "issuer with a fragment",
			mutate:  func(spec map[string]any) { spec["issuer"] = "https://gitlab.example.com/#a" },
			errText: "The issuer must be an https URL",
		},
		{
			name:    "issuer that is not a URL",
			mutate:  func(spec map[string]any) { spec["issuer"] = "gitlab" },
			errText: "The issuer must be an https URL",
		},
		{
			name:   "issuer with a path",
			mutate: func(spec map[string]any) { spec["issuer"] = "https://dex.example.com/dex/" },
		},
		{
			name:   "own token lifetime",
			mutate: func(spec map[string]any) { spec["subjectTokenMaxLifetime"] = "1h30m" },
		},
		{
			name:    "token lifetime in days",
			mutate:  func(spec map[string]any) { spec["subjectTokenMaxLifetime"] = "1d" },
			errText: "days such as 1d are not accepted",
		},
		{
			name:    "zero token lifetime",
			mutate:  func(spec map[string]any) { spec["subjectTokenMaxLifetime"] = "0s" },
			errText: "subjectTokenMaxLifetime must be a positive duration",
		},
		{
			name:   "discovery at another https address",
			mutate: func(spec map[string]any) { spec["keys"] = map[string]any{"discovery": map[string]any{"url": "https://idp.example.com/oidc/config"}} },
		},
		{
			name:    "discovery over plain http",
			mutate:  func(spec map[string]any) { spec["keys"] = map[string]any{"discovery": map[string]any{"url": "http://idp.example.com/.well-known/openid-configuration"}} },
			errText: "The discovery document must be read over https.",
		},
		{
			name:    "no clients",
			mutate:  func(spec map[string]any) { spec["clients"] = []any{} },
			errText: "spec.clients",
		},
		{
			name:    "client without a DexClient",
			mutate:  func(spec map[string]any) { spec["clients"] = []any{map[string]any{}} },
			errText: "dexClient",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := gitlabCISpec()
			tc.mutate(spec)
			errs := validateTokenExchangeProvider(t, schema, spec)
			if tc.errText == "" {
				require.Empty(t, errs)
				return
			}
			require.NotEmpty(t, errs)
			require.True(t, strings.Contains(strings.Join(errs, "\n"), tc.errText), "errors %q do not mention %q", errs, tc.errText)
		})
	}
}

func TestTokenExchangeProviderLifetimeDefault(t *testing.T) {
	t.Parallel()
	schema := tokenExchangeProviderSchema(t)
	structural, err := structuralschema.NewStructural(schema)
	require.NoError(t, err)
	object := map[string]any{"spec": gitlabCISpec()}
	defaulting.Default(object, structural)
	require.Equal(t, "24h", object["spec"].(map[string]any)["subjectTokenMaxLifetime"])
}

// The name is part of the connector ID, which goes into label values of at most 63 characters.
func TestTokenExchangeProviderNameLength(t *testing.T) {
	t.Parallel()
	schema := tokenExchangeProviderSchema(t)

	require.Empty(t, validateNamedTokenExchangeProvider(t, schema, strings.Repeat("a", 50), gitlabCISpec()))
	errs := validateNamedTokenExchangeProvider(t, schema, strings.Repeat("a", 51), gitlabCISpec())
	require.True(t, strings.Contains(strings.Join(errs, "\n"), "at most 50 characters"), "errors %q", errs)
}
