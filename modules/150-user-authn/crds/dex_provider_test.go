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
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"sigs.k8s.io/yaml"
)

const kerberosKeytabMessage = "spec.ldap.kerberos.keytabSecretName is required when spec.ldap.kerberos.enabled is true: " +
	"Dex needs the keytab to validate Kerberos tickets."

func providerSchemas(t *testing.T) map[string]*apiextensions.JSONSchemaProps {
	t.Helper()
	data, err := os.ReadFile("dex-provider.yaml")
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
	require.Len(t, schemas, 2)
	return schemas
}

// validateProvider runs the schema and CEL validation of the API server, after its pruning of nulls and
// defaulting, on a DexProvider with the given spec.
func validateProvider(t *testing.T, version string, schema *apiextensions.JSONSchemaProps, spec map[string]any) field.ErrorList {
	t.Helper()
	object := map[string]any{
		"apiVersion": "deckhouse.io/" + version,
		"kind":       "DexProvider",
		"metadata":   map[string]any{"name": "test"},
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
	return append(errs, celErrs...)
}

// ldapProviderSpec is an LDAP provider spec with the given kerberos block, or none when it is nil.
func ldapProviderSpec(kerberos map[string]any) map[string]any {
	ldap := map[string]any{
		"host":       "ldap.example.com:636",
		"userSearch": map[string]any{"baseDN": "ou=people,dc=example,dc=org", "username": "uid", "idAttr": "uid", "emailAttr": "mail"},
	}
	if kerberos != nil {
		ldap["kerberos"] = kerberos
	}
	return map[string]any{"type": "LDAP", "displayName": "LDAP", "ldap": ldap}
}

func TestDexProviderKerberosRequiresKeytab(t *testing.T) {
	t.Parallel()

	for version, schema := range providerSchemas(t) {
		for _, tc := range []struct {
			name     string
			kerberos map[string]any
			message  string
		}{
			{name: "kerberos absent"},
			{name: "enabled omitted", kerberos: map[string]any{"expectedRealm": "EXAMPLE.COM"}},
			{name: "disabled without keytab", kerberos: map[string]any{"enabled": false}},
			{
				name:     "disabled with settings kept",
				kerberos: map[string]any{"enabled": false, "keytabSecretName": "", "expectedRealm": "EXAMPLE.COM", "fallbackToPassword": true},
			},
			{name: "enabled with keytab", kerberos: map[string]any{"enabled": true, "keytabSecretName": "dex-keytab"}},
			{name: "enabled without keytab", kerberos: map[string]any{"enabled": true}, message: kerberosKeytabMessage},
			{
				name:     "enabled with empty keytab",
				kerberos: map[string]any{"enabled": true, "keytabSecretName": ""},
				message:  kerberosKeytabMessage,
			},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				errs := validateProvider(t, version, schema, ldapProviderSpec(tc.kerberos))
				if tc.message == "" {
					require.Empty(t, errs)
					return
				}
				require.Len(t, errs, 1)
				require.Equal(t, "spec.ldap.kerberos", errs[0].Field)
				require.Equal(t, tc.message, errs[0].Detail)
			})
		}
	}
}

// The search of an LDAP provider names the user by one attribute (username) or by several (usernames), and the
// groups of a user may be followed to their parents (recursionGroupAttr).
func TestDexProviderLDAPSearch(t *testing.T) {
	t.Parallel()

	const exactlyOne = "Set exactly one of username and usernames."
	userSearch := func(fields map[string]any) map[string]any {
		search := map[string]any{"baseDN": "ou=people,dc=example,dc=org", "idAttr": "uid", "emailAttr": "mail"}
		for k, v := range fields {
			search[k] = v
		}
		return search
	}
	tooMany := make([]any, 9)
	for i := range tooMany {
		tooMany[i] = "attr"
	}

	for version, schema := range providerSchemas(t) {
		for _, tc := range []struct {
			name    string
			search  map[string]any
			groups  map[string]any
			message string
		}{
			{name: "one attribute", search: userSearch(map[string]any{"username": "uid"})},
			{name: "several attributes", search: userSearch(map[string]any{"usernames": []any{"uid", "mail"}})},
			{name: "both", search: userSearch(map[string]any{"username": "uid", "usernames": []any{"mail"}}), message: exactlyOne},
			{name: "neither", search: userSearch(nil), message: exactlyOne},
			{name: "no attribute in the list", search: userSearch(map[string]any{"usernames": []any{}})},
			{name: "an empty attribute", search: userSearch(map[string]any{"usernames": []any{""}})},
			{name: "too many attributes", search: userSearch(map[string]any{"usernames": tooMany})},
			{
				name:   "recursion",
				search: userSearch(map[string]any{"username": "uid"}),
				groups: map[string]any{
					"baseDN": "ou=groups,dc=example,dc=org", "nameAttr": "cn",
					"userMatchers": []any{map[string]any{"userAttr": "DN", "groupAttr": "member", "recursionGroupAttr": "member"}},
				},
			},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				spec := ldapProviderSpec(nil)
				ldap := spec["ldap"].(map[string]any)
				ldap["userSearch"] = tc.search
				if tc.groups != nil {
					ldap["groupSearch"] = tc.groups
				}
				errs := validateProvider(t, version, schema, spec)
				switch tc.name {
				case "no attribute in the list", "too many attributes", "an empty attribute":
					require.NotEmpty(t, errs)
				case "both", "neither":
					require.Len(t, errs, 1)
					require.Equal(t, "spec.ldap.userSearch", errs[0].Field)
					require.Equal(t, tc.message, errs[0].Detail)
				default:
					require.Empty(t, errs)
				}
			})
		}
	}
}

