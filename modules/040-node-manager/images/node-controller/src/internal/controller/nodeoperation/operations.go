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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

// adopt gives an operator's operation the node owner reference the ones
// node-manager creates carry, so deleting the node collects it too.
func (r *Reconciler) adopt(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node) error {
	if len(op.OwnerReferences) > 0 {
		return nil
	}
	patch := client.MergeFrom(op.DeepCopy())
	op.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "Node",
		Name:       node.Name,
		UID:        node.UID,
	}}
	if err := r.Client.Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("adopt %s onto %s: %w", op.Name, node.Name, err)
	}
	return nil
}

// operationsOfNode lists every operation recorded against a node. A write to the node is
// decided from a read past the cache: a cached list missing a just-created one is how a node
// ended up with two operations that each thought they were alone.
func operationsOfNode(ctx context.Context, reader client.Reader, nodeName string) ([]v1alpha1.NodeOperation, error) {
	list := &v1alpha1.NodeOperationList{}
	if err := reader.List(ctx, list, client.MatchingFields{v1alpha1.NodeOperationNodeNameField: nodeName}); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// machineDrain reports the Drain node-manager created for a deleted Cluster API
// Machine, see internal/controller/machinedrain.
func machineDrain(op *v1alpha1.NodeOperation) bool {
	_, ofMachine := op.Labels[v1alpha1.NodeOperationMachineUIDLabel]
	return ofMachine && op.Spec.Type == v1alpha1.NodeOperationTypeDrain
}

// endForMachine fails every other live operation of the node: the machine is
// being removed, and its Drain must not wait behind them. Each one gives its
// drain request back at once, so the Drain takes it on this very pass.
func (r *Reconciler) endForMachine(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) error {
	// Past the cache: failing an operation is final.
	others, err := operationsOfNode(ctx, r.apiReader, op.Spec.NodeName)
	if err != nil {
		return fmt.Errorf("list the operations of %s: %w", op.Spec.NodeName, err)
	}
	message := fmt.Sprintf("Machine %s is deleted. node-manager ended this operation so that the machine can be removed",
		op.Annotations[v1alpha1.NodeOperationMachineAnnotation])
	for i := range others {
		other := &others[i]
		if other.UID == op.UID || terminal(other) {
			continue
		}
		// An operation of an unknown type refuses every write, see Reconcile.
		if !knownType(other.Spec.Type) {
			continue
		}
		if err := r.fail(ctx, other, "MachineDeleted", message, logger); err != nil {
			return err
		}
		if err := r.releaseNode(ctx, other, logger); err != nil {
			return err
		}
	}
	return nil
}
