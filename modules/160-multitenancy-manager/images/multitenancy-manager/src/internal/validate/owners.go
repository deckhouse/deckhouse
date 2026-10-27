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

package validate

import (
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddedOwnerReference returns a denial message when an update gives an object an ownerReference it did
// not have, or changes one it had, and an empty string otherwise. The garbage collector deletes an
// object whose owner is gone, and an owner that does not exist, or exists under another UID, is gone
// for it: an added ownerReference deletes a Project or a ProjectNamespace as the garbage collector,
// which the project delete check lets through, and their finalizers then delete the namespaces. A
// removed ownerReference deletes nothing, and the garbage collector removes them itself when an owner
// is deleted with the orphan policy, so removals pass.
func AddedOwnerReference(kind, name string, old, current []metav1.OwnerReference) string {
	for _, ref := range current {
		if slices.ContainsFunc(old, func(o metav1.OwnerReference) bool { return equality.Semantic.DeepEqual(o, ref) }) {
			continue
		}
		return fmt.Sprintf("the ownerReferences of the %q %s cannot be added or changed: the %s %s %q is not among them",
			name, kind, ref.APIVersion, ref.Kind, ref.Name)
	}
	return ""
}
