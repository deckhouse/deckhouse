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
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
)

func validTestKubeconfigB64() string {
	return "YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnCmNsdXN0ZXJzOgotIG5hbWU6IHRlc3QKICBjbHVzdGVyOgogICAgc2VydmVyOiBodHRwczovLzEyNy4wLjAuMTo2NDQzCiAgICBpbnNlY3VyZS1za2lwLXRscy12ZXJpZnk6IHRydWUKY29udGV4dHM6Ci0gbmFtZTogdGVzdAogIGNvbnRleHQ6CiAgICBjbHVzdGVyOiB0ZXN0CiAgICB1c2VyOiB0ZXN0CmN1cnJlbnQtY29udGV4dDogdGVzdAp1c2VyczoKLSBuYW1lOiB0ZXN0CiAgdXNlcjoKICAgIHRva2VuOiB0ZXN0LXRva2Vu"
}

func managedCredentialSecret(name, namespace string, data cpapi.CredentialSecretStringData) cpapi.CredentialSecret {
	return cpapi.CredentialSecret{
		ObjectMeta: cpapi.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type:       cpapi.CredentialsSecretType,
		StringData: data,
	}
}

func allSchemesValidator() CredentialsValidator {
	return &CombinedCredentialValidator{
		ValidatorMap: map[cpapi.AuthScheme]CredentialsValidator{
			cpapi.AuthSchemeAPIToken:       &APITokenValidator{},
			cpapi.AuthSchemeAccessKeyPair:  &AccessKeyPairValidator{},
			cpapi.AuthSchemeUserPassword:   &UserPasswordValidator{},
			cpapi.AuthSchemeClientSecret:   &ClientSecretValidator{},
			cpapi.AuthSchemeAppSecret:      &AppSecretValidator{},
			cpapi.AuthSchemeKubeconfig:     &KubeconfigValidator{},
			cpapi.AuthSchemeServiceAccount: &ServiceAccountValidator{},
		},
	}
}

func TestValidateCredentialSecretContentNilState(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(nil, &APITokenValidator{})
	if !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidateCredentialSecretContent(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

func TestValidateCredentialSecretContentAllowsConfiguredAuthScheme(t *testing.T) {
	t.Parallel()

	secret := managedCredentialSecret(
		cpapi.CredentialSecretName,
		testNamespace,
		cpapi.CredentialSecretStringData{
			AuthScheme: cpapi.AuthSchemeAPIToken,
			Secret:     "token-123",
		},
	)

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{secret}),
		&APITokenValidator{},
	)

	if result.HasErrors() {
		t.Fatalf("ValidateCredentialSecretContent() unexpected errors: %s", result.Error())
	}
}

func TestValidateCredentialSecretContentRejectsUnsupportedAuthScheme(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeAPIToken,
				Secret:     "token-123",
			},
		)}),
		&KubeconfigValidator{},
	)

	if !result.HasErrors() || !strings.Contains(result.Error(), "is not allowed") {
		t.Fatalf("ValidateCredentialSecretContent() expected unsupported auth scheme error, got: %s", result.Error())
	}

	for _, violation := range result.Errors() {
		if violation.Code == CodeUnsupportedAuthScheme && violation.Value != string(cpapi.AuthSchemeAPIToken) {
			t.Fatalf("unsupported_auth_scheme Value = %#v, want %q", violation.Value, cpapi.AuthSchemeAPIToken)
		}
	}
}

func TestValidateCredentialSecretContentIgnoresNonCredentialSecrets(t *testing.T) {
	t.Parallel()

	// Non-managed secrets (not of CredentialsSecretType) are filtered by
	// ListCredentialSecrets and therefore never reach the validator.
	state := credentialContentState([]cpapi.CredentialSecret{
		{
			ObjectMeta: cpapi.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: testNamespace},
			Type:       "kubernetes.io/tls",
			StringData: cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeAPIToken,
				Secret:     "not-used",
			},
		},
	})

	result := ValidateCredentialSecretContent(state, &APITokenValidator{})
	if result.HasErrors() {
		t.Fatalf("ValidateCredentialSecretContent() = %q, want non-credential secrets ignored", result.Error())
	}
}

