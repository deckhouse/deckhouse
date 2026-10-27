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

package namespace

import (
	"reflect"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"controller/apis/deckhouse.io/v1alpha3"
)

// MirrorNamespaceOwners makes the Project carry the ownerReferences of its main namespace, so that
// the garbage collector deletes the Project together with a namespace whose owner is gone, instead
// of the project reconcile creating the namespace again from the template. It reports whether it
// changed the Project.
//
// A namespace the release of the project created has no ownerReferences: Helm never sets them, and
// the admission policy keeps them from everyone but the identities it lets through. So the namespaces
// with ownerReferences are the ones that existed before they were adopted, and the Projects of the
// other namespaces -- the ones created by users -- are left as they are. The copied entries are
// recorded in the ProjectAnnotationNamespaceOwners annotation: only they are updated or removed
// later, and the ownerReferences the Project has of its own stay, an owner the namespace shares with
// them included. A copy sets neither controller,
// as the Project may have a controller of its own, nor blockOwnerDeletion, which would require the
// right to delete the owner.
//
// A namespace that is gone (nil) changes nothing: its deletion is either the garbage collector's,
// which deletes the Project as well, or a direct one, after which the recreated namespace has no
// owners and the next reconcile drops the copies.
func MirrorNamespaceOwners(project *v1alpha3.Project, namespace *corev1.Namespace) bool {
	if namespace == nil {
		return false
	}
	mirrored := mirroredOwners(project)
	if len(mirrored) == 0 && len(namespace.OwnerReferences) == 0 {
		return false
	}

	// An ownerReference the Project has of its own, one the annotation does not record, stays as it
	// is, also when the namespace has an owner with the same UID: that owner is neither copied nor
	// recorded, so that its removal from the namespace does not take the Project's own reference away.
	var own []types.UID
	for _, ref := range project.OwnerReferences {
		if !slices.Contains(mirrored, ref.UID) {
			own = append(own, ref.UID)
		}
	}

	desired := make([]metav1.OwnerReference, 0, len(namespace.OwnerReferences))
	desiredUIDs := make([]string, 0, len(namespace.OwnerReferences))
	for _, ref := range namespace.OwnerReferences {
		if slices.Contains(desiredUIDs, string(ref.UID)) || slices.Contains(own, ref.UID) {
			continue
		}
		desired = append(desired, metav1.OwnerReference{
			APIVersion: ref.APIVersion,
			Kind:       ref.Kind,
			Name:       ref.Name,
			UID:        ref.UID,
		})
		desiredUIDs = append(desiredUIDs, string(ref.UID))
	}

	refs := make([]metav1.OwnerReference, 0, len(project.OwnerReferences)+len(desired))
	for _, ref := range project.OwnerReferences {
		if slices.Contains(mirrored, ref.UID) {
			continue
		}
		refs = append(refs, ref)
	}
	refs = append(refs, desired...)
	if len(refs) == 0 {
		refs = nil
	}

	changed := false
	if !reflect.DeepEqual(refs, project.OwnerReferences) {
		project.OwnerReferences = refs
		changed = true
	}

	slices.Sort(desiredUIDs)
	record := strings.Join(desiredUIDs, ",")
	switch current, ok := project.Annotations[v1alpha3.ProjectAnnotationNamespaceOwners]; {
	case record == "" && ok:
		delete(project.Annotations, v1alpha3.ProjectAnnotationNamespaceOwners)
		changed = true
	case record != "" && current != record:
		if project.Annotations == nil {
			project.Annotations = map[string]string{}
		}
		project.Annotations[v1alpha3.ProjectAnnotationNamespaceOwners] = record
		changed = true
	}
	return changed
}

// mirroredOwners reads the UIDs the ProjectAnnotationNamespaceOwners annotation records.
func mirroredOwners(project *v1alpha3.Project) []types.UID {
	var uids []types.UID
	for _, uid := range strings.Split(project.Annotations[v1alpha3.ProjectAnnotationNamespaceOwners], ",") {
		if uid = strings.TrimSpace(uid); uid != "" {
			uids = append(uids, types.UID(uid))
		}
	}
	return uids
}
