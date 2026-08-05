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
	"maps"
	"slices"
	"strings"

	"k8s.io/client-go/tools/clientcmd"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

// Validation violation codes for credential Secrets.
const (
	CodeCredentialSecretRequired       = "credential_secret_required"
	CodeAuthSchemeRequired             = "auth_scheme_required"
	CodeUnsupportedAuthScheme          = "unsupported_auth_scheme"
	CodeCredentialIdentityRequired     = "credential_identity_required"
	CodeCredentialSecretKeyRequired    = "credential_secret_key_required"
	CodeCredentialFieldRequired        = "credential_field_required"
	CodeCredentialIdentityUnsupported  = "credential_identity_unsupported"
	CodeCredentialSecretKeyUnsupported = "credential_secret_key_unsupported"
	CodeCredentialFieldUnsupported     = "credential_field_unsupported"
	CodeInvalidKubeconfigSecret        = "invalid_kubeconfig_secret"
	CodeInvalidServiceAccountSecret    = "invalid_service_account_secret"
)

// ValidateCredentialSecretPresence checks that primary credential Secret exists (before bootstrap or converge).
func ValidateCredentialSecretPresence[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
](state *cpvalapi.State[IC, S, PCC]) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	if !state.ExistsCredentialSecret(cpapi.CredentialSecretName) {
		result.AddError(
			fmt.Sprintf("Secret/%s", cpapi.CredentialSecretName),
			CodeCredentialSecretRequired,
			nil,
			fmt.Sprintf(`credential Secret %q is required`, cpapi.CredentialSecretName),
		)

		return result
	}

	return result
}

// ValidateCredentialSecretContent checks secret type and compliance of the structure with the given credential validator.
func ValidateCredentialSecretContent[
	IC cpapi.InstanceClassObject,
	S cpapi.ModuleSettingsObject,
	PCC cpapi.ProviderClusterConfigObject,
](state *cpvalapi.State[IC, S, PCC], validator CredentialsValidator) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	result := cpvalapi.Result{}

	for _, secret := range state.ListCredentialSecrets() {
		path := getNamedResourcePath("Secret", secret.Name)
		data := secret.NormalizedData()
		result.Merge(validator.Validate(path, data))
	}

	return result
}

// CredentialsValidator validates credential Secret data for a specific auth scheme.
type CredentialsValidator interface {
	Validate(path string, data map[string]string) cpvalapi.Result
}

// AccessKeyPairValidator validates credentials with AccessKeyPair auth scheme.
type AccessKeyPairValidator struct {
}

// Validate checks that identity and secret keys are present.
func (v *AccessKeyPairValidator) Validate(path string, data map[string]string) cpvalapi.Result {
	result := cpvalapi.Result{}

	authScheme := cpapi.AuthScheme(strings.TrimSpace(data[cpapi.CredentialSecretAuthSchemeKey]))

	if ok := validateAuthScheme(path, authScheme, cpapi.AuthSchemeAccessKeyPair, &result); !ok {
		return result
	}

	_ = validateRequiredCredentialKey(path, data, cpapi.CredentialSecretIdentityKey, authScheme, &result)
	_ = validateRequiredCredentialKey(path, data, cpapi.CredentialSecretSecretKey, authScheme, &result)

	return result
}

// APITokenValidator validates credentials with APIToken auth scheme.
type APITokenValidator struct {
}

// Validate checks that secret key is present and identity is absent.
func (v *APITokenValidator) Validate(path string, data map[string]string) cpvalapi.Result {
	result := cpvalapi.Result{}

	authScheme := cpapi.AuthScheme(strings.TrimSpace(data[cpapi.CredentialSecretAuthSchemeKey]))

	if ok := validateAuthScheme(path, authScheme, cpapi.AuthSchemeAPIToken, &result); !ok {
		return result
	}

	_ = validateUnsupportedCredentialKey(path, data, cpapi.CredentialSecretIdentityKey, authScheme, &result)
	_ = validateRequiredCredentialKey(path, data, cpapi.CredentialSecretSecretKey, authScheme, &result)

	return result
}

// AppSecretValidator validates credentials with AppSecret auth scheme.
type AppSecretValidator struct {
}

