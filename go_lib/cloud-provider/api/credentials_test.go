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

package api

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecretToCredentialSecretNil(t *testing.T) {
	t.Parallel()

	if got := SecretToCredentialSecret(nil); got.Name != "" || got.Type != "" {
		t.Fatalf("SecretToCredentialSecret(nil) = %#v, want empty CredentialSecret", got)
	}
}

func TestSecretToCredentialSecretMapsTypedFields(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d8-credentials",
			Namespace: "d8-cloud-provider-dvp",
		},
		Type: CredentialsSecretType,
		Data: map[string][]byte{
			CredentialSecretAuthSchemeKey: []byte("kubeconfig"),
			CredentialSecretSecretKey:     []byte("encoded-kubeconfig"),
		},
		StringData: map[string]string{
			CredentialSecretIdentityKey: "ignored-when-data-present",
		},
	}

	got := SecretToCredentialSecret(secret)

	if got.Name != secret.Name || got.Namespace != secret.Namespace {
		t.Fatalf("SecretToCredentialSecret() metadata = %#v, want name %q namespace %q", got.ObjectMeta, secret.Name, secret.Namespace)
	}
	if got.Type != string(secret.Type) {
		t.Fatalf("SecretToCredentialSecret() type = %q, want %q", got.Type, secret.Type)
	}
	if string(got.Data.AuthScheme) != "kubeconfig" {
		t.Fatalf("SecretToCredentialSecret() data.authScheme = %q, want %q", got.Data.AuthScheme, "kubeconfig")
	}
	if string(got.Data.Secret) != "encoded-kubeconfig" {
		t.Fatalf("SecretToCredentialSecret() data.secret = %q, want %q", got.Data.Secret, "encoded-kubeconfig")
	}
	if got.StringData.Identity != "ignored-when-data-present" {
		t.Fatalf("SecretToCredentialSecret() stringData.identity = %q, want %q", got.StringData.Identity, "ignored-when-data-present")
	}
}

func TestIsManagedCredentialSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		secret *corev1.Secret
		want   bool
	}{
		{name: "nil secret", secret: nil, want: false},
		{name: "wrong type", secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: CredentialSecretName}, Type: corev1.SecretTypeTLS}, want: false},
		{name: "wrong name", secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Type: CredentialsSecretType}, want: false},
		{name: "primary", secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: CredentialSecretName}, Type: CredentialsSecretType}, want: true},
		{name: "component", secret: &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "d8-credentials-storage"}, Type: CredentialsSecretType}, want: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsManagedCredentialSecret(tt.secret); got != tt.want {
				t.Fatalf("IsManagedCredentialSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCredentialSecretIsManaged(t *testing.T) {
	t.Parallel()

	if !(CredentialSecret{
		ObjectMeta: metav1.ObjectMeta{Name: CredentialSecretName},
		Type:       CredentialsSecretType,
	}).IsManaged() {
		t.Fatal("IsManaged() = false for primary credential secret")
	}
	if (CredentialSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "other"},
		Type:       CredentialsSecretType,
	}).IsManaged() {
		t.Fatal("IsManaged() = true for unmanaged name")
	}
	if !(CredentialSecret{
		ObjectMeta: metav1.ObjectMeta{Name: CredentialSecretName + "-storage"},
		Type:       CredentialsSecretType,
	}).IsManaged() {
		t.Fatal("IsManaged() = false for component credential secret")
	}
	if (CredentialSecret{
		ObjectMeta: metav1.ObjectMeta{Name: CredentialSecretName + "-storage"},
		Type:       string(corev1.SecretTypeTLS),
	}).IsManaged() {
		t.Fatal("IsManaged() = true for component secret with wrong type")
	}
}

func TestCredentialSecretNormalizedDataFromDataOnly(t *testing.T) {
	t.Parallel()

	secret := CredentialSecret{
		Data: CredentialSecretData{
			AuthScheme: []byte("apiToken"),
			Identity:   []byte("id"),
			Secret:     []byte("secret"),
		},
	}

	got := secret.NormalizedData()
	if got[CredentialSecretAuthSchemeKey] != "apiToken" || got[CredentialSecretIdentityKey] != "id" || got[CredentialSecretSecretKey] != "secret" {
		t.Fatalf("NormalizedData() = %#v", got)
	}
}

func TestCredentialSecretNormalizedDataPrefersStringData(t *testing.T) {
	t.Parallel()

	secret := CredentialSecret{
		Data: CredentialSecretData{
			AuthScheme: []byte("apiToken"),
			Secret:     []byte("from-data"),
		},
		StringData: CredentialSecretStringData{
			AuthScheme: AuthSchemeKubeconfig,
			Secret:     "from-string-data",
		},
	}

	got := secret.NormalizedData()

	if got[CredentialSecretAuthSchemeKey] != string(AuthSchemeKubeconfig) {
		t.Fatalf("NormalizedData() authScheme = %q, want %q", got[CredentialSecretAuthSchemeKey], AuthSchemeKubeconfig)
	}
	if got[CredentialSecretSecretKey] != "from-string-data" {
		t.Fatalf("NormalizedData() secret = %q, want %q", got[CredentialSecretSecretKey], "from-string-data")
	}
}
