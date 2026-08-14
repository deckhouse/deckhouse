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

var secretFields = []string{"hash", "previousHashes", "connectorData", "totp", "token"}

// SecretFields returns Dex credential keys stripped from the informer cache
// and UserAccount. OfflineSessions.refresh is reduced to token IDs so Reset2FA
// can match sessions that lack userID.
func SecretFields() []string {
	return slices.Clone(secretFields)
}

// InformerCacheByObject scopes Password, OfflineSessions, and RefreshToken
// informers to d8-user-authn and strips Dex secrets before the objects are stored.
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
	reduceRefreshToIDs(m)
}

// reduceRefreshToIDs keeps OfflineSessions.refresh keys and each entry's ID,
// dropping token material. Empty or malformed refresh maps are removed.
func reduceRefreshToIDs(m map[string]any) {
	raw, ok := m["refresh"]
	if !ok {
		return
	}
	refreshMap, ok := raw.(map[string]any)
	if !ok {
		delete(m, "refresh")
		return
	}
	reduced := make(map[string]any, len(refreshMap))
	for k, v := range refreshMap {
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
		reduced[k] = map[string]any{"ID": id}
	}
	if len(reduced) == 0 {
		delete(m, "refresh")
		return
	}
	m["refresh"] = reduced
}
