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

// Package hookcache shrinks the objects that the informers of hooks keep in memory.
package hookcache

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// StripUnusedFields drops the fields that no hook reads before the informers of hooks
// cache and filter an object: metadata.managedFields of every object and status.images
// of Nodes. On large clusters these fields take most of the cache.
//
// It changes the object in place, is idempotent and returns anything that is not
// *unstructured.Unstructured, e.g. cache.DeletedFinalStateUnknown, as is.
func StripUnusedFields(obj any) (any, error) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return obj, nil
	}

	// Not cache.TransformStripManagedFields: on unstructured objects it first converts
	// every managedFields entry to check that the field is set.
	unstructured.RemoveNestedField(u.Object, "metadata", "managedFields")

	if u.GetAPIVersion() == "v1" && u.GetKind() == "Node" {
		unstructured.RemoveNestedField(u.Object, "status", "images")
	}

	return u, nil
}
