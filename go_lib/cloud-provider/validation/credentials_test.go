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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func validTestKubeconfigB64() string {
	return "YXBpVmVyc2lvbjogdjEKa2luZDogQ29uZmlnCmNsdXN0ZXJzOgotIG5hbWU6IHRlc3QKICBjbHVzdGVyOgogICAgc2VydmVyOiBodHRwczovLzEyNy4wLjAuMTo2NDQzCiAgICBpbnNlY3VyZS1za2lwLXRscy12ZXJpZnk6IHRydWUKY29udGV4dHM6Ci0gbmFtZTogdGVzdAogIGNvbnRleHQ6CiAgICBjbHVzdGVyOiB0ZXN0CiAgICB1c2VyOiB0ZXN0CmN1cnJlbnQtY29udGV4dDogdGVzdAp1c2VyczoKLSBuYW1lOiB0ZXN0CiAgdXNlcjoKICAgIHRva2VuOiB0ZXN0LXRva2Vu"
}

func managedCredentialSecret(name, namespace string, data cpapi.CredentialSecretStringData) cpapi.CredentialSecret {
	return cpapi.CredentialSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type:       cpapi.CredentialsSecretType,
		StringData: data,
	}
}

func TestValidateCredentialSecretContentAllowsConfiguredAuthScheme(t *testing.T) {
	t.Parallel()

	secret := managedCredentialSecret(
		cpapi.CredentialSecretName,
		"d8-cloud-provider-vcd",
		cpapi.CredentialSecretStringData{
			AuthScheme: cpapi.AuthSchemeAPIToken,
			Secret:     "token-123",
		},
	)

	result := ValidateCredentialSecretContent(credentialContentState(
		[]cpapi.CredentialSecret{secret},
		[]cpapi.AuthScheme{cpapi.AuthSchemeAPIToken},
	))

	if result.HasErrors() {
		t.Fatalf("ValidateCredentialSecretContent() unexpected errors: %s", result.Error())
	}
}

func TestValidateCredentialSecretContentRejectsUnsupportedAuthScheme(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(credentialContentState(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeAPIToken,
				Secret:     "token-123",
			},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeKubeconfig},
	))

	if !result.HasErrors() || !strings.Contains(result.Error(), "is not allowed") {
		t.Fatalf("ValidateCredentialSecretContent() expected unsupported auth scheme error, got: %s", result.Error())
	}
}

func TestValidateCredentialSecretsAcceptsValidKubeconfig(t *testing.T) {
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

	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeKubeconfig,
				Secret:     kubeconfigB64,
			},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeKubeconfig},
	)

	if result.HasErrors() {
		t.Fatalf("validateCredentialSecrets() unexpected errors: %s", result.Error())
	}
}

func TestValidateCredentialSecretsRejectsInvalidKubeconfig(t *testing.T) {
	t.Parallel()

	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeKubeconfig,
				Secret:     "not-base64",
			},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeKubeconfig},
	)

	if !result.HasErrors() || !strings.Contains(result.Error(), "base64-encoded kubeconfig") {
		t.Fatalf("validateCredentialSecrets() expected invalid kubeconfig error, got: %s", result.Error())
	}
}

func TestValidateCredentialSecretsRequiresServiceAccountSecret(t *testing.T) {
	t.Parallel()

	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeServiceAccount},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeServiceAccount},
	)

	if !result.HasErrors() || !hasViolationCode(result, "credential_secret_key_required") {
		t.Fatalf("validateCredentialSecrets() errors = %#v, want credential_secret_key_required", result.Errors())
	}
}

func TestValidateCredentialSecretContentRequiresAuthScheme(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecretContent(credentialContentState(
		[]cpapi.CredentialSecret{managedCredentialSecret(cpapi.CredentialSecretName, "", cpapi.CredentialSecretStringData{})},
		[]cpapi.AuthScheme{cpapi.AuthSchemeAPIToken},
	))

	if !result.HasErrors() || !strings.Contains(result.Error(), "authScheme is required") {
		t.Fatalf("ValidateCredentialSecretContent() = %q, want authScheme required", result.Error())
	}
}

func TestValidateCredentialSecretContentIgnoresOrdinaryModuleSecrets(t *testing.T) {
	t.Parallel()

	state := credentialContentState(
		[]cpapi.CredentialSecret{
			managedCredentialSecret(
				cpapi.CredentialSecretName,
				"d8-cloud-provider-test",
				cpapi.CredentialSecretStringData{
					AuthScheme: cpapi.AuthSchemeKubeconfig,
					Secret:     validTestKubeconfigB64(),
				},
			),
			{
				ObjectMeta: metav1.ObjectMeta{Name: "validation-webhook-tls", Namespace: "d8-cloud-provider-test"},
				Type:       string(corev1.SecretTypeTLS),
			},
		},
		[]cpapi.AuthScheme{cpapi.AuthSchemeKubeconfig},
	)

	if result := ValidateCredentialSecretContent(state); result.HasErrors() {
		t.Fatalf("ValidateCredentialSecretContent() unexpected errors: %s", result.Error())
	}
}

