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

package ctrlutils

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OwnerRefName returns the name of obj's owner reference of the given kind, or an empty
// string when it has none. It is how a switch to a different owner is detected.
func OwnerRefName(obj client.Object, kind string) string {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind == kind {
			return ref.Name
		}
	}

	return ""
}

// OwnerReference builds a non-controller owner reference that blocks the owner's deletion,
// so an object cannot be deleted from under the dependent still using it.
func OwnerReference(gvk schema.GroupVersionKind, name string, uid types.UID) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         gvk.GroupVersion().String(),
		Kind:               gvk.Kind,
		Name:               name,
		UID:                uid,
		Controller:         new(false),
		BlockOwnerDeletion: new(true),
	}
}

// ReplaceOwnerReferences swaps obj's owner references of the same kinds as refs for refs
// themselves, leaving references of every other kind untouched. Replacing per kind is what
// drops the reference to an owner the object has just switched away from.
func ReplaceOwnerReferences(obj client.Object, refs ...metav1.OwnerReference) {
	kinds := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		kinds[ref.Kind] = struct{}{}
	}

	existing := obj.GetOwnerReferences()

	kept := make([]metav1.OwnerReference, 0, len(existing)+len(refs))
	for _, ref := range existing {
		if _, replaced := kinds[ref.Kind]; replaced {
			continue
		}

		kept = append(kept, ref)
	}

	obj.SetOwnerReferences(append(kept, refs...))
}

// DropOwnerReferences removes obj's owner references of the given kinds. It is
// ReplaceOwnerReferences with nothing to put back, for an object that no longer has an owner of
// that kind at all — a reference left behind blocks its owner's deletion for ever.
func DropOwnerReferences(obj client.Object, kinds ...string) {
	existing := obj.GetOwnerReferences()

	kept := make([]metav1.OwnerReference, 0, len(existing))
	for _, ref := range existing {
		if !slices.Contains(kinds, ref.Kind) {
			kept = append(kept, ref)
		}
	}

	obj.SetOwnerReferences(kept)
}
