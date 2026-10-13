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