// Validate checks that secret key is present and identity is absent.
func (v *AppSecretValidator) Validate(path string, data map[string]string) cpvalapi.Result {
	result := cpvalapi.Result{}

	authScheme := cpapi.AuthScheme(strings.TrimSpace(data[cpapi.CredentialSecretAuthSchemeKey]))

	if ok := validateAuthScheme(path, authScheme, cpapi.AuthSchemeAppSecret, &result); !ok {
		return result
	}

	_ = validateUnsupportedCredentialKey(path, data, cpapi.CredentialSecretIdentityKey, authScheme, &result)
	_ = validateRequiredCredentialKey(path, data, cpapi.CredentialSecretSecretKey, authScheme, &result)

	return result
}

// ClientSecretValidator validates credentials with ClientSecret auth scheme.
type ClientSecretValidator struct {
}

// Validate checks that identity and secret keys are present.
func (v *ClientSecretValidator) Validate(path string, data map[string]string) cpvalapi.Result {
	result := cpvalapi.Result{}

	authScheme := cpapi.AuthScheme(strings.TrimSpace(data[cpapi.CredentialSecretAuthSchemeKey]))

	if ok := validateAuthScheme(path, authScheme, cpapi.AuthSchemeClientSecret, &result); !ok {
		return result
	}

	_ = validateRequiredCredentialKey(path, data, cpapi.CredentialSecretIdentityKey, authScheme, &result)
	_ = validateRequiredCredentialKey(path, data, cpapi.CredentialSecretSecretKey, authScheme, &result)

	return result
}

// KubeconfigValidator validates credentials with Kubeconfig auth scheme.
type KubeconfigValidator struct{}

// Validate checks that secret key is present (valid base64 kubeconfig), identity is absent.
func (v *KubeconfigValidator) Validate(path string, data map[string]string) cpvalapi.Result {
	result := cpvalapi.Result{}

	authScheme := cpapi.AuthScheme(strings.TrimSpace(data[cpapi.CredentialSecretAuthSchemeKey]))

	if ok := validateAuthScheme(path, authScheme, cpapi.AuthSchemeKubeconfig, &result); !ok {
		return result
	}

	_ = validateUnsupportedCredentialKey(path, data, cpapi.CredentialSecretIdentityKey, authScheme, &result)

	if ok := validateRequiredCredentialKey(path, data, cpapi.CredentialSecretSecretKey, authScheme, &result); !ok {
		return result
	}

	kubeconfigB64 := strings.TrimSpace(data[cpapi.CredentialSecretSecretKey])
	if err := ValidateKubeconfigBase64(kubeconfigB64); err != nil {
		result.AddError(
			fmt.Sprintf("%s.data.%s", path, cpapi.CredentialSecretSecretKey),
			CodeInvalidKubeconfigSecret,
			"masked",
			fmt.Sprintf("invalid kubeconfig: %v", err),
		)
	}

	return result
}

