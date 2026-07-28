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
	"strings"
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

func pccScheme(secretPath string) *PCCCredentialsValidationAdapter {
	return &PCCCredentialsValidationAdapter{
		Validator: &KubeconfigValidator{},
		Schemes: []PCCSchemeConfig{
			{AuthScheme: cpapi.AuthSchemeKubeconfig, SecretPath: secretPath},
		},
	}
}

func pccMultiScheme(usernamePath, passwordPath, apiTokenPath string) *PCCCredentialsValidationAdapter {
	return &PCCCredentialsValidationAdapter{
		Validator: &CombinedCredentialValidator{
			ValidatorMap: map[cpapi.AuthScheme]CredentialsValidator{
				cpapi.AuthSchemeUserPassword: &UserPasswordValidator{},
				cpapi.AuthSchemeAPIToken:     &APITokenValidator{},
			},
		},
		Schemes: []PCCSchemeConfig{
			{AuthScheme: cpapi.AuthSchemeAPIToken, SecretPath: apiTokenPath},
			{AuthScheme: cpapi.AuthSchemeUserPassword, IdentityPath: usernamePath, SecretPath: passwordPath},
		},
	}
}

// --- Single-scheme tests ---

func TestValidateProviderClusterConfigNilState(t *testing.T) {
	t.Parallel()

	result := ValidateProviderClusterConfig(nil, pccScheme("provider.kubeconfigDataBase64"))
	if !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidateProviderClusterConfig(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

func TestValidateProviderClusterConfigEmptyPCC(t *testing.T) {
	t.Parallel()

	state := &cpvalapi.State{LegacyProviderClusterConfig: map[string]any{}}

	result := ValidateProviderClusterConfig(state, pccScheme("provider.kubeconfigDataBase64"))
	if result.HasErrors() {
		t.Fatalf("ValidateProviderClusterConfig() = %q, want no errors for empty PCC", result.Error())
	}
}

func TestValidateProviderClusterConfigRejectsMissingPath(t *testing.T) {
	t.Parallel()

	state := &cpvalapi.State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{"namespace": "d8"},
		},
	}

	result := ValidateProviderClusterConfig(state, pccScheme("provider.kubeconfigDataBase64"))
	if !hasViolationCode(result, CodePCCSecretPathEmpty) {
		t.Fatalf("ValidateProviderClusterConfig() = %q, want %s", result.Error(), CodePCCSecretPathEmpty)
	}
}

func TestValidateProviderClusterConfigWhitespaceValue(t *testing.T) {
	t.Parallel()

	state := &cpvalapi.State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{"kubeconfigDataBase64": "   "},
		},
	}

	result := ValidateProviderClusterConfig(state, pccScheme("provider.kubeconfigDataBase64"))
	if !result.HasErrors() || !strings.Contains(result.Error(), "required") {
		t.Fatalf("ValidateProviderClusterConfig() = %q, want 'required'", result.Error())
	}
}

func TestValidateProviderClusterConfigRejectsInvalidBase64(t *testing.T) {
	t.Parallel()

	state := &cpvalapi.State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{"kubeconfigDataBase64": "%%%"},
		},
	}

	result := ValidateProviderClusterConfig(state, pccScheme("provider.kubeconfigDataBase64"))
	if !result.HasErrors() || !strings.Contains(result.Error(), "invalid kubeconfig") {
		t.Fatalf("ValidateProviderClusterConfig() = %q, want 'invalid kubeconfig'", result.Error())
	}
}

func TestValidateProviderClusterConfigRejectsInvalidYAML(t *testing.T) {
	t.Parallel()

	b64 := base64.StdEncoding.EncodeToString([]byte("not-a-kubeconfig"))
	state := &cpvalapi.State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{"kubeconfigDataBase64": b64},
		},
	}

	result := ValidateProviderClusterConfig(state, pccScheme("provider.kubeconfigDataBase64"))
	if !result.HasErrors() || !strings.Contains(result.Error(), "invalid kubeconfig") {
		t.Fatalf("ValidateProviderClusterConfig() = %q, want 'invalid kubeconfig'", result.Error())
	}
}

func TestValidateProviderClusterConfigAcceptsValidKubeconfig(t *testing.T) {
	t.Parallel()

	state := &cpvalapi.State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{"kubeconfigDataBase64": validTestKubeconfigB64()},
		},
	}

	result := ValidateProviderClusterConfig(state, pccScheme("provider.kubeconfigDataBase64"))
	if result.HasErrors() {
		t.Fatalf("ValidateProviderClusterConfig() = %q, want no errors", result.Error())
	}
}

