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

// Package noderef answers the one question the controller has to settle before
// it acts on an incident: does this object identify the live Node it names.
//
// The CRD cannot answer it. Kubernetes CEL validation has no access to
// metadata.ownerReferences and does no lookup of other objects, so the identity
// invariants are not duplicated in the spec and are checked here instead, at
// runtime, against the Node as it currently is.
package noderef

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
)

// nodeKind is the kind an owner reference of an incident has to name.
const nodeKind = "Node"

// Problem is a broken reference from an incident to its Node, reported both as
// the machine-readable reason of the condition and as a message that names the
// values an operator would have to compare by hand.
type Problem struct {
	Reason  string
	Message string
}

// Nodes reads the Node an incident names.
type Nodes interface {
	GetNode(ctx context.Context, name string) (*corev1.Node, error)
}

// Validator checks incidents against the Node they name.
type Validator struct {
	nodes Nodes
}

func NewValidator(nodes Nodes) *Validator {
	return &Validator{nodes: nodes}
}

// Validate reports the first broken invariant of the reference from the incident
// to its Node, or nil when the object identifies that Node. An error means the
// question could not be answered at all, which is transient and worth a retry;
// a Problem is an answer, and a definite one.
//
// The rules and their order are those of the pre-reconcile validation of the
// ADR: the Node is read first, so an incident whose Node is gone is reported as
// gone rather than as structurally broken.
func (v *Validator) Validate(ctx context.Context, incident *v1alpha1.FencingFailedNodeState) (*Problem, error) {
	if incident == nil {
		return nil, errors.New("fencingfailednodestate is nil")
	}

	node, err := v.nodes.GetNode(ctx, incident.Name)

	switch {
	case apierrors.IsNotFound(err):
		return &Problem{
			Reason:  common.ReasonNodeNotFound,
			Message: fmt.Sprintf("Node %q does not exist.", incident.Name),
		}, nil
	case err != nil:
		return nil, err
	}

	return check(incident, node.UID), nil
}

// check compares the owner reference of the incident with the identity of the
// Node that was read for it.
func check(incident *v1alpha1.FencingFailedNodeState, uid types.UID) *Problem {
	owner, problem := ownerReference(incident)
	if problem != nil {
		return problem
	}

	// metadata.name is the key of the object and the name of the target Node for
	// the controller, kubectl and every integration, so an owner reference that
	// names a different Node makes the object ambiguous about which Node it is
	// about.
	if owner.Name != incident.Name {
		return &Problem{
			Reason: common.ReasonNameMismatch,
			Message: fmt.Sprintf("Owner reference names node %q, but the object is named after node %q.",
				owner.Name, incident.Name),
		}
	}

	// The UID is what tells a recreated Node from the one the incident was
	// created for. Acting on a mismatch would delete the pods of a Node that
	// has never been reported as failed.
	if owner.UID != uid {
		return &Problem{
			Reason: common.ReasonUIDMismatch,
			Message: fmt.Sprintf("Owner reference points at UID %q, but node %q currently has UID %q, so the object refers to a node that no longer exists.",
				owner.UID, incident.Name, uid),
		}
	}

	return nil
}

// ownerReference returns the single owner reference to a Node the ADR requires.
// Everything it rejects is reported as a missing owner reference, because in all
// of those shapes the object has no usable reference to a Node: without exactly
// one, there is nothing to compare the Node against, and the garbage collector
// has nothing to delete the object by either.
func ownerReference(incident *v1alpha1.FencingFailedNodeState) (metav1.OwnerReference, *Problem) {
	owners := incident.OwnerReferences

	if len(owners) != 1 {
		return metav1.OwnerReference{}, &Problem{
			Reason: common.ReasonMissingOwnerReference,
			Message: fmt.Sprintf("Object has %d owner references, want exactly one reference to node %q.",
				len(owners), incident.Name),
		}
	}

	owner := owners[0]

	if owner.APIVersion != corev1.SchemeGroupVersion.String() || owner.Kind != nodeKind {
		return metav1.OwnerReference{}, &Problem{
			Reason: common.ReasonMissingOwnerReference,
			Message: fmt.Sprintf("Owner reference is %s %s, want %s %s.",
				owner.APIVersion, owner.Kind, corev1.SchemeGroupVersion.String(), nodeKind),
		}
	}

	return owner, nil
}
