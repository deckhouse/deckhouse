// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"encoding/base64"
	"fmt"
	"strings"

	"k8s.io/client-go/tools/clientcmd"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

// ValidateCredentialSecretPresence checks that primary credential Secret exists before bootstrap or converge.
func ValidateCredentialSecretPresence(state *State) Result {
	result := Result{}
	if state == nil {
		return result
	}

	secret, ok := findCredentialSecret(state, cpapi.CredentialSecretName)
	if !ok {
		result.AddError(
			"Secret/"+cpapi.CredentialSecretName,
			"credential_secret_required",
			fmt.Sprintf(`credential Secret %q is required`, cpapi.CredentialSecretName),
		)

		return result
	}

	if secret.Type != cpapi.CredentialsSecretType {
		result.AddError(
			fmt.Sprintf("Secret/%s.type", secret.Name),
			"invalid_credential_secret_type",
			fmt.Sprintf("credential Secret type must be %q", cpapi.CredentialsSecretType),
		)
	}

	return result
}

// ValidateCredentialSecretContent checks semantic validity of managed credential Secrets.
func ValidateCredentialSecretContent(state *State, allowedAuthSchemes []cpapi.AuthScheme) Result {
	result := Result{}
	if state == nil {
		return result
	}

	secrets := getManagedCredentialSecrets(state)
	for _, secret := range secrets {
		if secret.Type != cpapi.CredentialsSecretType {
			result.AddError(
				fmt.Sprintf("Secret/%s.type", secret.Name),
				"invalid_credential_secret_type",
				fmt.Sprintf("credential Secret type must be %q", cpapi.CredentialsSecretType),
			)
		}
	}

	result.Merge(
		validateCredentialSecrets(secrets, allowedAuthSchemes),
	)

	return result
}

func validateCredentialSecrets(secrets []cpapi.CredentialSecret, allowedAuthSchemes []cpapi.AuthScheme) Result {
	result := Result{}

	allowed := make(map[cpapi.AuthScheme]struct{}, len(allowedAuthSchemes))
	for _, scheme := range allowedAuthSchemes {
		allowed[scheme] = struct{}{}
	}

	for _, secret := range secrets {
		path := namedResourcePath("Secret", secret.Name)
		data := secret.NormalizedData()
		authScheme := cpapi.AuthScheme(strings.TrimSpace(data["authScheme"]))

		if authScheme == "" {
			result.AddError(path+".data.authScheme", "auth_scheme_required", "authScheme is required")
			continue
		}

		if _, ok := allowed[authScheme]; !ok {
			result.AddError(
				path+".data.authScheme",
				"unsupported_auth_scheme",
				fmt.Sprintf("authScheme %q is not allowed", authScheme),
			)
			continue
		}

		validateAuthSchemeKeys(path, data, authScheme, &result)
	}

	return result
}

func validateAuthSchemeKeys(path string, data map[string]string, authScheme cpapi.AuthScheme, result *Result) {
	switch authScheme {
	case cpapi.AuthSchemeAccessKeyPair, cpapi.AuthSchemeUserPassword, cpapi.AuthSchemeClientSecret, cpapi.AuthSchemeAppSecret:
		validateRequiredCredentialKey(path, data, "identity", authScheme, result)
		validateRequiredCredentialKey(path, data, "secret", authScheme, result)
	case cpapi.AuthSchemeAPIToken, cpapi.AuthSchemeServiceAccount:
		validateRequiredCredentialKey(path, data, "secret", authScheme, result)
	case cpapi.AuthSchemeKubeconfig:
		validateRequiredCredentialKey(path, data, "secret", authScheme, result)

		secret := strings.TrimSpace(data["secret"])
		if secret == "" {
			return
		}

		if err := validateKubeconfigBase64(secret); err != nil {
			result.AddError(
				path+".data.secret",
				"invalid_kubeconfig_secret",
				"secret must contain base64-encoded kubeconfig",
			)
		}
	default:
		result.AddError(
			path+".data.authScheme",
			"unsupported_auth_scheme",
			fmt.Sprintf("authScheme %q is not allowed", authScheme),
		)
	}
}

func validateRequiredCredentialKey(path string, data map[string]string, key string, authScheme cpapi.AuthScheme, result *Result) {
	if strings.TrimSpace(data[key]) != "" {
		return
	}

	var code, message string
	switch key {
	case "identity":
		code = "credential_identity_required"
		message = fmt.Sprintf("identity is required for authScheme %q", authScheme)
	case "secret":
		code = "credential_secret_key_required"
		message = fmt.Sprintf("secret is required for authScheme %q", authScheme)
	default:
		code = "credential_field_required"
		message = fmt.Sprintf("%s is required for authScheme %q", key, authScheme)
	}

	result.AddError(path+".data."+key, code, message)
}

func validateKubeconfigBase64(kubeconfigB64 string) error {
	kubeconfigBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(kubeconfigB64))
	if err != nil {
		return fmt.Errorf("decode kubeconfig: %w", err)
	}

	cfg, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return fmt.Errorf("parse kubeconfig: %w", err)
	}

	if err := clientcmd.Validate(*cfg); err != nil {
		return fmt.Errorf("validate kubeconfig: %w", err)
	}

	return nil
}

func namedResourcePath(kind, name string) string {
	if name == "" {
		return kind
	}

	return fmt.Sprintf("%s/%s", kind, name)
}
