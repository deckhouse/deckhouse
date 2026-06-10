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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateCredentialSecretsAllowsConfiguredAuthScheme(t *testing.T) {
	t.Parallel()

	secret := cpapi.CredentialSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-credentials",
			Namespace: "d8-cloud-provider-vcd",
		},
		Type: cpapi.CredentialsSecretType,
		StringData: cpapi.CredentialSecretStringData{
			AuthScheme: cpapi.AuthSchemeAPIToken,
			Secret:     "token-123",
		},
	}

	result := ValidateCredentialSecrets(
		[]cpapi.CredentialSecret{secret},
		[]cpapi.AuthScheme{cpapi.AuthSchemeAPIToken},
	)

	if result.HasErrors() {
		t.Fatalf("ValidateCredentialSecrets() unexpected errors: %s", result.Error())
	}
}

func TestValidateCredentialSecretsRejectsUnsupportedAuthScheme(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecrets(
		[]cpapi.CredentialSecret{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "d8-credentials"},
				StringData: cpapi.CredentialSecretStringData{
					AuthScheme: cpapi.AuthSchemeAPIToken,
					Secret:     "token-123",
				},
			},
		},
		[]cpapi.AuthScheme{cpapi.AuthSchemeKubeconfig},
	)

	if !result.HasErrors() || !strings.Contains(result.Error(), "is not allowed") {
		t.Fatalf("ValidateCredentialSecrets() expected unsupported auth scheme error, got: %s", result.Error())
	}
}

func TestValidateAuthSchemeKubeconfigKeys(t *testing.T) {
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

	result := Result{}
	ValidateAuthSchemeKubeconfigKeys("Secret/d8-credentials", map[string]string{
		"authScheme": string(cpapi.AuthSchemeKubeconfig),
		"secret":     kubeconfigB64,
	}, &result)

	if result.HasErrors() {
		t.Fatalf("ValidateAuthSchemeKubeconfigKeys() unexpected errors: %s", result.Error())
	}
}

func TestValidateAuthSchemeKubeconfigKeysInvalidSecret(t *testing.T) {
	t.Parallel()

	result := Result{}
	ValidateAuthSchemeKubeconfigKeys("Secret/d8-credentials", map[string]string{
		"authScheme": string(cpapi.AuthSchemeKubeconfig),
		"secret":     "not-base64",
	}, &result)

	if !result.HasErrors() || !strings.Contains(result.Error(), "base64-encoded kubeconfig") {
		t.Fatalf("ValidateAuthSchemeKubeconfigKeys() expected invalid kubeconfig error, got: %s", result.Error())
	}
}

func TestValidateAuthSchemeServiceAccountKeys(t *testing.T) {
	t.Parallel()

	result := Result{}
	ValidateAuthSchemeServiceAccountKeys("Secret/d8-credentials", map[string]string{}, &result)
	if !result.HasErrors() || !strings.Contains(result.Error(), "secret is required") {
		t.Fatalf("ValidateAuthSchemeServiceAccountKeys() expected secret required error, got: %s", result.Error())
	}
}

func TestValidateCredentialSecretsRequiresAuthScheme(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecrets(
		[]cpapi.CredentialSecret{{ObjectMeta: metav1.ObjectMeta{Name: "d8-credentials"}}},
		[]cpapi.AuthScheme{cpapi.AuthSchemeAPIToken},
	)

	if !result.HasErrors() || !strings.Contains(result.Error(), "authScheme is required") {
		t.Fatalf("ValidateCredentialSecrets() = %q, want authScheme required", result.Error())
	}
}

func TestValidateCredentialSecretsEmptyNameUsesKindPath(t *testing.T) {
	t.Parallel()

	result := ValidateCredentialSecrets(
		[]cpapi.CredentialSecret{{StringData: cpapi.CredentialSecretStringData{Secret: "x"}}},
		[]cpapi.AuthScheme{cpapi.AuthSchemeAPIToken},
	)

	if !strings.Contains(result.Error(), "Secret.data.authScheme") {
		t.Fatalf("ValidateCredentialSecrets() = %q, want Secret path without name", result.Error())
	}
}