// --- Multi-scheme tests (userPassword + apiToken) ---

func TestValidateProviderClusterConfigMultiSchemeBothMissing(t *testing.T) {
	t.Parallel()

	state := &cpvalapi.State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{"namespace": "d8"},
		},
	}

	result := ValidateProviderClusterConfig(state, pccMultiScheme("provider.username", "provider.password", "provider.apiToken"))
	// userPassword: identityPath "provider.username" missing + secretPath "provider.password" missing = 2 errors
	// apiToken: secretPath "provider.apiToken" missing = 1 error
	// no scheme passed → all errors accumulated
	if !result.HasErrors() || len(result.Errors()) < 3 {
		t.Fatalf("ValidateProviderClusterConfig() = %d errors, want >= 3: %q", len(result.Errors()), result.Error())
	}
}

func TestValidateProviderClusterConfigMultiSchemeOneMissingOneValid1(t *testing.T) {
	t.Parallel()

	state := &cpvalapi.State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{
				"username": "admin",
				"password": "pass123",
			},
		},
	}

	// userPassword: identity + secret present → passes → no errors
	// apiToken: secretPath "provider.apiToken" missing, but userPassword passed → errors cleared
	result := ValidateProviderClusterConfig(state, pccMultiScheme("provider.username", "provider.password", "provider.apiToken"))
	if result.HasErrors() {
		t.Fatalf("ValidateProviderClusterConfig() = %q, want no errors (userPassword passed)", result.Error())
	}
}

func TestValidateProviderClusterConfigMultiSchemeOneMissingOneValid2(t *testing.T) {
	t.Parallel()

	state := &cpvalapi.State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{
				"apiToken": "token123",
			},
		},
	}

	// apiToken: secretPath "provider.apiToken" present → apiToken passes → no errors
	// userPassword: identity + secret missing, but apiToken passed → errors discarded
	result := ValidateProviderClusterConfig(state, pccMultiScheme("provider.username", "provider.password", "provider.apiToken"))
	if result.HasErrors() {
		t.Fatalf("ValidateProviderClusterConfig() = %q, want no errors (apiToken passed)", result.Error())
	}
}

func TestValidateProviderClusterConfigMultiSchemeBothValid(t *testing.T) {
	t.Parallel()

	state := &cpvalapi.State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{
				"username": "admin",
				"password": "pass123",
				"apiToken": "tok-abc",
			},
		},
	}

	// userPassword: identity + secret present, valid → no errors
	// apiToken: secret present → no errors
	result := ValidateProviderClusterConfig(state, pccMultiScheme("provider.username", "provider.password", "provider.apiToken"))
	if result.HasErrors() {
		t.Fatalf("ValidateProviderClusterConfig() = %q, want no errors", result.Error())
	}
}

func TestValidateProviderClusterConfigMultiSchemeOneMissingOneInheritableError(t *testing.T) {
	t.Parallel()

	state := &cpvalapi.State{
		LegacyProviderClusterConfig: map[string]any{
			"provider": map[string]any{
				"username": "",
				"password": "pass123",
			},
		},
	}

	// userPassword: identity + secret present, but username is invalid → 1 error
	// apiToken: secret missing → 1 error
	result := ValidateProviderClusterConfig(state, pccMultiScheme("provider.username", "provider.password", "provider.apiToken"))
	if !result.HasErrors() || len(result.Errors()) < 2 {
		t.Fatalf("ValidateProviderClusterConfig() = %d, want >= 2: %q", len(result.Errors()), result.Error())
	}
}

// --- Path remapping ---

func TestConvertToPCCViolationRemapsIdentityPath(t *testing.T) {
	t.Parallel()

	v := cpvalapi.Violation{
		Path:    "ProviderClusterConfiguration.data.identity",
		Code:    "secret_required",
		Message: "secret is required",
	}

	converted := convertToPCCViolation(v, "provider.accessKeyId", "provider.secretAccessKey")
	if converted.Path != "ProviderClusterConfiguration.provider.accessKeyId" {
		t.Fatalf("converted.Path = %q", converted.Path)
	}
	if converted.Code != "pcc_secret_required" {
		t.Fatalf("converted.Code = %q", converted.Code)
	}
	if converted.Message != "secret is required" {
		t.Fatalf("converted.Message = %q", converted.Message)
	}
}