// groupPrefix goes in front of every group a provider asserts. The API server refuses a prefix the API server or the
// peering reserve, one that is no name, and one that Basic Authentication of an OIDC or Crowd provider would let
// pass without it (basic-auth-proxy passes the groups of such a provider as they are).
func TestDexProviderGroupPrefix(t *testing.T) {
	t.Parallel()

	const (
		reserved   = "spec.groupPrefix must not start with system: or peering:, which the Kubernetes API server and the peering reserve."
		basicAuth  = "spec.groupPrefix cannot be set with enableBasicAuth of an OIDC or Crowd provider: Basic Authentication passes the groups of such a provider as they are, without the prefix."
		unprefixed = "Every name in spec.allowedIdentities.groups must start with spec.groupPrefix: Dex compares the groups after it adds the prefix, so a name without it matches nothing."
	)
	withPrefix := func(spec map[string]any, prefix any) map[string]any {
		spec["groupPrefix"] = prefix
		return spec
	}
	crowd := func(basicAuth bool) map[string]any {
		return map[string]any{"type": "Crowd", "displayName": "Crowd", "crowd": map[string]any{
			"baseURL": "https://crowd.example.com", "clientID": "id", "clientSecret": "secret", "enableBasicAuth": basicAuth}}
	}

	for version, schema := range providerSchemas(t) {
		for _, tc := range []struct {
			name    string
			spec    map[string]any
			message string
			invalid bool
		}{
			{name: "an LDAP provider", spec: withPrefix(ldapProviderSpec(nil), "corp-ldap:")},
			{name: "a prefix with a slash and a dot", spec: withPrefix(ldapProviderSpec(nil), "idp.example.com/ldap:")},
			{name: "a prefix with two colons", spec: withPrefix(ldapProviderSpec(nil), "corp:ldap:")},
			{name: "a prefix without a separator", spec: withPrefix(ldapProviderSpec(nil), "corp"), invalid: true},
			{name: "a prefix that ends with a dash", spec: withPrefix(ldapProviderSpec(nil), "corp-"), invalid: true},
			{name: "a prefix that ends with a slash", spec: withPrefix(ldapProviderSpec(nil), "idp.example.com/"), invalid: true},
			{name: "a colon alone", spec: withPrefix(ldapProviderSpec(nil), ":"), invalid: true},
			{name: "an OIDC provider", spec: withPrefix(oidcProviderSpec(false), "idp:")},
			{name: "no prefix", spec: ldapProviderSpec(nil)},
			{name: "system:", spec: withPrefix(ldapProviderSpec(nil), "system:"), message: reserved},
			{name: "system:masters:", spec: withPrefix(ldapProviderSpec(nil), "system:masters:"), message: reserved},
			{name: "peering:", spec: withPrefix(ldapProviderSpec(nil), "peering:"), message: reserved},
			{name: "an empty prefix", spec: withPrefix(ldapProviderSpec(nil), ""), invalid: true},
			{name: "a prefix with a space", spec: withPrefix(ldapProviderSpec(nil), "corp ldap:"), invalid: true},
			{name: "a prefix with a line break", spec: withPrefix(ldapProviderSpec(nil), "corp\nldap:"), invalid: true},
			{name: "a prefix that starts with a separator", spec: withPrefix(ldapProviderSpec(nil), ":corp:"), invalid: true},
			{name: "a prefix of 65 characters", spec: withPrefix(ldapProviderSpec(nil), strings.Repeat("a", 64)+":"), invalid: true},
			{name: "OIDC with Basic Authentication", spec: withPrefix(oidcProviderSpec(true), "idp:"), message: basicAuth},
			{name: "Crowd with Basic Authentication", spec: withPrefix(crowd(true), "crowd:"), message: basicAuth},
			{name: "Crowd without Basic Authentication", spec: withPrefix(crowd(false), "crowd:")},
			// Basic Authentication of an LDAP provider goes through Dex, which puts the prefix in front.
			{name: "groups that start with the prefix", spec: func() map[string]any {
				spec := withPrefix(ldapProviderSpec(nil), "corp-ldap:")
				spec["allowedIdentities"] = map[string]any{"groups": []any{"corp-ldap:admins"}}
				return spec
			}()},
			{name: "a group without the prefix", spec: func() map[string]any {
				spec := withPrefix(ldapProviderSpec(nil), "corp-ldap:")
				spec["allowedIdentities"] = map[string]any{"groups": []any{"admins"}}
				return spec
			}(), message: unprefixed},
			{name: "one of the groups without the prefix", spec: func() map[string]any {
				spec := withPrefix(ldapProviderSpec(nil), "corp-ldap:")
				spec["allowedIdentities"] = map[string]any{"groups": []any{"corp-ldap:admins", "developers"}}
				return spec
			}(), message: unprefixed},
			{name: "groups without a prefix on the provider", spec: func() map[string]any {
				spec := ldapProviderSpec(nil)
				spec["allowedIdentities"] = map[string]any{"groups": []any{"admins"}}
				return spec
			}()},
			{name: "LDAP with Basic Authentication", spec: func() map[string]any {
				spec := ldapProviderSpec(nil)
				spec["ldap"].(map[string]any)["enableBasicAuth"] = true
				return withPrefix(spec, "ldap:")
			}()},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				errs := validateProvider(t, version, schema, tc.spec)
				switch {
				case tc.message != "":
					require.Len(t, errs, 1)
					require.Equal(t, "spec", errs[0].Field)
					require.Equal(t, tc.message, errs[0].Detail)
				case tc.invalid:
					require.NotEmpty(t, errs)
				default:
					require.Empty(t, errs)
				}
			})
		}
	}
}