func TestValidateAuthSchemeAccessKeyPairKeys(t *testing.T) {
	t.Parallel()

	result := Result{}
	ValidateAuthSchemeAccessKeyPairKeys("Secret/x", map[string]string{}, &result)
	if !result.HasErrors() || !strings.Contains(result.Error(), "identity is required") || !strings.Contains(result.Error(), "secret is required") {
		t.Fatalf("ValidateAuthSchemeAccessKeyPairKeys() = %q", result.Error())
	}
}

func TestValidateAuthSchemeUserPasswordKeys(t *testing.T) {
	t.Parallel()

	result := Result{}
	ValidateAuthSchemeUserPasswordKeys("Secret/x", map[string]string{"identity": "user"}, &result)
	if !result.HasErrors() || !strings.Contains(result.Error(), "secret is required") {
		t.Fatalf("ValidateAuthSchemeUserPasswordKeys() = %q", result.Error())
	}
}

func TestValidateAuthSchemeAPITokenKeys(t *testing.T) {
	t.Parallel()

	result := Result{}
	ValidateAuthSchemeAPITokenKeys("Secret/x", map[string]string{}, &result)
	if !result.HasErrors() || !strings.Contains(result.Error(), "secret is required") {
		t.Fatalf("ValidateAuthSchemeAPITokenKeys() = %q", result.Error())
	}
}

func TestValidateAuthSchemeClientSecretKeys(t *testing.T) {
	t.Parallel()

	result := Result{}
	ValidateAuthSchemeClientSecretKeys("Secret/x", map[string]string{}, &result)
	if len(result.Errors) != 2 {
		t.Fatalf("ValidateAuthSchemeClientSecretKeys() errors = %d, want 2", len(result.Errors))
	}
}

func TestValidateAuthSchemeAppSecretKeys(t *testing.T) {
	t.Parallel()

	result := Result{}
	ValidateAuthSchemeAppSecretKeys("Secret/x", map[string]string{}, &result)
	if len(result.Errors) != 2 {
		t.Fatalf("ValidateAuthSchemeAppSecretKeys() errors = %d, want 2", len(result.Errors))
	}
}

func TestValidateAuthSchemeKubeconfigKeysSkipsValidationForEmptySecret(t *testing.T) {
	t.Parallel()

	result := Result{}
	ValidateAuthSchemeKubeconfigKeys("Secret/x", map[string]string{}, &result)
	if !result.HasErrors() || len(result.Errors) != 1 {
		t.Fatalf("ValidateAuthSchemeKubeconfigKeys() = %q, want only required secret error", result.Error())
	}
}

func TestValidateAuthSchemeKubeconfigKeysInvalidYAML(t *testing.T) {
	t.Parallel()

	invalid := base64.StdEncoding.EncodeToString([]byte("not-a-kubeconfig"))
	result := Result{}
	ValidateAuthSchemeKubeconfigKeys("Secret/x", map[string]string{
		"authScheme": string(cpapi.AuthSchemeKubeconfig),
		"secret":     invalid,
	}, &result)

	if !result.HasErrors() || !strings.Contains(result.Error(), "base64-encoded kubeconfig") {
		t.Fatalf("ValidateAuthSchemeKubeconfigKeys() = %q", result.Error())
	}
}

func TestValidateAuthSchemeServiceAccountKeysSuccess(t *testing.T) {
	t.Parallel()

	result := Result{}
	ValidateAuthSchemeServiceAccountKeys("Secret/x", map[string]string{"secret": "token"}, &result)
	if result.HasErrors() {
		t.Fatalf("ValidateAuthSchemeServiceAccountKeys() unexpected errors: %s", result.Error())
	}
}

func TestValidateCredentialSecretsDispatchesAllAuthSchemeValidators(t *testing.T) {
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

			result := ValidateCredentialSecrets(
				[]cpapi.CredentialSecret{{
					ObjectMeta: metav1.ObjectMeta{Name: cpapi.CredentialSecretName},
					StringData: cpapi.CredentialSecretStringData{AuthScheme: scheme},
				}},
				allowed,
			)
			if !result.HasErrors() {
				t.Fatalf("ValidateCredentialSecrets(%s) = %q, want validation errors", scheme, result.Error())
			}
		})
	}
}
