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
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MaskedValue is what kube-apiserver returns instead of a field marked x-kubernetes-sensitive-data
// to a caller without the <resource>/sensitive subresource. In a sensitive field, a create with it
// is refused and an update that sends it back keeps the stored value.
const MaskedValue = "<omitted>"

// ErrSensitiveMasked reports a spec that still reads as MaskedValue from the API server: the
// controller is not allowed to read the sensitive subresource of the resource.
var ErrSensitiveMasked = errors.New("a sensitive field reads as " + MaskedValue +
	": the controller is not allowed to read the sensitive subresource of the resource")

// GetUnmasked reads obj through c, the cache. The cache keeps the objects as they came when the
// controller did not have the sensitive subresource yet: the CRDs with the marker are applied
// before the module release grants it, and a cached object changes only with its next event. When
// a sensitive field holds MaskedValue, GetUnmasked reads the object again through apiReader, past the cache,
// and fails with ErrSensitiveMasked if it is still masked.
func GetUnmasked(ctx context.Context, c, apiReader client.Reader, key client.ObjectKey, obj *unstructured.Unstructured) error {
	if err := c.Get(ctx, key, obj); err != nil {
		return err
	}
	if !hasMaskedSensitiveField(obj) {
		return nil
	}

	if err := apiReader.Get(ctx, key, obj); err != nil {
		return err
	}
	if hasMaskedSensitiveField(obj) {
		return fmt.Errorf("%s %s: %w", obj.GetKind(), key.Name, ErrSensitiveMasked)
	}
	return nil
}

// hasMaskedSensitiveField checks the fields marked x-kubernetes-sensitive-data in the module CRDs.
func hasMaskedSensitiveField(obj *unstructured.Unstructured) bool {
	paths := [][]string{}
	switch obj.GroupVersionKind().GroupKind() {
	case UserGVK.GroupKind():
		paths = [][]string{{"spec", "password"}}
	case UserOperationGVK.GroupKind():
		paths = [][]string{{"spec", "resetPassword", "newPasswordHash"}}
	case DexProviderGVK.GroupKind():
		paths = [][]string{
			{"spec", "github", "clientSecret"},
			{"spec", "gitlab", "clientSecret"},
			{"spec", "bitbucketCloud", "clientSecret"},
			{"spec", "crowd", "clientSecret"},
			{"spec", "oidc", "clientSecret"},
			{"spec", "ldap", "bindPW"},
		}
	}
	for _, path := range paths {
		value, _, err := unstructured.NestedString(obj.Object, path...)
		if err == nil && value == MaskedValue {
			return true
		}
	}
	return false
}
