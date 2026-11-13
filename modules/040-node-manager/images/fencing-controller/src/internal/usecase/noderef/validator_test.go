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

package noderef

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

const (
	nodeName = "worker-3"
	nodeUID  = types.UID("1a2b3c4d-1111-2222-3333-444455556666")
	// otherUID is the identity of a Node that was recreated under the same name.
	otherUID = types.UID("99998888-7777-6666-5555-444433332222")
)

// TestIdentifiesTheOwnerNode covers the Node with the name of the object: the
// owner, a new Node with the same name, and no Node.
func TestIdentifiesTheOwnerNode(t *testing.T) {
	for name, tc := range map[string]struct {
		node *corev1.Node
		want bool
	}{
		"the owner node":                      {node: node(nodeName, nodeUID), want: true},
		"a new node with the same name":       {node: node(nodeName, otherUID)},
		"no node with the name of the object": {},
	} {
		t.Run(name, func(t *testing.T) {
			if got := NewValidator().Identifies(incident(ownerOf(nodeName, nodeUID)), tc.node); got != tc.want {
				t.Errorf("identifies is %t, want %t", got, tc.want)
			}
		})
	}
}

func incident(owners ...metav1.OwnerReference) *v1alpha1.FencingFailedNodeState {
	return &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, OwnerReferences: owners},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			Armed:      true,
			ProfileRef: &v1alpha1.ProfileRef{Name: v1alpha1.ProfileCritical},
		},
	}
}

func node(name string, uid types.UID) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid}}
}

// ownerOf is the owner reference the agent is required to create the object
// with: apiVersion v1, kind Node, and the name and UID of that Node.
func ownerOf(name string, uid types.UID) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: corev1.SchemeGroupVersion.String(),
		Kind:       "Node",
		Name:       name,
		UID:        uid,
	}
}
