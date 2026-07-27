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

package admission

import (
	corev1 "k8s.io/api/core/v1"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

// SecretToCredentialSecret converts a Kubernetes Secret into a typed CredentialSecret.
func SecretToCredentialSecret(secret *corev1.Secret) cpapi.CredentialSecret {
	if secret == nil {
		return cpapi.CredentialSecret{}
	}

	return cpapi.CredentialSecret{
		TypeMeta: cpapi.TypeMeta{
			APIVersion: secret.APIVersion,
			Kind:       secret.Kind,
		},
		ObjectMeta: cpapi.ObjectMeta{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
		Type: string(secret.Type),
		Data: cpapi.CredentialSecretData{
			AuthScheme: secret.Data[cpapi.CredentialSecretAuthSchemeKey],
			Identity:   secret.Data[cpapi.CredentialSecretIdentityKey],
			Secret:     secret.Data[cpapi.CredentialSecretSecretKey],
		},
		StringData: cpapi.CredentialSecretStringData{
			AuthScheme: cpapi.AuthScheme(secret.StringData[cpapi.CredentialSecretAuthSchemeKey]),
			Identity:   secret.StringData[cpapi.CredentialSecretIdentityKey],
			Secret:     secret.StringData[cpapi.CredentialSecretSecretKey],
		},
	}
}

// IsManagedCredentialSecret reports whether secret is a managed provider credential Secret.
func IsManagedCredentialSecret(secret *corev1.Secret) bool {
	if secret == nil {
		return false
	}

	return secret.Type == cpapi.CredentialsSecretType
}
