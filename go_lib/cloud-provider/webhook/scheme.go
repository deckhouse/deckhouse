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

package webhook

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RegisterUnstructuredGVKs registers placeholder Unstructured types for the given GVKs.
// controller-runtime webhook builder requires known GroupKinds in the scheme even when
// validators operate on unstructured admission objects.
func RegisterUnstructuredGVKs(scheme *runtime.Scheme, gvks ...schema.GroupVersionKind) error {
	if scheme == nil {
		return fmt.Errorf("scheme is required")
	}

	seenGroupVersions := make(map[schema.GroupVersion]struct{}, len(gvks))

	for _, gvk := range gvks {
		if gvk.Group == "" || gvk.Version == "" || gvk.Kind == "" {
			return fmt.Errorf("invalid GVK %#v", gvk)
		}

		scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})

		gv := gvk.GroupVersion()
		if _, ok := seenGroupVersions[gv]; ok {
			continue
		}

		seenGroupVersions[gv] = struct{}{}
		metav1.AddToGroupVersion(scheme, gv)
	}

	return nil
}
