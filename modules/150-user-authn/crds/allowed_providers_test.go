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
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/listtype"
	"sigs.k8s.io/yaml"
)

func dexClientSchemas(t *testing.T) map[string]*apiextensions.JSONSchemaProps {
	t.Helper()
	data, err := os.ReadFile("dex-client.yaml")
	require.NoError(t, err)
	var crd apiextensionsv1.CustomResourceDefinition
	require.NoError(t, yaml.Unmarshal(data, &crd))
	schemas := make(map[string]*apiextensions.JSONSchemaProps)
	for _, version := range crd.Spec.Versions {
		schema := new(apiextensions.JSONSchemaProps)
		require.NoError(t, apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(version.Schema.OpenAPIV3Schema, schema, nil))
		schemas[version.Name] = schema
	}
	return schemas
}

// requireSetValues checks the uniqueness of the x-kubernetes-list-type: set lists, which the API server validates
// apart from the schema.
func requireSetValues(t *testing.T, schema *apiextensions.JSONSchemaProps, object map[string]any, unique bool) {
	t.Helper()
	structural, err := structuralschema.NewStructural(schema)
	require.NoError(t, err)
	errors := listtype.ValidateListSetsAndMaps(nil, structural, object)
	if unique {
		require.Empty(t, errors)
	} else {
		require.NotEmpty(t, errors)
	}
}

// allowedProviders become the connectors Dex allows for the client, so a name that is no DexProvider name, a
// repeated name or an unbounded list has to be refused at the API server. An allowedProviders that an older
// version of the schema pruned would let everybody in, so every served version of both kinds must keep it.
func TestAllowedProvidersSchema(t *testing.T) {
	many := make([]any, 65)
	for i := range many {
		many[i] = fmt.Sprintf("provider-%d", i)
	}

	for _, tc := range []struct {
		name      string
		providers []any
		valid     bool
		duplicate bool
	}{
		{"providers and local", []any{"corp-ldap", "local"}, true, false},
		{"dotted name", []any{"corp.ldap"}, true, false},
		{"empty list", []any{}, true, false},
		{"repeated name", []any{"corp-ldap", "corp-ldap"}, true, true},
		{"upper case", []any{"Corp"}, false, false},
		{"empty name", []any{""}, false, false},
		{"space", []any{"corp ldap"}, false, false},
		{"line break", []any{"corp\nldap"}, false, false},
		{"leading dash", []any{"-corp"}, false, false},
		{"too many", many, false, false},
	} {
		for version, schema := range authenticatorSchemas(t) {
			t.Run("DexAuthenticator/"+version+"/"+tc.name, func(t *testing.T) {
				object := namedAuthenticator(version, "app", "app.example.com")
				object["spec"].(map[string]any)["allowedProviders"] = tc.providers
				validateAuthenticator(t, schema, object, tc.valid)
				if tc.valid {
					requireSetValues(t, schema, object, !tc.duplicate)
				}
			})
		}
		for version, schema := range dexClientSchemas(t) {
			t.Run("DexClient/"+version+"/"+tc.name, func(t *testing.T) {
				object := map[string]any{
					"apiVersion": "deckhouse.io/" + version,
					"kind":       "DexClient",
					"metadata":   map[string]any{"name": "app", "namespace": "test"},
					"spec": map[string]any{
						"redirectURIs":     []any{"https://app.example.com/callback"},
						"allowedProviders": tc.providers,
					},
				}
				validateAuthenticator(t, schema, object, tc.valid)
				if tc.valid {
					requireSetValues(t, schema, object, !tc.duplicate)
				}
			})
		}
	}
}