func TestConvertToPCCViolationRemapsSecretPath(t *testing.T) {
	t.Parallel()

	v := cpvalapi.Violation{
		Path:    "ProviderClusterConfiguration.data.secret",
		Code:    "identity_required",
		Message: "identity is required",
	}

	converted := convertToPCCViolation(v, "provider.accessKeyId", "provider.secretAccessKey")
	if converted.Path != "ProviderClusterConfiguration.provider.secretAccessKey" {
		t.Fatalf("converted.Path = %q", converted.Path)
	}
	if converted.Code != "pcc_identity_required" {
		t.Fatalf("converted.Code = %q", converted.Code)
	}
}

func TestConvertToPCCViolationKeepsNonDataPath(t *testing.T) {
	t.Parallel()

	v := cpvalapi.Violation{
		Path:    "ProviderClusterConfiguration.data.otherField",
		Code:    "some_code",
		Message: "error",
	}

	converted := convertToPCCViolation(v, "provider.accessKeyId", "provider.secretAccessKey")
	if converted.Path != "ProviderClusterConfiguration.data.otherField" {
		t.Fatalf("converted.Path = %q, want unchanged", converted.Path)
	}
	if converted.Code != "pcc_some_code" {
		t.Fatalf("converted.Code = %q, want pcc_some_code", converted.Code)
	}
}

// --- Warnings → errors promotion ---

func TestConvertAndMergeIntoPreservesWarnings(t *testing.T) {
	t.Parallel()

	src := cpvalapi.Result{}
	src.AddWarning("ProviderClusterConfiguration.data.secret", "code_warn", nil, "warning msg")

	dst := cpvalapi.Result{}
	convertAndMergeInto(&dst, src, "", "provider.secret")

	if len(dst.Errors()) != 0 {
		t.Fatalf("dst.Errors() = %d, want 0", len(dst.Errors()))
	}
	if len(dst.Warnings()) != 1 {
		t.Fatalf("dst.Warnings() = %d, want 1", len(dst.Warnings()))
	}
	if dst.Warnings()[0].Code != "pcc_code_warn" {
		t.Fatalf("dst.Warnings()[0].Code = %q, want pcc_code_warn", dst.Warnings()[0].Code)
	}
}

// --- extractIdentityAndSecretData accumulates all errors ---

func TestExtractIdentityAndSecretDataBothMissing(t *testing.T) {
	t.Parallel()

	pcc := map[string]any{"other": "data"}
	scheme := PCCSchemeConfig{
		AuthScheme:   cpapi.AuthSchemeAccessKeyPair,
		IdentityPath: "provider.missingId",
		SecretPath:   "provider.missingSecret",
	}

	result := cpvalapi.Result{}
	identity, secret, found := extractIdentityAndSecretData(pcc, scheme, &result)

	if found {
		t.Fatal("found = true, want false")
	}
	if identity != "" || secret != "" {
		t.Fatalf("identity=%q secret=%q, want empty", identity, secret)
	}
	if len(result.Errors()) != 2 {
		t.Fatalf("result.Errors() = %d, want 2 (both paths missing)", len(result.Errors()))
	}
}

func TestExtractIdentityAndSecretDataOneMissingOneFound(t *testing.T) {
	t.Parallel()

	pcc := map[string]any{
		"provider": map[string]any{"accessKeyId": "AKID"},
	}
	scheme := PCCSchemeConfig{
		AuthScheme:   cpapi.AuthSchemeAccessKeyPair,
		IdentityPath: "provider.accessKeyId",
		SecretPath:   "provider.missingSecret",
	}

	result := cpvalapi.Result{}
	identity, _, found := extractIdentityAndSecretData(pcc, scheme, &result)

	if !found {
		t.Fatal("found = false, want true (identity was found)")
	}
	if identity != "AKID" {
		t.Fatalf("identity = %q, want AKID", identity)
	}
	if len(result.Errors()) != 1 {
		t.Fatalf("result.Errors() = %d, want 1 (secret missing)", len(result.Errors()))
	}
}

func TestLookupMapStringPath(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"provider": map[string]any{"kubeconfigDataBase64": "value"},
	}

	if got, ok := lookupMapStringPath(data, "provider.kubeconfigDataBase64"); !ok || got != "value" {
		t.Fatalf("lookupMapStringPath() = (%q, %v), want (value, true)", got, ok)
	}
	if _, ok := lookupMapStringPath(data, "provider.missing"); ok {
		t.Fatal("lookupMapStringPath() missing path = true, want false")
	}
}