// ValidateKubeconfigBase64 decodes and validates a base64-encoded kubeconfig.
func ValidateKubeconfigBase64(kubeconfigB64 string) error {
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

// ServiceAccountValidator validates credentials with ServiceAccount auth scheme.
type ServiceAccountValidator struct {
	// ValidateContentFunc validates the service account JSON content.
	// When nil, only schema checks are performed.
	ValidateContentFunc func(string) error
}

// Validate checks that secret key is present, identity is absent, and optionally validates content.
func (v *ServiceAccountValidator) Validate(path string, data map[string]string) cpvalapi.Result {
	result := cpvalapi.Result{}

	authScheme := cpapi.AuthScheme(strings.TrimSpace(data[cpapi.CredentialSecretAuthSchemeKey]))

	if ok := validateAuthScheme(path, authScheme, cpapi.AuthSchemeServiceAccount, &result); !ok {
		return result
	}

	_ = validateUnsupportedCredentialKey(path, data, cpapi.CredentialSecretIdentityKey, authScheme, &result)

	if ok := validateRequiredCredentialKey(path, data, cpapi.CredentialSecretSecretKey, authScheme, &result); !ok {
		return result
	}

	if v.ValidateContentFunc == nil {
		return result
	}

	serviceAccount := strings.TrimSpace(data[cpapi.CredentialSecretSecretKey])
	if err := v.ValidateContentFunc(serviceAccount); err != nil {
		result.AddError(
			fmt.Sprintf("%s.data.%s", path, cpapi.CredentialSecretSecretKey),
			CodeInvalidServiceAccountSecret,
			"masked",
			fmt.Sprintf("invalid service account: %v", err),
		)
	}

	return result
}

// UserPasswordValidator validates credentials with UserPassword auth scheme.
type UserPasswordValidator struct {
}

// Validate checks that identity and secret keys are present.
func (v *UserPasswordValidator) Validate(path string, data map[string]string) cpvalapi.Result {
	result := cpvalapi.Result{}

	authScheme := cpapi.AuthScheme(strings.TrimSpace(data[cpapi.CredentialSecretAuthSchemeKey]))

	if ok := validateAuthScheme(path, authScheme, cpapi.AuthSchemeUserPassword, &result); !ok {
		return result
	}

	_ = validateRequiredCredentialKey(path, data, cpapi.CredentialSecretIdentityKey, authScheme, &result)
	_ = validateRequiredCredentialKey(path, data, cpapi.CredentialSecretSecretKey, authScheme, &result)

	return result
}

// CombinedCredentialValidator dispatches validation to a specific validator based on authScheme.
type CombinedCredentialValidator struct {
	ValidatorMap map[cpapi.AuthScheme]CredentialsValidator
}

// Validate selects a validator from ValidatorMap by authScheme or reports an unsupported scheme error.
func (v *CombinedCredentialValidator) Validate(path string, data map[string]string) cpvalapi.Result {
	result := cpvalapi.Result{}

	authScheme := cpapi.AuthScheme(strings.TrimSpace(data[cpapi.CredentialSecretAuthSchemeKey]))

	validator, ok := v.ValidatorMap[authScheme]
	if !ok {
		// Sorted: the list of allowed values must not change between runs.
		expectedAuthSchemes := slices.Sorted(maps.Keys(v.ValidatorMap))
		result.AddError(
			path+".data.authScheme",
			CodeUnsupportedAuthScheme,
			string(authScheme),
			fmt.Sprintf("authScheme %q is not allowed, expected %v", authScheme, expectedAuthSchemes),
		)
		return result
	}

	result.Merge(validator.Validate(path, data))

	return result
}

func validateAuthScheme(path string, actualAuthScheme, expectedAuthScheme cpapi.AuthScheme, result *cpvalapi.Result) bool {
	if actualAuthScheme == "" {
		result.AddError(path+".data.authScheme", CodeAuthSchemeRequired, nil, "authScheme is required")
		return false
	}

	if actualAuthScheme != expectedAuthScheme {
		result.AddError(
			path+".data.authScheme",
			CodeUnsupportedAuthScheme,
			string(actualAuthScheme),
			fmt.Sprintf("authScheme %q is not allowed, expected %q", actualAuthScheme, expectedAuthScheme),
		)
		return false
	}

	return true
}

func validateRequiredCredentialKey(path string, data map[string]string, key string, authScheme cpapi.AuthScheme, result *cpvalapi.Result) bool {
	if strings.TrimSpace(data[key]) != "" {
		return true
	}

	var code, message string
	switch key {
	case cpapi.CredentialSecretIdentityKey:
		code = CodeCredentialIdentityRequired
		message = fmt.Sprintf("identity is required for authScheme %q", authScheme)
	case cpapi.CredentialSecretSecretKey:
		code = CodeCredentialSecretKeyRequired
		message = fmt.Sprintf("secret is required for authScheme %q", authScheme)
	default:
		code = CodeCredentialFieldRequired
		message = fmt.Sprintf("%s is required for authScheme %q", key, authScheme)
	}

	result.AddError(fmt.Sprintf("%s.data.%s", path, key), code, nil, message)
	return false
}

func validateUnsupportedCredentialKey(path string, data map[string]string, key string, authScheme cpapi.AuthScheme, result *cpvalapi.Result) bool { //nolint:unparam
	if strings.TrimSpace(data[key]) == "" {
		return true
	}

	var code, message string
	switch key {
	case cpapi.CredentialSecretIdentityKey:
		code = CodeCredentialIdentityUnsupported
		message = fmt.Sprintf("identity is unsupported for authScheme %q", authScheme)
	case cpapi.CredentialSecretSecretKey:
		code = CodeCredentialSecretKeyUnsupported
		message = fmt.Sprintf("secret is unsupported for authScheme %q", authScheme)
	default:
		code = CodeCredentialFieldUnsupported
		message = fmt.Sprintf("%s is unsupported for authScheme %q", key, authScheme)
	}

	result.AddError(fmt.Sprintf("%s.data.%s", path, key), code, nil, message)
	return false
}
