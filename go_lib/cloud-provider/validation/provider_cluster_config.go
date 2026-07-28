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
	"fmt"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

const (
	// pccPath is the base path used for legacy ProviderClusterConfiguration validation messages.
	pccPath = "ProviderClusterConfiguration"

	// Validation violation codes for ProviderClusterConfiguration.
	CodePCCIdentityPathEmpty = "pcc_identity_path_empty"
	CodePCCSecretPathEmpty   = "pcc_secret_path_empty"
)

// ValidateProviderClusterConfig validates legacy ProviderClusterConfiguration using the given adapter.
func ValidateProviderClusterConfig(state *cpvalapi.State, adapter *PCCCredentialsValidationAdapter) cpvalapi.Result {
	if state == nil {
		return cpvalapi.ResultForNilState()
	}

	return adapter.Validate(state.LegacyProviderClusterConfig)
}

// PCCSchemeConfig maps an auth scheme to the PCC key paths where its credentials are stored.
type PCCSchemeConfig struct {
	AuthScheme   cpapi.AuthScheme
	IdentityPath string
	SecretPath   string
}

// PCCCredentialsValidationAdapter adapts a CredentialsValidator to validate credentials stored in
// legacy ProviderClusterConfiguration (PCC) settings. It iterates over configured Schemes,
// extracts identity and secret values from the PCC map, and delegates to the validator.
type PCCCredentialsValidationAdapter struct {
	Validator CredentialsValidator
	Schemes   []PCCSchemeConfig
}

// Validate iterates all configured schemes. If any scheme validates successfully
// (data found + validator returns no errors), the result has no errors — only warnings
// from that scheme. If no scheme passes, all accumulated errors are returned.
func (a *PCCCredentialsValidationAdapter) Validate(pcc map[string]any) cpvalapi.Result {
	if len(pcc) == 0 {
		return cpvalapi.Result{}
	}

	var commonResult cpvalapi.Result

	for _, scheme := range a.Schemes {
		identity, secret, ok := extractIdentityAndSecretData(pcc, scheme, &commonResult)
		if !ok {
			continue
		}

		secretData := map[string]string{
			CredentialSecretAuthSchemeKey: string(scheme.AuthScheme),
			CredentialSecretIdentityKey:   identity,
			CredentialSecretSecretKey:     secret,
		}

		result := a.Validator.Validate(pccPath, secretData)

		if result.HasErrors() {
			convertAndMergeInto(&commonResult, result, scheme.IdentityPath, scheme.SecretPath)
			continue
		}

		passedResult := cpvalapi.Result{}
		convertAndMergeInto(&passedResult, result, scheme.IdentityPath, scheme.SecretPath)

		return passedResult
	}

	return commonResult
}

// extractIdentityAndSecretData tries to extract identity and secret from the PCC map for the
// given scheme. Missing paths produce errors accumulated into result. All errors are collected
// — execution never returns early on the first missing path.
// Returns found=true when at least one of the configured paths returned data.
func extractIdentityAndSecretData(pcc map[string]any, scheme PCCSchemeConfig, result *cpvalapi.Result) (string, string, bool) {
	var identity, secret string
	identityFound := false
	secretFound := false

	if scheme.IdentityPath != "" {
		var ok bool
		identity, ok = lookupMapStringPath(pcc, scheme.IdentityPath)
		if ok {
			identityFound = true
		} else {
			result.AddError(
				fmt.Sprintf("%s.%s", pccPath, scheme.IdentityPath),
				CodePCCIdentityPathEmpty,
				"",
				fmt.Sprintf("%s must be set", scheme.IdentityPath),
			)
		}
	}

	if scheme.SecretPath != "" {
		var ok bool
		secret, ok = lookupMapStringPath(pcc, scheme.SecretPath)
		if ok {
			secretFound = true
		} else {
			result.AddError(
				fmt.Sprintf("%s.%s", pccPath, scheme.SecretPath),
				CodePCCSecretPathEmpty,
				"",
				fmt.Sprintf("%s must be set", scheme.SecretPath),
			)
		}
	}

	return identity, secret, identityFound || secretFound
}

// convertAndMergeInto remaps validated Secret paths back to PCC paths with a "pcc_" code
// prefix, then merges all violations into dst.
func convertAndMergeInto(dst *cpvalapi.Result, src cpvalapi.Result, identityPath, secretPath string) {
	for _, violation := range src.Errors() {
		violation = convertToPCCViolation(violation, identityPath, secretPath)
		dst.AddError(violation.Path, violation.Code, violation.Value, violation.Message)
	}

	for _, violation := range src.Warnings() {
		violation = convertToPCCViolation(violation, identityPath, secretPath)
		dst.AddWarning(violation.Path, violation.Code, violation.Value, violation.Message)
	}
}

func convertToPCCViolation(violation cpvalapi.Violation, identityPath, secretPath string) cpvalapi.Violation {
	identityKey := fmt.Sprintf("%s.data.%s", pccPath, CredentialSecretIdentityKey)
	secretKey := fmt.Sprintf("%s.data.%s", pccPath, CredentialSecretSecretKey)

	switch violation.Path {
	case identityKey:
		violation.Path = fmt.Sprintf("%s.%s", pccPath, identityPath)
	case secretKey:
		violation.Path = fmt.Sprintf("%s.%s", pccPath, secretPath)
	}

	violation.Code = fmt.Sprintf("pcc_%s", violation.Code)

	return violation
}