func TestKubeconfigValidatorAcceptsValidKubeconfig(t *testing.T) {
	t.Parallel()

	kubeconfigB64 := base64.StdEncoding.EncodeToString([]byte(`apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user:
    token: test-token`))

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeKubeconfig,
				Secret:     kubeconfigB64,
			},
		)}),
		&KubeconfigValidator{},
	)

	if result.HasErrors() {
		t.Fatalf("ValidateCredentialSecretContent() unexpected errors: %s", result.Error())
	}
}

func TestKubeconfigValidatorRejectsInvalidBase64(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeKubeconfig,
				Secret:     "not-base64",
			},
		)}),
		&KubeconfigValidator{},
	)

	if !result.HasErrors() || !strings.Contains(result.Error(), "invalid kubeconfig") {
		t.Fatalf("ValidateCredentialSecretContent() expected invalid kubeconfig error, got: %s", result.Error())
	}
}

func TestKubeconfigValidatorRejectsInvalidYAML(t *testing.T) {
	t.Parallel()

	invalid := base64.StdEncoding.EncodeToString([]byte("not-a-kubeconfig"))
	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeKubeconfig,
				Secret:     invalid,
			},
		)}),
		&KubeconfigValidator{},
	)

	if !result.HasErrors() || !strings.Contains(result.Error(), "invalid kubeconfig") {
		t.Fatalf("ValidateCredentialSecretContent() = %q", result.Error())
	}
}

func TestServiceAccountValidatorRequiresSecret(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeServiceAccount},
		)}),
		&ServiceAccountValidator{},
	)

	if !result.HasErrors() || !hasViolationCode(result, CodeCredentialSecretKeyRequired) {
		t.Fatalf("ValidateCredentialSecretContent() errors = %#v, want %s", result.Errors(), CodeCredentialSecretKeyRequired)
	}
}

func TestServiceAccountValidatorAcceptsSecret(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeServiceAccount,
				Secret:     `{"type":"service_account"}`,
			},
		)}),
		&ServiceAccountValidator{},
	)

	if result.HasErrors() {
		t.Fatalf("ValidateCredentialSecretContent() unexpected errors: %s", result.Error())
	}
}

func TestValidateCredentialSecretContentRequiresAuthScheme(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{
			managedCredentialSecret(cpapi.CredentialSecretName, testNamespace, cpapi.CredentialSecretStringData{}),
		}),
		&APITokenValidator{},
	)

	if !result.HasErrors() || !strings.Contains(result.Error(), "authScheme is required") {
		t.Fatalf("ValidateCredentialSecretContent() = %q, want authScheme required", result.Error())
	}
}

func TestValidateCredentialSecretContentIgnoresOrdinaryModuleSecrets(t *testing.T) {
	t.Parallel()

	state := credentialContentState([]cpapi.CredentialSecret{
		managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeKubeconfig,
				Secret:     validTestKubeconfigB64(),
			},
		),
		{
			ObjectMeta: cpapi.ObjectMeta{Name: "validation-webhook-tls", Namespace: testNamespace},
			Type:       "kubernetes.io/tls",
		},
	})

	if result := ValidateCredentialSecretContent(state, &KubeconfigValidator{}); result.HasErrors() {
		t.Fatalf("ValidateCredentialSecretContent() unexpected errors: %s", result.Error())
	}
}

func TestValidateCredentialSecretContentIgnoresOtherNamespace(t *testing.T) {
	t.Parallel()

	state := credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
		cpapi.CredentialSecretName,
		"other",
		cpapi.CredentialSecretStringData{AuthScheme: "invalid"},
	)})

	if result := ValidateCredentialSecretContent(state, &APITokenValidator{}); result.HasErrors() {
		t.Fatalf("ValidateCredentialSecretContent() = %q, want other namespace secret ignored", result.Error())
	}
}

func TestValidateRequiredCredentialKeyErrorCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		key      string
		wantCode string
	}{
		{
			name:     "identity",
			key:      "identity",
			wantCode: CodeCredentialIdentityRequired,
		},
		{
			name:     "secret",
			key:      "secret",
			wantCode: CodeCredentialSecretKeyRequired,
		},
		{
			name:     "custom field",
			key:      "customKey",
			wantCode: CodeCredentialFieldRequired,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := cpvalapi.Result{}
			validateRequiredCredentialKey(
				fmt.Sprintf("Secret/%s", cpapi.CredentialSecretName),
				map[string]string{},
				tt.key,
				cpapi.AuthSchemeAPIToken,
				&result,
			)

			if !hasViolationCode(result, tt.wantCode) {
				t.Fatalf("validateRequiredCredentialKey() errors = %#v, want code %q", result.Errors(), tt.wantCode)
			}
		})
	}
}

func TestAccessKeyPairValidatorRequiresIdentity(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeAccessKeyPair},
		)}),
		&AccessKeyPairValidator{},
	)

	if !result.HasErrors() || !hasViolationCode(result, CodeCredentialIdentityRequired) {
		t.Fatalf("errors = %#v, want %s", result.Errors(), CodeCredentialIdentityRequired)
	}
}

func TestAccessKeyPairValidatorRequiresSecret(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeAccessKeyPair,
				Identity:   "access-key-id",
			},
		)}),
		&AccessKeyPairValidator{},
	)

	if !result.HasErrors() || !hasViolationCode(result, CodeCredentialSecretKeyRequired) {
		t.Fatalf("errors = %#v, want %s", result.Errors(), CodeCredentialSecretKeyRequired)
	}
}

func TestUserPasswordValidatorRequiresSecret(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeUserPassword,
				Identity:   "user",
			},
		)}),
		&UserPasswordValidator{},
	)

	if !result.HasErrors() || !hasViolationCode(result, CodeCredentialSecretKeyRequired) {
		t.Fatalf("errors = %#v, want %s", result.Errors(), CodeCredentialSecretKeyRequired)
	}
}

func TestAPITokenValidatorRequiresSecret(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeAPIToken},
		)}),
		&APITokenValidator{},
	)

	if !result.HasErrors() || !hasViolationCode(result, CodeCredentialSecretKeyRequired) {
		t.Fatalf("errors = %#v, want %s", result.Errors(), CodeCredentialSecretKeyRequired)
	}
}

func TestClientSecretValidatorRequiresIdentity(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeClientSecret},
		)}),
		&ClientSecretValidator{},
	)

	if !result.HasErrors() || !hasViolationCode(result, CodeCredentialIdentityRequired) {
		t.Fatalf("errors = %#v, want %s", result.Errors(), CodeCredentialIdentityRequired)
	}
}

func TestClientSecretValidatorRequiresSecret(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeClientSecret,
				Identity:   "client-id",
			},
		)}),
		&ClientSecretValidator{},
	)

	if !result.HasErrors() || !hasViolationCode(result, CodeCredentialSecretKeyRequired) {
		t.Fatalf("errors = %#v, want %s", result.Errors(), CodeCredentialSecretKeyRequired)
	}
}

func TestAppSecretValidatorRequiresSecret(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeAppSecret},
		)}),
		&AppSecretValidator{},
	)

	if !result.HasErrors() || !hasViolationCode(result, CodeCredentialSecretKeyRequired) {
		t.Fatalf("errors = %#v, want %s", result.Errors(), CodeCredentialSecretKeyRequired)
	}
}

func TestKubeconfigValidatorRequiresSecret(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeKubeconfig},
		)}),
		&KubeconfigValidator{},
	)

	if !result.HasErrors() || !hasViolationCode(result, CodeCredentialSecretKeyRequired) {
		t.Fatalf("errors = %#v, want %s", result.Errors(), CodeCredentialSecretKeyRequired)
	}
}

