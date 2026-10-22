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
	"strings"
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

// drainRequested reports whether the operation has asked for its eviction: from
// the ask on, its Drained condition is there, False until the workload has left.
func drainRequested(op *v1alpha1.NodeOperation) bool {
	return meta.FindStatusCondition(op.Status.Conditions, conditionDrained) != nil
}

// drainMarker is the value this operation writes into the node's drain
// annotations and the only value it reads back or erases: the identity lets
// operations share a node without clearing each other's (or bashible's) marker.
func drainMarker(op *v1alpha1.NodeOperation) string {
	return drainingSource + "/" + string(lineageUID(op))
}

// lineageUID is the UID of the operation the eviction is for. A Drain the
// previous controller created for another operation carries that operation's
// UID, so both read the same answer off the node.
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

// evictionHolder names who holds the node's one drain request. An operation's
// marker carries its lineage UID, which is the UID of the operation at its root.
func (r *Reconciler) evictionHolder(ctx context.Context, node *corev1.Node) (string, error) {
	requested := node.Annotations[nodecommon.DrainingAnnotation]
	uid, byOperation := strings.CutPrefix(requested, drainingSource+"/")
	if !byOperation {
		// An empty value is bashible's as well, see the draining controller.
		if requested == "" || requested == "bashible" {
			return "a bashible update", nil
		}
		return "a drain requested by " + requested, nil
	}
	ops, err := operationsOfNode(ctx, r.Client, node.Name)
	if err != nil {
		return "", fmt.Errorf("list the operations of %s: %w", node.Name, err)
	}
	for i := range ops {
		if ops[i].UID == types.UID(uid) {
			return "NodeOperation " + ops[i].Name, nil
		}
	}
	return fmt.Sprintf("a NodeOperation that no longer exists (UID %s)", uid), nil
}

// evictionFailure is the error the draining controller keeps hitting on this
// operation's eviction; empty while the eviction is not failing or not ours.
func evictionFailure(op *v1alpha1.NodeOperation, node *corev1.Node) string {
	if node.Annotations[nodecommon.DrainingAnnotation] != drainMarker(op) {
		return ""
	}
	return node.Annotations[nodecommon.DrainFailedAnnotation]
}

// evictionRefusals are the evictions the draining controller reports refused
// to this operation's drain; empty while none are.
func evictionRefusals(op *v1alpha1.NodeOperation) string {
	blocked := meta.FindStatusCondition(op.Status.Conditions, v1alpha1.NodeOperationConditionEvictionBlocked)
	if blocked == nil {
		return ""
	}
	return blocked.Message
}

// recordDrainRequested remembers that the eviction has been asked for and, the
// first time only, until when it may run. The deadline is written once and
// never moved: a deadline renewed on every re-issue is no deadline.
func (r *Reconciler) recordDrainRequested(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node) error {
	deadline := metav1.NewTime(time.Now().Add(
		nodecommon.DrainTimeout(ctx, r.Client, node.Labels[nodecommon.NodeGroupLabel])))

	return r.patchStatus(ctx, op, fmt.Sprintf("record that %s asked for its eviction", op.Name), func() {
		meta.SetStatusCondition(&op.Status.Conditions, metav1.Condition{
			Type:               conditionDrained,
			Status:             metav1.ConditionFalse,
			Reason:             "Draining",
			Message:            fmt.Sprintf("the draining controller is requested to evict the workload from node %s", op.Spec.NodeName),
			ObservedGeneration: op.Generation,
		})
		if op.Status.DrainDeadline == nil && !unlimitedDrain(op) {
			op.Status.DrainDeadline = ptr.To(deadline)
		}
	})
}

func skipDrain(op *v1alpha1.NodeOperation) bool {
	return op.Spec.Drain != nil && op.Spec.Drain.Skip
}

func drained(op *v1alpha1.NodeOperation, node *corev1.Node) bool {
	return node.Annotations[nodecommon.DrainedAnnotation] == drainMarker(op)
}
