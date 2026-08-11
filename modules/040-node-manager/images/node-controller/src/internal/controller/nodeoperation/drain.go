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
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// startDrain hands the node to the draining controller, which evicts the pods
// and reports back through the drained annotation. The request carries this
// operation's marker, so the answer comes back carrying it too.
func (r *Reconciler) startDrain(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node, logger logr.Logger) error {
	patch := client.MergeFrom(node.DeepCopy())
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	node.Annotations[nodecommon.DrainingAnnotation] = drainMarker(op)
	if err := r.Client.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("start drain of %s: %w", node.Name, err)
	}
	logger.Info("draining the node for the operation", "node", node.Name, "operation", op.Name)
	return nil
}

func drainRequested(op *v1alpha1.NodeOperation) bool {
	return meta.IsStatusConditionTrue(op.Status.Conditions, conditionDrainRequested)
}

// drainMarker is the value this operation writes into the node's drain
// annotations and the only value it reads back or erases: the identity lets
// operations share a node without clearing each other's (or bashible's) marker.
func drainMarker(op *v1alpha1.NodeOperation) string {
	return drainingSource + "/" + string(lineageUID(op))
}

func lineageUID(op *v1alpha1.NodeOperation) types.UID {
	for _, owner := range op.OwnerReferences {
		if owner.Kind == nodeOperationKind {
			return owner.UID
		}
	}
	return op.UID
}

// idle reports that the node carries no drain of this operation's — neither a
// request waiting to be picked up nor an answer. Something removed them, so the
// eviction is not going to arrive unless it is asked for again.
func idle(op *v1alpha1.NodeOperation, node *corev1.Node) bool {
	marker := drainMarker(op)
	return node.Annotations[nodecommon.DrainingAnnotation] != marker &&
		node.Annotations[nodecommon.DrainedAnnotation] != marker
}

// requestedByAnother reports that the node's one drain request already belongs
// to somebody else. Presence decides, not the value: an empty value is the
// mutable path's own request (see the draining controller).
func requestedByAnother(op *v1alpha1.NodeOperation, node *corev1.Node) bool {
	requested, ok := node.Annotations[nodecommon.DrainingAnnotation]
	return ok && requested != drainMarker(op)
}

// recordDrainRequested remembers that the eviction has been asked for and, the
// first time only, until when it may run. The deadline is written once and
// never moved: a deadline renewed on every re-issue is no deadline.
func (r *Reconciler) recordDrainRequested(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node) error {
	patch := client.MergeFromWithOptions(op.DeepCopy(), client.MergeFromWithOptimisticLock{})
	meta.SetStatusCondition(&op.Status.Conditions, metav1.Condition{
		Type:               conditionDrainRequested,
		Status:             metav1.ConditionTrue,
		Reason:             "Draining",
		Message:            fmt.Sprintf("the draining controller was asked to empty %s", op.Spec.NodeName),
		ObservedGeneration: op.Generation,
	})
	if op.Status.DrainDeadline == nil {
		op.Status.DrainDeadline = ptr.To(metav1.NewTime(time.Now().Add(r.drainTimeout(ctx, node))))
	}
	if err := r.Client.Status().Patch(ctx, op, patch); err != nil {
		return fmt.Errorf("record that %s asked for its eviction: %w", op.Name, err)
	}
	return nil
}

func skipDrain(op *v1alpha1.NodeOperation) bool {
	return op.Spec.Drain != nil && op.Spec.Drain.Skip
}

func drained(op *v1alpha1.NodeOperation, node *corev1.Node) bool {
	return node.Annotations[nodecommon.DrainedAnnotation] == drainMarker(op)
}
