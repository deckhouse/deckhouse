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
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
)

const (
	nodeName = "worker-3"
	nodeUID  = types.UID("1a2b3c4d-1111-2222-3333-444455556666")
	// otherUID is the identity of a Node that was recreated under the same name.
	otherUID = types.UID("99998888-7777-6666-5555-444433332222")
)

// TestValidateAcceptsTheReferenceOfALiveNode covers the object the agent is
// supposed to create: named after the Node, owned by that Node, same UID.
func TestValidateAcceptsTheReferenceOfALiveNode(t *testing.T) {
	problem, err := validator(node(nodeName, nodeUID)).Validate(context.Background(), incident(ownerOf(nodeName, nodeUID)))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if problem != nil {
		t.Errorf("valid reference was refused as %s: %s", problem.Reason, problem.Message)
	}
}

// TestValidateReportsEveryBrokenInvariant walks the rules of the pre-reconcile
// validation of the ADR, one case per machine-readable reason it names.
func TestValidateReportsEveryBrokenInvariant(t *testing.T) {
	for name, tc := range map[string]struct {
		nodes      Nodes
		owners     []metav1.OwnerReference
		wantReason string
		// wantInMessage are the values an operator has to be told to make sense
		// of the refusal.
		wantInMessage []string
	}{
		"node is gone": {
			nodes:         &stubNodes{},
			owners:        []metav1.OwnerReference{ownerOf(nodeName, nodeUID)},
			wantReason:    common.ReasonNodeNotFound,
			wantInMessage: []string{nodeName},
		},
		"no owner reference at all": {
			owners:        nil,
			wantReason:    common.ReasonMissingOwnerReference,
			wantInMessage: []string{"0", nodeName},
		},
		"more than one owner reference": {
			owners: []metav1.OwnerReference{
				ownerOf(nodeName, nodeUID),
				ownerOf("worker-4", otherUID),
			},
			wantReason:    common.ReasonMissingOwnerReference,
			wantInMessage: []string{"2"},
		},
		"owner is not a core object": {
			owners: []metav1.OwnerReference{{
				APIVersion: "node-manager.deckhouse.io/v1alpha1",
				Kind:       "Node",
				Name:       nodeName,
				UID:        nodeUID,
			}},
			wantReason:    common.ReasonMissingOwnerReference,
			wantInMessage: []string{"node-manager.deckhouse.io/v1alpha1"},
		},
		"owner is not a node": {
			owners: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       nodeName,
				UID:        nodeUID,
			}},
			wantReason:    common.ReasonMissingOwnerReference,
			wantInMessage: []string{"Pod"},
		},
		"owner names another node": {
			owners:        []metav1.OwnerReference{ownerOf("worker-4", nodeUID)},
			wantReason:    common.ReasonNameMismatch,
			wantInMessage: []string{"worker-4", nodeName},
		},
		"node was recreated": {
			owners:        []metav1.OwnerReference{ownerOf(nodeName, otherUID)},
			wantReason:    common.ReasonUIDMismatch,
			wantInMessage: []string{string(otherUID), string(nodeUID)},
		},
	} {
		t.Run(name, func(t *testing.T) {
			nodes := tc.nodes
			if nodes == nil {
				nodes = &stubNodes{node: node(nodeName, nodeUID)}
			}

			problem, err := NewValidator(nodes).Validate(context.Background(), incident(tc.owners...))
			if err != nil {
				t.Fatalf("validate: %v", err)
			}

			if problem == nil {
				t.Fatal("validate accepted the reference, want it refused")
			}

			if problem.Reason != tc.wantReason {
				t.Errorf("refused as %s, want %s", problem.Reason, tc.wantReason)
			}

			for _, want := range tc.wantInMessage {
				if !strings.Contains(problem.Message, want) {
					t.Errorf("message %q does not mention %q", problem.Message, want)
				}
			}
		})
	}
}

// TestValidateReadsTheNodeItIsNamedAfter pins which Node is looked up: the
// target is metadata.name of the object, not the name in its owner reference,
// so a reference that names another Node cannot redirect the lookup and pass.
func TestValidateReadsTheNodeItIsNamedAfter(t *testing.T) {
	nodes := &stubNodes{node: node(nodeName, nodeUID)}

	if _, err := NewValidator(nodes).Validate(context.Background(), incident(ownerOf("worker-4", nodeUID))); err != nil {
		t.Fatalf("validate: %v", err)
	}

	if nodes.asked != nodeName {
		t.Errorf("validate read node %q, want the %q the object is named after", nodes.asked, nodeName)
	}
}

// TestValidateKeepsTransientErrorsRetryable separates the two kinds of answer: a
// Problem is a decision about the object, an error means the API could not be
// asked and the incident has to be retried instead of refused.
func TestValidateKeepsTransientErrorsRetryable(t *testing.T) {
	apiDown := apierrors.NewServiceUnavailable("etcd leader changed")
	nodes := &stubNodes{err: apiDown}

	problem, err := NewValidator(nodes).Validate(context.Background(), incident(ownerOf(nodeName, nodeUID)))
	if !errors.Is(err, apiDown) {
		t.Fatalf("validate returned %v, want the API error so the incident is retried", err)
	}

	if problem != nil {
		t.Errorf("validate refused the reference as %s on an API failure, want no decision", problem.Reason)
	}
}

func TestValidateRejectsNoObject(t *testing.T) {
	if _, err := validator(node(nodeName, nodeUID)).Validate(context.Background(), nil); err == nil {
		t.Error("validate of a nil object succeeded, want an error")
	}
}

func validator(n *corev1.Node) *Validator {
	return NewValidator(&stubNodes{node: n})
}

func incident(owners ...metav1.OwnerReference) *v1alpha1.FencingFailedNodeState {
	return &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, OwnerReferences: owners},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			NodeGroup:  "worker",
			ProfileRef: v1alpha1.ProfileRef{Name: v1alpha1.ProfileCritical},
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

// stubNodes answers with one Node, or with the error it was given. A stub with
// neither reports the Node as missing, which is how the API answers for a Node
// that was deleted.
type stubNodes struct {
	node  *corev1.Node
	err   error
	asked string
}

func (s *stubNodes) GetNode(_ context.Context, name string) (*corev1.Node, error) {
	s.asked = name

	if s.err != nil {
		return nil, s.err
	}

	if s.node == nil || s.node.Name != name {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "nodes"}, name)
	}

	return s.node, nil
}
