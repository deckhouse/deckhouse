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
	"testing"

	"github.com/stretchr/testify/require"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func userOperationSchemas(t *testing.T) map[string]*apiextensions.JSONSchemaProps {
	t.Helper()
	data, err := os.ReadFile("user-operation.yaml")
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

// spec.target names a user of an external provider. A user of an LDAP or Crowd provider can have a second factor, so
// Reset2FA takes a target as Lock, Unlock and Logout do; ResetPassword and RequirePasswordChange stay local.
func TestUserOperationTarget(t *testing.T) {
	target := map[string]any{"connectorID": "corp-ldap", "email": "jane@example.org"}

	for version, schema := range userOperationSchemas(t) {
		// v1alpha1 has no target: its operations name a local user.
		if version != "v1" {
			continue
		}
		for _, tc := range []struct {
			name  string
			spec  map[string]any
			valid bool
		}{
			{"Reset2FA of a target", map[string]any{"type": "Reset2FA", "target": target}, true},
			{"Reset2FA of a local user", map[string]any{"type": "Reset2FA", "user": "jane"}, true},
			{"Lock of a target", map[string]any{"type": "Lock", "target": target, "lock": map[string]any{"for": "1h"}}, true},
			{"Unlock of a target", map[string]any{"type": "Unlock", "target": target}, true},
			{"Logout of a target", map[string]any{"type": "Logout", "target": target}, true},
			{"ResetPassword of a target", map[string]any{"type": "ResetPassword", "target": target}, false},
			{"RequirePasswordChange of a target", map[string]any{"type": "RequirePasswordChange", "target": target}, false},
			{"Reset2FA of a user and a target", map[string]any{"type": "Reset2FA", "user": "jane", "target": target}, false},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				object := map[string]any{
					"apiVersion": "deckhouse.io/" + version,
					"kind":       "UserOperation",
					"metadata":   map[string]any{"name": "op"},
					"spec":       tc.spec,
				}
				validateAuthenticator(t, schema, object, tc.valid)
			})
		}
	}
}
