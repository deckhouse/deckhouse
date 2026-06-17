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

// Package api defines typed Kubernetes resource models shared by cloud-provider modules.
package api

const (
	// CredentialsSecretType is the Kubernetes Secret type that marks provider credentials.
	CredentialsSecretType = "cloud-provider.deckhouse.io/credentials"
	// CredentialSecretName is the default name of the primary provider credential Secret.
	CredentialSecretName = "d8-credentials"
	// CredentialSecretAuthSchemeKey is the Secret data key for the authentication scheme.
	CredentialSecretAuthSchemeKey = "authScheme"
	// CredentialSecretIdentityKey is the Secret data key for the public identity value.
	CredentialSecretIdentityKey = "identity"
	// CredentialSecretSecretKey is the Secret data key for the secret credential value.
	CredentialSecretSecretKey = "secret"
)

// AuthScheme identifies how provider credentials are stored in a credential Secret.
type AuthScheme string

const (
	// AuthSchemeAccessKeyPair stores access key and secret key pair credentials.
	AuthSchemeAccessKeyPair AuthScheme = "accessKeyPair"
	// AuthSchemeUserPassword stores username and password credentials.
	AuthSchemeUserPassword AuthScheme = "userPassword"
	// AuthSchemeAPIToken stores a single API token.
	AuthSchemeAPIToken AuthScheme = "apiToken"
	// AuthSchemeServiceAccount stores a service account key or token.
	AuthSchemeServiceAccount AuthScheme = "serviceAccount"
	// AuthSchemeClientSecret stores OAuth client ID and client secret.
	AuthSchemeClientSecret AuthScheme = "clientSecret"
	// AuthSchemeKubeconfig stores a base64-encoded kubeconfig.
	AuthSchemeKubeconfig AuthScheme = "kubeconfig"
	// AuthSchemeAppSecret stores application-specific identity and secret values.
	AuthSchemeAppSecret AuthScheme = "appSecret"
)

// CredentialSecret is a typed view of a provider credential Secret.
type CredentialSecret struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	// Type is the Kubernetes Secret type.
	Type string `json:"type,omitempty"`
	// Data holds typed binary Secret data fields.
	Data CredentialSecretData `json:"data,omitempty"`
	// StringData holds typed string Secret data fields.
	StringData CredentialSecretStringData `json:"stringData,omitempty"`
}

// CredentialSecretData holds typed binary Secret data fields.
type CredentialSecretData struct {
	AuthScheme []byte `json:"authScheme,omitempty"`
	Identity   []byte `json:"identity,omitempty"`
	Secret     []byte `json:"secret,omitempty"`
}

// CredentialSecretStringData holds typed string Secret data fields.
type CredentialSecretStringData struct {
	AuthScheme AuthScheme `json:"authScheme,omitempty"`
	Identity   string     `json:"identity,omitempty"`
	Secret     string     `json:"secret,omitempty"`
}

// IsManaged reports whether the credential Secret uses the managed provider type.
func (s CredentialSecret) IsManaged() bool {
	return s.Type == CredentialsSecretType
}

// NormalizedData returns credential fields as strings; StringData overrides Data.
func (s CredentialSecret) NormalizedData() map[string]string {
	result := make(map[string]string, 3)
	if len(s.Data.AuthScheme) > 0 {
		result[CredentialSecretAuthSchemeKey] = string(s.Data.AuthScheme)
	}
	if len(s.Data.Identity) > 0 {
		result[CredentialSecretIdentityKey] = string(s.Data.Identity)
	}
	if len(s.Data.Secret) > 0 {
		result[CredentialSecretSecretKey] = string(s.Data.Secret)
	}
	if s.StringData.AuthScheme != "" {
		result[CredentialSecretAuthSchemeKey] = string(s.StringData.AuthScheme)
	}
	if s.StringData.Identity != "" {
		result[CredentialSecretIdentityKey] = s.StringData.Identity
	}
	if s.StringData.Secret != "" {
		result[CredentialSecretSecretKey] = s.StringData.Secret
	}

	return result
}
