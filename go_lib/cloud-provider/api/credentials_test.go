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
)

func TestCredentialSecretIsManaged(t *testing.T) {
	t.Parallel()

	if !(CredentialSecret{
		ObjectMeta: ObjectMeta{Name: CredentialSecretName},
		Type:       CredentialsSecretType,
	}).IsManaged() {
		t.Fatal("IsManaged() = false for primary credential secret")
	}
	if !(CredentialSecret{
		ObjectMeta: ObjectMeta{Name: "other"},
		Type:       CredentialsSecretType,
	}).IsManaged() {
		t.Fatal("IsManaged() = false for credential secret with arbitrary name")
	}
	if !(CredentialSecret{
		ObjectMeta: ObjectMeta{Name: CredentialSecretName + "-storage"},
		Type:       CredentialsSecretType,
	}).IsManaged() {
		t.Fatal("IsManaged() = false for component credential secret")
	}
	if (CredentialSecret{
		ObjectMeta: ObjectMeta{Name: CredentialSecretName + "-storage"},
		Type:       "kubernetes.io/tls",
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
