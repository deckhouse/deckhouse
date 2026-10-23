/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	PasswordGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "Password",
	}
	OfflineSessionsGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "OfflineSessions",
	}
	RefreshTokenGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "RefreshToken",
	}
	// UserIdentityGVK is the per-pair (connector, user) record of Dex v2.46: claims, second
	// factors, lock. Its name is the OfflineSessions name of the same pair.
	UserIdentityGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "UserIdentity",
	}
	// AuthSessionGVK is a Dex v2.46 browser session.
	AuthSessionGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "AuthSession",
	}
	// AuthCodeGVK is a Dex authorization code: a sign-in a client has not exchanged for tokens
	// yet. The controller deletes the codes of a revoked pair and never caches them: the object
	// name is the code.
	AuthCodeGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "AuthCode",
	}
	// AuthRequestGVK is a Dex auth request: a sign-in in progress. The controller deletes the
	// requests of a revoked pair and never caches them.
	AuthRequestGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "AuthRequest",
	}
	// RevocationGVK is the record of a revocation: the generation of a pair that was revoked, an
	// ended browser session, or a revoked grant. Dex and the controller write it; the revocation
	// feed, the user-authz webhook, and basic-auth-proxy read it.
	RevocationGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "Revocation",
	}
	// OAuth2ClientGVK is a Dex client. The revocation feed authenticates the clients of
	// DexAuthenticators with it; user-authn-controller writes the kubernetes client, and
	// dexclient-controller the clients of DexClients.
	OAuth2ClientGVK = schema.GroupVersionKind{
		Group:   "dex.coreos.com",
		Version: "v1",
		Kind:    "OAuth2Client",
	}
	UserGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1",
		Kind:    "User",
	}
	GroupGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    "Group",
	}
	DexProviderGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1",
		Kind:    "DexProvider",
	}
	DexClientGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1",
		Kind:    "DexClient",
	}
	DexAuthenticatorGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1",
		Kind:    "DexAuthenticator",
	}
	UserOperationGVK = schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1",
		Kind:    "UserOperation",
	}
	NamespaceGVK = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Namespace",
	}
	SecretGVK = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Secret",
	}
)
