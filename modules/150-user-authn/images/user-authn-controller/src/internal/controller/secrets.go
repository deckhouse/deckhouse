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
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/naming"
)

// secretFields are the top-level keys of Dex storage objects that hold credentials:
// Password hash and previousHashes, connectorData of OfflineSessions and RefreshToken, the TOTP
// secret of OfflineSessions, refresh token values, the WebAuthn credentials of UserIdentity, and
// the cookie secret and nonce of AuthSession. The TOTP secrets of UserIdentity (mfaSecrets) are
// reduced to mfaSecretFlags instead.
var secretFields = []string{
	"hash",
	"previousHashes",
	"connectorData",
	"totp",
	"token",
	"obsoleteToken",
	"webauthnCredentials",
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
// and UserIdentity.mfaSecrets to mfaSecretFlags.
func SecretFields() []string {
	return slices.Clone(secretFields)
}

// InformerCacheByObject scopes Password, OfflineSessions, RefreshToken, UserIdentity, and
// AuthSession informers to d8-user-authn and strips Dex secrets before the objects are stored.
// Code that needs a secret field (the UserIdentity migration reads the TOTP secret) reads through
// the API reader instead. Revocation objects hold no secrets; they lose only their managed fields.
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
			Transform: StripDexSecrets,
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

// StripDexSecrets clones an unstructured object and deletes Dex secret fields.
// The input object is never mutated.
func StripDexSecrets(obj any) (any, error) {
	switch typed := obj.(type) {
	case *unstructured.Unstructured:
		if typed == nil {
			return obj, nil
		}
		cloned := typed.DeepCopy()
		stripSecretKeys(cloned.Object)
		return cloned, nil
	case unstructured.Unstructured:
		cloned := typed.DeepCopy()
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