func TestValidateCredentialSecretContentIgnoresOtherNamespace(t *testing.T) {
	t.Parallel()

	state := credentialContentState(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"other",
			cpapi.CredentialSecretStringData{AuthScheme: "invalid"},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeKubeconfig},
	)

	if result := ValidateCredentialSecretContent(state); result.HasErrors() {
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
			wantCode: "credential_identity_required",
		},
		{
			name:     "secret",
			key:      "secret",
			wantCode: "credential_secret_key_required",
		},
		{
			name:     "custom field",
			key:      "customKey",
			wantCode: "credential_field_required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := Result{}
			validateRequiredCredentialKey(
				"Secret/"+cpapi.CredentialSecretName,
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

func TestValidateCredentialSecretsRequiresAccessKeyPairFields(t *testing.T) {
	t.Parallel()

	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeAccessKeyPair},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeAccessKeyPair},
	)

	if !result.HasErrors() {
		t.Fatalf("validateCredentialSecrets() = %q, want errors", result.Error())
	}
	if !hasViolationCode(result, "credential_identity_required") {
		t.Fatalf("validateCredentialSecrets() errors = %#v, want credential_identity_required", result.Errors())
	}
	if !hasViolationCode(result, "credential_secret_key_required") {
		t.Fatalf("validateCredentialSecrets() errors = %#v, want credential_secret_key_required", result.Errors())
	}
}

func TestValidateCredentialSecretsRequiresUserPasswordSecret(t *testing.T) {
	t.Parallel()

	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeUserPassword,
				Identity:   "user",
			},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeUserPassword},
	)

	if !result.HasErrors() || !hasViolationCode(result, "credential_secret_key_required") {
		t.Fatalf("validateCredentialSecrets() errors = %#v, want credential_secret_key_required", result.Errors())
	}
}

func TestValidateCredentialSecretsRequiresAPITokenSecret(t *testing.T) {
	t.Parallel()

	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeAPIToken},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeAPIToken},
	)

	if !result.HasErrors() || !hasViolationCode(result, "credential_secret_key_required") {
		t.Fatalf("validateCredentialSecrets() errors = %#v, want credential_secret_key_required", result.Errors())
	}
}

func TestValidateCredentialSecretsRequiresClientSecretFields(t *testing.T) {
	t.Parallel()

	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeClientSecret},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeClientSecret},
	)

	if !result.HasErrors() {
		t.Fatalf("validateCredentialSecrets() = %q, want errors", result.Error())
	}
	if !hasViolationCode(result, "credential_identity_required") {
		t.Fatalf("validateCredentialSecrets() errors = %#v, want credential_identity_required", result.Errors())
	}
	if !hasViolationCode(result, "credential_secret_key_required") {
		t.Fatalf("validateCredentialSecrets() errors = %#v, want credential_secret_key_required", result.Errors())
	}
}

func TestValidateCredentialSecretsRequiresAppSecretFields(t *testing.T) {
	t.Parallel()

	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeAppSecret},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeAppSecret},
	)

	if !result.HasErrors() {
		t.Fatalf("validateCredentialSecrets() = %q, want errors", result.Error())
	}
	if !hasViolationCode(result, "credential_identity_required") {
		t.Fatalf("validateCredentialSecrets() errors = %#v, want credential_identity_required", result.Errors())
	}
	if !hasViolationCode(result, "credential_secret_key_required") {
		t.Fatalf("validateCredentialSecrets() errors = %#v, want credential_secret_key_required", result.Errors())
	}
}

func TestValidateCredentialSecretsRequiresKubeconfigSecret(t *testing.T) {
	t.Parallel()

	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{AuthScheme: cpapi.AuthSchemeKubeconfig},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeKubeconfig},
	)

	if !result.HasErrors() || len(result.Errors()) != 1 {
		t.Fatalf("validateCredentialSecrets() = %q, want only required secret error", result.Error())
	}
	if !hasViolationCode(result, "credential_secret_key_required") {
		t.Fatalf("validateCredentialSecrets() errors = %#v, want credential_secret_key_required", result.Errors())
	}
}

func TestValidateCredentialSecretsRejectsInvalidKubeconfigYAML(t *testing.T) {
	t.Parallel()

	invalid := base64.StdEncoding.EncodeToString([]byte("not-a-kubeconfig"))
	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeKubeconfig,
				Secret:     invalid,
			},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeKubeconfig},
	)

	if !result.HasErrors() || !strings.Contains(result.Error(), "base64-encoded kubeconfig") {
		t.Fatalf("validateCredentialSecrets() = %q", result.Error())
	}
}

func TestValidateCredentialSecretsAcceptsServiceAccountSecret(t *testing.T) {
	t.Parallel()

	result := validateCredentialSecrets(
		[]cpapi.CredentialSecret{managedCredentialSecret(
			cpapi.CredentialSecretName,
			"",
			cpapi.CredentialSecretStringData{
				AuthScheme: cpapi.AuthSchemeServiceAccount,
				Secret:     "token",
			},
		)},
		[]cpapi.AuthScheme{cpapi.AuthSchemeServiceAccount},
	)

	if result.HasErrors() {
		t.Fatalf("validateCredentialSecrets() unexpected errors: %s", result.Error())
	}
}

func TestValidateCredentialSecretContentDispatchesAllAuthSchemeValidators(t *testing.T) {
	t.Parallel()

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

			result := ValidateCredentialSecretContent(credentialContentState(
				[]cpapi.CredentialSecret{managedCredentialSecret(
					cpapi.CredentialSecretName,
					"",
					cpapi.CredentialSecretStringData{AuthScheme: scheme},
				)},
				allowed,
			))
			if !result.HasErrors() {
				t.Fatalf("ValidateCredentialSecretContent(%s) = %q, want validation errors", scheme, result.Error())
			}
		})
	}
}
