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

import (
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/naming"
)

// secretFields are the top-level keys of Dex storage objects that hold credentials:
// Password hash and previousHashes, connectorData of OfflineSessions and RefreshToken, the TOTP
// secret of OfflineSessions, refresh token values, and the cookie secret and nonce of AuthSession.
// The TOTP secrets of UserIdentity (mfaSecrets) are reduced to mfaSecretFlags instead, and its
// WebAuthn credentials (webauthnCredentials) to as many empty entries as there are credentials,
// since UserAccount shows whether the user has a key.
var secretFields = []string{
	"hash",
	"previousHashes",
	"connectorData",
	"totp",
	"token",
	"obsoleteToken",
	"secret",
	"nonce",
}

// mfaSecretFlags are the keys of an entry of UserIdentity.mfaSecrets that the cache keeps: which
// factor it is and whether the user confirmed it, for UserAccount. The secret and the counter of
// the last accepted code are dropped.
var mfaSecretFlags = []string{
	"authenticatorID",
	"type",
	"confirmed",
	"createdAt",
}

// refFields are the keys of a reference of OfflineSessions (an entry of refresh or grants) that the
// cache keeps: the grant, its client and browser session, and the device UserAccount lists it as.
var refFields = []string{
	"ID",
	"ClientID",
	"SessionID",
	"CreatedAt",
	"LastUsed",
	"IPAddress",
	"UserAgent",
	"DeviceHash",
}

// SecretFields returns Dex credential keys stripped from the informer cache
// and UserAccount. The references of OfflineSessions.refresh and grants are reduced to refFields,
// UserIdentity.mfaSecrets to mfaSecretFlags, and UserIdentity.webauthnCredentials to a count.
func SecretFields() []string {
	return slices.Clone(secretFields)
}

// refreshTokenFields are the top-level keys of a RefreshToken the cache keeps besides apiVersion,
// kind and the metadata: everything the controller reads from a cached token. The pair index reads
// claims.userID and connectorID; the revocations read clientID, connectorID, claims.userID,
// CreatedAt and generation; the migration of the DexAuthenticator tokens reads clientID; the
// UserIdentity migration reads connectorID, claims and LastUsed. A field read from a cached token
// that is missing here reads as empty.
var refreshTokenFields = []string{
	"clientID",
	"connectorID",
	"claims",
	"CreatedAt",
	"LastUsed",
	"generation",
}

// InformerCacheByObject scopes Password, OfflineSessions, RefreshToken, UserIdentity, and
// AuthSession informers to d8-user-authn and strips Dex secrets and managed fields before the
// objects are stored; a RefreshToken keeps only refreshTokenFields. Code that needs a secret field
// (the UserIdentity migration reads the TOTP secret) reads through the API reader instead.
// Revocation objects hold no secrets; they lose only their managed fields.
func InformerCacheByObject() map[client.Object]cache.ByObject {
	return map[client.Object]cache.ByObject{
		Object(PasswordGVK): {
			Namespaces: map[string]cache.Config{
				naming.DexNamespace: {},
			},
			Transform: StripDexSecrets,
		},
		Object(OfflineSessionsGVK): {
			Namespaces: map[string]cache.Config{
				naming.DexNamespace: {},
			},
			Transform: StripDexSecrets,
		},
		Object(RefreshTokenGVK): {
			Namespaces: map[string]cache.Config{
				naming.DexNamespace: {},
			},
			Transform: ReduceRefreshToken,
		},
		Object(UserIdentityGVK): {
			Namespaces: map[string]cache.Config{
				naming.DexNamespace: {},
			},
			Transform: StripDexSecrets,
		},
		Object(AuthSessionGVK): {
			Namespaces: map[string]cache.Config{
				naming.DexNamespace: {},
			},
			Transform: StripDexSecrets,
		},
		Object(RevocationGVK): {
			Namespaces: map[string]cache.Config{
				naming.DexNamespace: {},
			},
			Transform: cache.TransformStripManagedFields(),
		},
	}
}

// StripDexSecrets clones an unstructured object and deletes Dex secret fields and the managed
// fields, which are about a third of a Dex object and which nothing reads from the cache. The
// managed fields go as nil, not as an empty list: an object written back with an empty list would
// reset them on the API server, one written back without them keeps them. The input object is
// never mutated.
func StripDexSecrets(obj any) (any, error) {
	switch typed := obj.(type) {
	case *unstructured.Unstructured:
		if typed == nil {
			return obj, nil
		}
		cloned := typed.DeepCopy()
		cloned.SetManagedFields(nil)
		stripSecretKeys(cloned.Object)
		return cloned, nil
	case unstructured.Unstructured:
		cloned := typed.DeepCopy()
		cloned.SetManagedFields(nil)
		stripSecretKeys(cloned.Object)
		return *cloned, nil
	case toolscache.DeletedFinalStateUnknown:
		inner, err := StripDexSecrets(typed.Obj)
		if err != nil {
			return nil, err
		}
		typed.Obj = inner
		return typed, nil
	default:
		return obj, nil
	}
}

