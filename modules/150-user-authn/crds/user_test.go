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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"sigs.k8s.io/yaml"
)

// maxUserEmailBytes is the longest email Dex can name the Password of a local user after (see
// naming.MaxLocalEmailBytes of the user-authn controller).
const maxUserEmailBytes = 150

func userSchemas(t *testing.T) map[string]*apiextensions.JSONSchemaProps {
	t.Helper()
	data, err := os.ReadFile("user.yaml")
	require.NoError(t, err)
	var crd apiextensionsv1.CustomResourceDefinition
	require.NoError(t, yaml.Unmarshal(data, &crd))
	schemas := make(map[string]*apiextensions.JSONSchemaProps)
	for _, version := range crd.Spec.Versions {
		if !version.Served {
			continue
		}
		schema := new(apiextensions.JSONSchemaProps)
		require.NoError(t, apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(version.Schema.OpenAPIV3Schema, schema, nil))
		structural, err := structuralschema.NewStructural(schema)
		require.NoError(t, err)
		require.Empty(t, structuralschema.ValidateStructural(nil, structural), version.Name)
		schemas[version.Name] = schema
	}
	require.Len(t, schemas, 2, "every served version of User is checked")
	return schemas
}

func userObject(version, email string) map[string]any {
	return map[string]any{
		"apiVersion": "deckhouse.io/" + version,
		"kind":       "User",
		"metadata":   map[string]any{"name": "jane"},
		"spec": map[string]any{
			"email":    email,
			"password": "JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H",
		},
	}
}

// emailOfBytes is an email of n bytes: the local part repeated, at example.com.
func emailOfBytes(n int, local string) string {
	const domain = "@example.com"
	return strings.Repeat(local, (n-len(domain))/len(local)) + domain
}

// The email is counted in bytes, as Dex names the Password: a letter outside ASCII counts twice.
func TestUserEmailLength(t *testing.T) {
	tests := []struct {
		name  string
		email string
		valid bool
	}{
		{name: "at the limit", email: emailOfBytes(maxUserEmailBytes, "a"), valid: true},
		{name: "one byte above the limit", email: emailOfBytes(maxUserEmailBytes+1, "a")},
		{name: "the longest address of RFC 5321", email: emailOfBytes(254, "a")},
		{name: "two-byte letters at the limit", email: emailOfBytes(maxUserEmailBytes, "ü"), valid: true},
		{name: "two-byte letters above the limit in bytes, not in letters", email: emailOfBytes(maxUserEmailBytes+2, "ü")},
	}
	for version, schema := range userSchemas(t) {
		for _, tc := range tests {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				errors := validateAuthenticator(t, schema, userObject(version, tc.email), tc.valid)
				if tc.valid {
					return
				}
				var details []string
				for _, err := range errors {
					details = append(details, err.Detail)
				}
				require.Contains(t, strings.Join(details, "\n"), "The email must be at most 150 bytes long")
			})
		}
	}
}

// A User created before the limit, with a longer email, can still be updated while its email stays.
func TestUserEmailLengthRatchets(t *testing.T) {
	long := emailOfBytes(254, "a")
	for version, schema := range userSchemas(t) {
		t.Run(version, func(t *testing.T) {
			old := userObject(version, long)
			updated := userObject(version, long)
			updated["spec"].(map[string]any)["ttl"] = "24h"
			require.Empty(t, validateAuthenticatorUpdate(t, schema, updated, old))
		})
	}
}