func oidcProviderSpec(basicAuth bool) map[string]any {
	return map[string]any{"type": "OIDC", "displayName": "OIDC", "oidc": map[string]any{
		"issuer": "https://idp.example.com", "clientID": "id", "clientSecret": "secret", "enableBasicAuth": basicAuth}}
}

// Only the providers whose connectors exchange tokens in Dex may have token exchange enabled.
func TestDexProviderTokenExchangeTypes(t *testing.T) {
	t.Parallel()

	oidc := func(tokenExchange map[string]any) map[string]any {
		spec := map[string]any{
			"type": "OIDC", "displayName": "OIDC",
			"oidc": map[string]any{"issuer": "https://idp.example.com", "clientID": "dex", "clientSecret": "plainstring"},
		}
		if tokenExchange != nil {
			spec["tokenExchange"] = tokenExchange
		}
		return spec
	}
	ldap := func(tokenExchange map[string]any) map[string]any {
		spec := ldapProviderSpec(nil)
		spec["tokenExchange"] = tokenExchange
		return spec
	}

	for version, schema := range providerSchemas(t) {
		for _, tc := range []struct {
			name    string
			spec    map[string]any
			refused bool
		}{
			{name: "OIDC without the field", spec: oidc(nil)},
			{name: "OIDC with exchange", spec: oidc(map[string]any{"enabled": true})},
			{name: "OIDC without exchange", spec: oidc(map[string]any{"enabled": false})},
			{name: "LDAP without exchange", spec: ldap(map[string]any{"enabled": false})},
			{name: "LDAP with exchange", spec: ldap(map[string]any{"enabled": true}), refused: true},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				errs := validateProvider(t, version, schema, tc.spec)
				if !tc.refused {
					require.Empty(t, errs)
					return
				}
				require.Len(t, errs, 1)
				require.Contains(t, errs[0].Detail, "Only OIDC and Gitlab providers can exchange tokens")
			})
		}
	}
}