// ReduceRefreshToken builds the cached form of a RefreshToken: apiVersion, kind, the metadata
// without managed fields and refreshTokenFields. The token values, the connector data, the nonce,
// the scopes and the device of the token are dropped. Every kept value is copied, so the input object
// is never mutated and the result shares nothing with it.
func ReduceRefreshToken(obj any) (any, error) {
	switch typed := obj.(type) {
	case *unstructured.Unstructured:
		if typed == nil {
			return obj, nil
		}
		return reduceRefreshToken(typed), nil
	case unstructured.Unstructured:
		return *reduceRefreshToken(&typed), nil
	case toolscache.DeletedFinalStateUnknown:
		inner, err := ReduceRefreshToken(typed.Obj)
		if err != nil {
			return nil, err
		}
		typed.Obj = inner
		return typed, nil
	default:
		return obj, nil
	}
}

func reduceRefreshToken(u *unstructured.Unstructured) *unstructured.Unstructured {
	out := make(map[string]any, len(refreshTokenFields)+3)
	for _, key := range append([]string{"apiVersion", "kind", "metadata"}, refreshTokenFields...) {
		if value, ok := u.Object[key]; ok {
			out[key] = runtime.DeepCopyJSONValue(value)
		}
	}
	reduced := &unstructured.Unstructured{Object: out}
	reduced.SetManagedFields(nil)
	return reduced
}

func stripSecretKeys(m map[string]any) {
	if m == nil {
		return
	}
	for _, key := range secretFields {
		delete(m, key)
	}
	reduceRefs(m, "refresh")
	reduceRefs(m, "grants")
	reduceMFASecretsToFlags(m)
	reduceWebAuthnToCount(m)
}

// reduceWebAuthnToCount keeps UserIdentity.webauthnCredentials as a map of the authenticator IDs to lists of empty
// entries, one per credential: the keys, the public keys and the counters are dropped, and the count stays.
// Empty or malformed maps are removed.
func reduceWebAuthnToCount(m map[string]any) {
	raw, ok := m["webauthnCredentials"]
	if !ok {
		return
	}
	byAuthenticator, ok := raw.(map[string]any)
	if !ok {
		delete(m, "webauthnCredentials")
		return
	}
	reduced := make(map[string]any, len(byAuthenticator))
	for id, v := range byAuthenticator {
		credentials, ok := v.([]any)
		if !ok || len(credentials) == 0 {
			continue
		}
		entries := make([]any, len(credentials))
		for i := range entries {
			entries[i] = map[string]any{}
		}
		reduced[id] = entries
	}
	if len(reduced) == 0 {
		delete(m, "webauthnCredentials")
		return
	}
	m["webauthnCredentials"] = reduced
}

// reduceMFASecretsToFlags keeps UserIdentity.mfaSecrets keys and each entry's mfaSecretFlags,
// dropping the secrets. Empty or malformed mfaSecrets maps are removed.
func reduceMFASecretsToFlags(m map[string]any) {
	raw, ok := m["mfaSecrets"]
	if !ok {
		return
	}
	secrets, ok := raw.(map[string]any)
	if !ok {
		delete(m, "mfaSecrets")
		return
	}
	reduced := make(map[string]any, len(secrets))
	for id, v := range secrets {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		flags := make(map[string]any, len(mfaSecretFlags))
		for _, key := range mfaSecretFlags {
			if value, ok := entry[key]; ok {
				flags[key] = value
			}
		}
		reduced[id] = flags
	}
	if len(reduced) == 0 {
		delete(m, "mfaSecrets")
		return
	}
	m["mfaSecrets"] = reduced
}

// reduceRefs keeps the keys of the references in OfflineSessions.<field> and each entry's
// refFields, dropping anything else. An entry without an ID is dropped; empty or malformed maps are
// removed.
func reduceRefs(m map[string]any, field string) {
	raw, ok := m[field]
	if !ok {
		return
	}
	refs, ok := raw.(map[string]any)
	if !ok {
		delete(m, field)
		return
	}
	reduced := make(map[string]any, len(refs))
	for k, v := range refs {
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		id, ok := entry["ID"].(string)
		if !ok || id == "" {
			id, ok = entry["id"].(string)
		}
		if !ok || id == "" {
			continue
		}
		kept := make(map[string]any, len(refFields))
		for _, key := range refFields {
			if value, ok := entry[key]; ok {
				kept[key] = value
			}
		}
		kept["ID"] = id
		reduced[k] = kept
	}
	if len(reduced) == 0 {
		delete(m, field)
		return
	}
	m[field] = reduced
}
