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

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	"k8s.io/client-go/tools/clientcmd"
)

// ValidateCredentialSecrets checks managed credential Secrets against allowed auth schemes.
func ValidateCredentialSecrets(secrets []cpapi.CredentialSecret, allowedAuthSchemes []cpapi.AuthScheme) Result {
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
	case cpapi.AuthSchemeAccessKeyPair:
		ValidateAuthSchemeAccessKeyPairKeys(path, data, result)
	case cpapi.AuthSchemeUserPassword:
		ValidateAuthSchemeUserPasswordKeys(path, data, result)
	case cpapi.AuthSchemeAPIToken:
		ValidateAuthSchemeAPITokenKeys(path, data, result)
	case cpapi.AuthSchemeServiceAccount:
		ValidateAuthSchemeServiceAccountKeys(path, data, result)
	case cpapi.AuthSchemeClientSecret:
		ValidateAuthSchemeClientSecretKeys(path, data, result)
	case cpapi.AuthSchemeKubeconfig:
		ValidateAuthSchemeKubeconfigKeys(path, data, result)
	case cpapi.AuthSchemeAppSecret:
		ValidateAuthSchemeAppSecretKeys(path, data, result)
	}
}

// ValidateAuthSchemeAccessKeyPairKeys validates required fields for accessKeyPair credentials.
func ValidateAuthSchemeAccessKeyPairKeys(path string, data map[string]string, result *Result) {
	validateRequiredCredentialKey(path, data, "identity", cpapi.AuthSchemeAccessKeyPair, result)
	validateRequiredCredentialKey(path, data, "secret", cpapi.AuthSchemeAccessKeyPair, result)
}

// ValidateAuthSchemeUserPasswordKeys validates required fields for userPassword credentials.
func ValidateAuthSchemeUserPasswordKeys(path string, data map[string]string, result *Result) {
	validateRequiredCredentialKey(path, data, "identity", cpapi.AuthSchemeUserPassword, result)
	validateRequiredCredentialKey(path, data, "secret", cpapi.AuthSchemeUserPassword, result)
}

// ValidateAuthSchemeAPITokenKeys validates required fields for apiToken credentials.
func ValidateAuthSchemeAPITokenKeys(path string, data map[string]string, result *Result) {
	validateRequiredCredentialKey(path, data, "secret", cpapi.AuthSchemeAPIToken, result)
}

// ValidateAuthSchemeServiceAccountKeys validates required fields for serviceAccount credentials.
func ValidateAuthSchemeServiceAccountKeys(path string, data map[string]string, result *Result) {
	validateRequiredCredentialKey(path, data, "secret", cpapi.AuthSchemeServiceAccount, result)
}

// ValidateAuthSchemeClientSecretKeys validates required fields for clientSecret credentials.
func ValidateAuthSchemeClientSecretKeys(path string, data map[string]string, result *Result) {
	validateRequiredCredentialKey(path, data, "identity", cpapi.AuthSchemeClientSecret, result)
	validateRequiredCredentialKey(path, data, "secret", cpapi.AuthSchemeClientSecret, result)
}

// ValidateAuthSchemeKubeconfigKeys validates required fields and kubeconfig format.
func ValidateAuthSchemeKubeconfigKeys(path string, data map[string]string, result *Result) {
	validateRequiredCredentialKey(path, data, "secret", cpapi.AuthSchemeKubeconfig, result)

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
}

// ValidateAuthSchemeAppSecretKeys validates required fields for appSecret credentials.
func ValidateAuthSchemeAppSecretKeys(path string, data map[string]string, result *Result) {
	validateRequiredCredentialKey(path, data, "identity", cpapi.AuthSchemeAppSecret, result)
	validateRequiredCredentialKey(path, data, "secret", cpapi.AuthSchemeAppSecret, result)
}

func validateRequiredCredentialKey(path string, data map[string]string, key string, authScheme cpapi.AuthScheme, result *Result) {
	if strings.TrimSpace(data[key]) != "" {
		return
	}

	result.AddError(
		path+".data."+key,
		"required_credential_field",
		fmt.Sprintf("%s is required for authScheme %q", key, authScheme),
	)
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
