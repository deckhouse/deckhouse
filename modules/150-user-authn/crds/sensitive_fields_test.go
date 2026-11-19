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
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const sensitiveMarker = "x-kubernetes-sensitive-data"

// The API server returns <omitted> instead of a field marked x-kubernetes-sensitive-data to a caller
// without the <resource>/sensitive subresource. Every served version must carry the marker, or the
// secret stays readable through that version. The schemas are read untyped: the marker is a Deckhouse
// extension the upstream JSONSchemaProps does not know.
func TestSecretFieldsAreSensitiveInEveryVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file   string
		plural string
		fields [][]string
	}{
		{
			file:   "dex-provider.yaml",
			plural: "dexproviders",
			fields: [][]string{
				{"spec", "github", "clientSecret"},
				{"spec", "gitlab", "clientSecret"},
				{"spec", "bitbucketCloud", "clientSecret"},
				{"spec", "crowd", "clientSecret"},
				{"spec", "oidc", "clientSecret"},
				{"spec", "ldap", "bindPW"},
			},
		},
		{file: "user.yaml", plural: "users", fields: [][]string{{"spec", "password"}}},
		{file: "user-operation.yaml", plural: "useroperations", fields: [][]string{{"spec", "resetPassword", "newPasswordHash"}}},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			// The description tells the reader why the value can come back as <omitted>.
			subresource := "`" + tt.plural + "/sensitive`"
			versions := readCRDVersions(t, tt.file)
			for name, schema := range versions {
				var marked [][]string
				collectSensitive(schema, nil, &marked)
				require.ElementsMatch(t, tt.fields, marked, "version %s of %s", name, tt.file)
				for _, path := range marked {
					description, _ := propertyAt(schema, path)["description"].(string)
					require.Contains(t, description, subresource, "description of %v in version %s of %s", path, name, tt.file)
				}
			}
		})
	}
}

// The other CRDs of the module keep no secrets in their fields: the secrets of DexClient and
// DexAuthenticator live in Secrets, and TokenExchangeProvider takes only public keys.
func TestNoOtherCRDHasSensitiveFields(t *testing.T) {
	t.Parallel()

	for _, file := range []string{
		"dex-authenticator.yaml", "dex-client.yaml", "dex-provider-check.yaml", "group.yaml",
		"token-exchange-provider.yaml", "user-account.yaml",
	} {
		for name, schema := range readCRDVersions(t, file) {
			var marked [][]string
			collectSensitive(schema, nil, &marked)
			require.Empty(t, marked, "version %s of %s", name, file)
		}
	}
}

func readCRDVersions(t *testing.T, file string) map[string]map[string]any {
	t.Helper()

	data, err := os.ReadFile(file)
	require.NoError(t, err)

	var crd struct {
		Spec struct {
			Versions []struct {
				Name   string `json:"name"`
				Served bool   `json:"served"`
				Schema struct {
					OpenAPIV3Schema map[string]any `json:"openAPIV3Schema"`
				} `json:"schema"`
			} `json:"versions"`
		} `json:"spec"`
	}
	require.NoError(t, yaml.Unmarshal(data, &crd))

	versions := map[string]map[string]any{}
	for _, v := range crd.Spec.Versions {
		if v.Served {
			versions[v.Name] = v.Schema.OpenAPIV3Schema
		}
	}
	require.NotEmpty(t, versions, file)

	return versions
}

// propertyAt returns the schema of the property at path.
func propertyAt(schema map[string]any, path []string) map[string]any {
	for _, name := range path {
		properties, _ := schema["properties"].(map[string]any)
		schema, _ = properties[name].(map[string]any)
	}
	return schema
}

// collectSensitive appends the property paths of the schema that carry the marker.
func collectSensitive(schema map[string]any, path []string, out *[][]string) {
	if marked, _ := schema[sensitiveMarker].(bool); marked {
		*out = append(*out, slices.Clone(path))
	}

	properties, _ := schema["properties"].(map[string]any)
	for name, property := range properties {
		if child, ok := property.(map[string]any); ok {
			collectSensitive(child, append(slices.Clone(path), name), out)
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		collectSensitive(items, append(slices.Clone(path), "[]"), out)
	}
	if additional, ok := schema["additionalProperties"].(map[string]any); ok {
		collectSensitive(additional, append(slices.Clone(path), "{}"), out)
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		branches, _ := schema[key].([]any)
		for _, branch := range branches {
			if child, ok := branch.(map[string]any); ok {
				collectSensitive(child, path, out)
			}
		}
	}
}
