/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	networkv1alpha1 "service-with-healthchecks/api/v1alpha1"
)

// ServiceWithHealthchecksKind is the kind an owner reference of the module points to.
const ServiceWithHealthchecksKind = "ServiceWithHealthchecks"

// IsOwnedByServiceWithHealthchecks reports whether the object is controlled by the
// ServiceWithHealthchecks of that name. Both components ask this question: the controller to
// decide whether it may manage the child Service, the agents to decide whether they may publish
// EndpointSlices under its name.
func IsOwnedByServiceWithHealthchecks(object metav1.Object, name string) bool {
	ref := metav1.GetControllerOf(object)
	return ref != nil && referencesServiceWithHealthchecks(ref, name)
}

// ForeignControllerReference returns the controller reference of the object when it points to
// something other than the ServiceWithHealthchecks of that name. An object without a controller
// reference belongs to nobody yet, which is not a conflict on its own.
//
// Only a controller reference means ownership: a plain owner reference is an extra garbage
// collection link that anything may add, and giving up an object because of one would be worse
// than the clash it is meant to catch.
func ForeignControllerReference(object metav1.Object, name string) *metav1.OwnerReference {
	ref := metav1.GetControllerOf(object)
	if ref == nil || referencesServiceWithHealthchecks(ref, name) {
		return nil
	}
	return ref
}

// referencesServiceWithHealthchecks deliberately ignores the UID: a reference to the same name is
// still ours after the parent was recreated, and the controller replaces the stale UID on its next
// pass instead of treating its own object as somebody else's.
func referencesServiceWithHealthchecks(ref *metav1.OwnerReference, name string) bool {
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	return err == nil &&
		gv.Group == networkv1alpha1.GroupVersion.Group &&
		ref.Kind == ServiceWithHealthchecksKind &&
		ref.Name == name
}
