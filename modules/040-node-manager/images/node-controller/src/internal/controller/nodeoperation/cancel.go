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
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

// holdDrain puts the finalizer on a Drain before it writes its request, so a Drain
// deleted without the finalizer has left nothing on the node.
func (r *Reconciler) holdDrain(ctx context.Context, op *v1alpha1.NodeOperation) error {
	if op.Spec.Type != v1alpha1.NodeOperationTypeDrain || controllerutil.ContainsFinalizer(op, drainFinalizer) {
		return nil
	}
	patch := client.MergeFromWithOptions(op.DeepCopy(), client.MergeFromWithOptimisticLock{})
	controllerutil.AddFinalizer(op, drainFinalizer)
	if err := r.Client.Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("add the finalizer to %s: %w", op.Name, err)
	}
	return nil
}

// cancel lets a deleted Drain go. A running one withdraws its request; a failed one
// is released like any failed operation; a completed one keeps the node out.
func (r *Reconciler) cancel(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) error {
	if !controllerutil.ContainsFinalizer(op, drainFinalizer) {
		return nil
	}
	switch {
	case op.Status.Phase == v1alpha1.NodeOperationPhaseFailed:
		if err := r.releaseNode(ctx, op, logger); err != nil {
			return err
		}
	case !terminal(op):
		if err := r.withdrawDrain(ctx, op, logger); err != nil {
			return err
		}
	}
	patch := client.MergeFromWithOptions(op.DeepCopy(), client.MergeFromWithOptimisticLock{})
	controllerutil.RemoveFinalizer(op, drainFinalizer)
	if err := r.Client.Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("remove the finalizer of %s: %w", op.Name, err)
	}
	return nil
}

// withdrawDrain takes the request of a cancelled Drain off its node and leaves the
// cordon, as withdrawing a hand drain does. The draining controller stops the eviction.
func (r *Reconciler) withdrawDrain(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) error {
	node := &corev1.Node{}
	if err := r.apiReader.Get(ctx, types.NamespacedName{Name: op.Spec.NodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get node %s: %w", op.Spec.NodeName, err)
	}
	annotations := markerErasure(node, drainMarker(op))
	if len(annotations) == 0 {
		return nil
	}
	body, err := json.Marshal(map[string]any{"metadata": map[string]any{"annotations": annotations}})
	if err != nil {
		return fmt.Errorf("build the withdrawal patch for %s: %w", node.Name, err)
	}
	if err := r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, body)); err != nil {
		return fmt.Errorf("withdraw the drain request of %s from %s: %w", op.Name, node.Name, err)
	}
	logger.Info("drain request withdrawn with its deleted operation", "node", node.Name, "operation", op.Name)
	return nil
}