func TestCombinedCredentialValidatorDispatchesAllAuthSchemes(t *testing.T) {
	t.Parallel()

	validator := allSchemesValidator()

	allowed := []cpapi.AuthScheme{
		cpapi.AuthSchemeAccessKeyPair,
		cpapi.AuthSchemeUserPassword,
		cpapi.AuthSchemeAPIToken,
		cpapi.AuthSchemeServiceAccount,
		cpapi.AuthSchemeClientSecret,
		cpapi.AuthSchemeKubeconfig,
		cpapi.AuthSchemeAppSecret,
	}

	for _, scheme := range allowed {
		scheme := scheme
		t.Run(string(scheme), func(t *testing.T) {
			t.Parallel()

			result := ValidateCredentialSecretContent(
				credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
					cpapi.CredentialSecretName,
					testNamespace,
					cpapi.CredentialSecretStringData{AuthScheme: scheme},
				)}),
				validator,
			)
			if !result.HasErrors() {
				t.Fatalf("ValidateCredentialSecretContent(%s) = %q, want validation errors", scheme, result.Error())
			}
		})
	}
}

func TestCombinedCredentialValidatorRejectsUnknownAuthScheme(t *testing.T) {
	t.Parallel()

	validator := &CombinedCredentialValidator{
		ValidatorMap: map[cpapi.AuthScheme]CredentialsValidator{
			cpapi.AuthSchemeAPIToken: &APITokenValidator{},
		},
	}

	result := ValidateCredentialSecretContent(
		credentialContentState([]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			testNamespace,
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeKubeconfig,
				Secret:     validTestKubeconfigB64(),
			},
		)}),
		validator,
	)

	if !hasViolationCode(result, CodeUnsupportedAuthScheme) {
		t.Fatalf("ValidateCredentialSecretContent() = %q, want %s", result.Error(), CodeUnsupportedAuthScheme)
	}
}

func TestValidateCredentialSecretPresenceNilState(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretPresence(nil)
	if !hasViolationCode(result, cpvalapi.CodeInternalStateNil) {
		t.Fatalf("ValidateCredentialSecretPresence(nil) = %q, want %s", result.Error(), cpvalapi.CodeInternalStateNil)
	}
}

func TestValidateCredentialSecretPresenceRequiresPrimarySecret(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretPresence(credentialContentState(nil))
	if !hasViolationCode(result, CodeCredentialSecretRequired) {
		t.Fatalf("ValidateCredentialSecretPresence() = %q, want %s", result.Error(), CodeCredentialSecretRequired)
	}
}

func TestValidateCredentialSecretPresenceRejectsWrongType(t *testing.T) {
	t.Parallel()

	// A secret with the right name but wrong type is invisible to ExistsCredentialSecret
	// (ExistsCredentialSecret filters by IsManaged()), so the function reports "required" instead.
	result := ValidateCredentialSecretPresence(credentialContentState([]cpapi.CredentialSecret{
		{
			ObjectMeta: cpapi.ObjectMeta{Name: cpapi.CredentialSecretName, Namespace: testNamespace},
			Type:       "kubernetes.io/tls",
		},
	}))
	if !hasViolationCode(result, CodeCredentialSecretRequired) {
		t.Fatalf("ValidateCredentialSecretPresence() = %q, want %s", result.Error(), CodeCredentialSecretRequired)
	}
}

func TestValidateCredentialSecretPresenceAllowsManagedSecret(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretPresence(credentialContentState([]cpapi.CredentialSecret{
		managedCredentialSecret(cpapi.CredentialSecretName, testNamespace, cpapi.CredentialSecretStringData{
			AuthScheme: cpapi.AuthSchemeKubeconfig,
			Secret:     validTestKubeconfigB64(),
		}),
	}))
	if result.HasErrors() {
		t.Fatalf("ValidateCredentialSecretPresence() unexpected errors: %s", result.Error())
	}
}
