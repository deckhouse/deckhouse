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
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
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
			// reset2FA.methods narrows a Reset2FA to some of the methods. A method listed twice is refused by the API
			// server (x-kubernetes-list-type: set), which this validation does not run.
			{"Reset2FA of the key", map[string]any{"type": "Reset2FA", "target": target, "reset2FA": map[string]any{"methods": []any{"WebAuthn"}}}, true},
			{"Reset2FA of both", map[string]any{"type": "Reset2FA", "user": "jane", "reset2FA": map[string]any{"methods": []any{"TOTP", "WebAuthn"}}}, true},
			{"Reset2FA of no method", map[string]any{"type": "Reset2FA", "user": "jane", "reset2FA": map[string]any{"methods": []any{}}}, false},
			{"Reset2FA of an unknown method", map[string]any{"type": "Reset2FA", "user": "jane", "reset2FA": map[string]any{"methods": []any{"SMS"}}}, false},
			{"reset2FA of a Logout", map[string]any{"type": "Logout", "user": "jane", "reset2FA": map[string]any{"methods": []any{"TOTP"}}}, false},
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

// resetPassword.requirePasswordChange defaults to true: an operation that does not set it makes the
// user change the password, as every reset did before the field.
func TestUserOperationResetPasswordRequirePasswordChange(t *testing.T) {
	const rawBcryptHash = "$2y$10$dMXeFQPduGXaS2X1OpgYvOGK/5.Gl6nluOfRHg5iPt8nJXzK8iy.G"
	schema := userOperationSchemas(t)["v1"]
	structural, err := structuralschema.NewStructural(schema)
	require.NoError(t, err)

	for _, tc := range []struct {
		name     string
		reset    map[string]any
		expected any
		valid    bool
	}{
		{name: "unset defaults to true", reset: map[string]any{"newPasswordHash": rawBcryptHash}, expected: true, valid: true},
		{name: "false kept", reset: map[string]any{"newPasswordHash": rawBcryptHash, "requirePasswordChange": false}, expected: false, valid: true},
		{name: "true kept", reset: map[string]any{"newPasswordHash": rawBcryptHash, "requirePasswordChange": true}, expected: true, valid: true},
		{name: "not a boolean", reset: map[string]any{"newPasswordHash": rawBcryptHash, "requirePasswordChange": "no"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			object := map[string]any{
				"apiVersion": "deckhouse.io/v1",
				"kind":       "UserOperation",
				"metadata":   map[string]any{"name": "op"},
				"spec":       map[string]any{"type": "ResetPassword", "user": "jane", "resetPassword": tc.reset},
			}
			defaulting.Default(object, structural)
			validateAuthenticator(t, schema, object, tc.valid)
			if tc.valid {
				require.Equal(t, tc.expected, object["spec"].(map[string]any)["resetPassword"].(map[string]any)["requirePasswordChange"])
			}
		})
	}
}

// newPasswordHash is a raw bcrypt hash. A Base64-encoded hash or a hash whose $2y$10$ prefix the
// shell has replaced (a heredoc without quotes around EOF) is refused when the operation is applied,
// in every version, with the cause in the message.
func TestUserOperationNewPasswordHashIsRawBcrypt(t *testing.T) {
	const message = "newPasswordHash must be a raw bcrypt hash starting with $2"
	for version, schema := range userOperationSchemas(t) {
		for _, tc := range []struct {
			name  string
			hash  string
			valid bool
		}{
			{name: "raw bcrypt", hash: "$2y$10$dMXeFQPduGXaS2X1OpgYvOGK/5.Gl6nluOfRHg5iPt8nJXzK8iy.G", valid: true},
			{name: "Base64 of bcrypt", hash: "JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H"},
			{name: "plain password", hash: "dh-test-secret-0123456789abcdef"},
			{name: "empty", hash: ""},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				object := map[string]any{
					"apiVersion": "deckhouse.io/" + version,
					"kind":       "UserOperation",
					"metadata":   map[string]any{"name": "op"},
					"spec": map[string]any{
						"type":          "ResetPassword",
						"user":          "jane",
						"resetPassword": map[string]any{"newPasswordHash": tc.hash},
					},
				}
				errors := validateAuthenticator(t, schema, object, tc.valid)
				if tc.valid {
					return
				}
				require.Len(t, errors, 1)
				require.Contains(t, errors[0].Error(), message)
				require.Contains(t, errors[0].Error(), "<<'EOF'")
			})
		}
	}
}
