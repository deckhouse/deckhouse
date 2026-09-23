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

package nodeoperation

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

// adopt gives an operation its node owner reference (node deletion collects
// it) and node label (how siblings find each other). An operator's arrives
// with neither, and unlabelled it would leave the node cordoned for good.
func (r *Reconciler) adopt(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node) error {
	if len(op.OwnerReferences) > 0 && op.Labels[v1alpha1.NodeOperationNodeLabel] == op.Spec.NodeName {
		return nil
	}
	patch := client.MergeFrom(op.DeepCopy())
	if op.Labels == nil {
		op.Labels = map[string]string{}
	}
	op.Labels[v1alpha1.NodeOperationNodeLabel] = op.Spec.NodeName
	if len(op.OwnerReferences) == 0 {
		op.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "v1",
			Kind:       "Node",
			Name:       node.Name,
			UID:        node.UID,
		}}
	}
	if err := r.Client.Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("adopt %s onto %s: %w", op.Name, node.Name, err)
	}
	return nil
}

// operationsOfNode lists every operation recorded against a node. The read goes
// straight to the API server: a cached list missing a just-created one is how a
// node ended up with two operations that each thought they were alone.
func (r *Reconciler) operationsOfNode(ctx context.Context, nodeName string) ([]v1alpha1.NodeOperation, error) {
	list := &v1alpha1.NodeOperationList{}
	if err := r.apiReader.List(ctx, list, client.MatchingLabels{v1alpha1.NodeOperationNodeLabel: nodeName}); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// drainOf finds the eviction this operation spawned, by ownership rather than
// name. The read goes straight to the API server: a cached list missing a
// just-created child is how one operation got two evictions of the same node.
func (r *Reconciler) drainOf(ctx context.Context, op *v1alpha1.NodeOperation) (*v1alpha1.NodeOperation, error) {
	children, err := r.operationsOfNode(ctx, op.Spec.NodeName)
	if err != nil {
		return nil, fmt.Errorf("list the drains of %s: %w", op.Name, err)
	}
	for i := range children {
		child := &children[i]
		if child.Spec.Type == v1alpha1.NodeOperationTypeDrain && ownedBy(child, op) {
			return child, nil
		}
	}
	return nil, nil
}

// ownedBy reports whether the child was created for this exact operation, not
// for an earlier one of the same name.
func ownedBy(child, parent *v1alpha1.NodeOperation) bool {
	return slices.ContainsFunc(child.OwnerReferences, func(owner metav1.OwnerReference) bool {
		return owner.UID == parent.UID
	})
}
