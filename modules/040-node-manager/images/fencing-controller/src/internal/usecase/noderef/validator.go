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

// Package noderef tells if an incident belongs to the Node that has its name
// now.
package noderef

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// Validator checks incidents against the Node they name.
type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

// Identifies tells if node exists and an owner reference of the incident has
// the UID of node. node is the Node with the name of the incident, or nil when
// there is no such Node. A Node with the same name and another UID is a new
// Node: the controller must not delete its pods for the incident of the old
// Node.
func (v *Validator) Identifies(incident *v1alpha1.FencingFailedNodeState, node *corev1.Node) bool {
	if node == nil {
		return false
	}

	return slices.ContainsFunc(incident.OwnerReferences, func(owner metav1.OwnerReference) bool {
		return owner.UID == node.UID
	})
}
