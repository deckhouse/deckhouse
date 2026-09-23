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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// rememberCordon records whether the node was already cordoned for a reason of
// its own, so release does not undo an operator's cordon. A holder's recorded
// answer is copied instead of live state, which may be the holder's own cordon.
func (r *Reconciler) rememberCordon(ctx context.Context, op *v1alpha1.NodeOperation, node *corev1.Node) error {
	if op.Status.NodeWasUnschedulable != nil {
		return nil
	}
	recorded, err := r.cordonRecordedByHolder(ctx, op)
	if err != nil {
		return err
	}
	wasUnschedulable := node.Spec.Unschedulable
	if recorded != nil {
		wasUnschedulable = *recorded
	}

	return r.patchStatus(ctx, op, fmt.Sprintf("record how %s found the node", op.Name), func() {
		op.Status.NodeWasUnschedulable = ptr.To(wasUnschedulable)
	})
}

// cordonRecordedByHolder returns what an operation already holding this node
// recorded; one that has not recorded yet has not cordoned either, so nil. The
// read goes straight to the API server: a stale cache reads as "nobody here".
func (r *Reconciler) cordonRecordedByHolder(ctx context.Context, op *v1alpha1.NodeOperation) (*bool, error) {
	others, err := r.operationsOfNode(ctx, op.Spec.NodeName)
	if err != nil {
		return nil, fmt.Errorf("list the operations of %s: %w", op.Spec.NodeName, err)
	}
	for i := range others {
		other := &others[i]
		if other.UID == op.UID || terminal(other) {
			continue
		}
		if other.Status.NodeWasUnschedulable != nil {
			return other.Status.NodeWasUnschedulable, nil
		}
	}
	return nil, nil
}

// releaseNode puts a node back the way the operation found it. It runs on
// every reconcile of a terminal operation until collection, so it touches only
// this operation's own markers and a cordon no other operation relies on.
func (r *Reconciler) releaseNode(ctx context.Context, op *v1alpha1.NodeOperation, logger logr.Logger) error {
	// Past the cache, like every read this controller decides a write from: the
	// erasure below removes whatever value the key holds now, so a stale marker
	// read here wipes the request a sibling operation has since written.
	node := &corev1.Node{}
	if err := r.apiReader.Get(ctx, types.NamespacedName{Name: op.Spec.NodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	marker := drainMarker(op)
	ours := node.Annotations[nodecommon.DrainingAnnotation] == marker ||
		node.Annotations[nodecommon.DrainedAnnotation] == marker
	if !ours {
		return nil
	}

	// An operation from before this was recorded falls back to the old
	// behaviour: leaving such a node cordoned forever is worse than the cordon
	// this restores, and finished operations are collected within a day.
	restored := op.Status.NodeWasUnschedulable != nil && *op.Status.NodeWasUnschedulable
	if !restored {
		// Lowering it is only ours while nobody else needs it raised: a Drain,
		// or an interruption mid-eviction, needs the node kept out of the
		// scheduler, and releasing would put pods back onto an emptying node.
		held, err := r.heldUnschedulableByAnother(ctx, op)
		if err != nil {
			return err
		}
		if held {
			restored = node.Spec.Unschedulable
		}
	}

	// Spelled out rather than computed from a mutated copy: annotations carry
	// omitempty, so removing the last key makes the merge patch say
	// "annotations: null", deleting every annotation on the node.
	annotations := map[string]any{}
	for _, key := range []string{nodecommon.DrainingAnnotation, nodecommon.DrainedAnnotation} {
		if node.Annotations[key] == marker {
			annotations[key] = nil
		}
	}
	body, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"annotations": annotations},
		"spec":     map[string]any{"unschedulable": restored},
	})
	if err != nil {
		return fmt.Errorf("build the release patch for %s: %w", node.Name, err)
	}
	if err := r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, body)); err != nil {
		return fmt.Errorf("release %s after its operation: %w", node.Name, err)
	}
	logger.Info("node released after its operation",
		"node", node.Name, "operation", op.Name, "unschedulable", restored)
	return nil
}

// heldUnschedulableByAnother reports whether an operation of another lineage
// still needs this node out of the scheduler: a Drain, or an interruption
// mid-eviction. An operation's own child eviction shares its lineage.
func (r *Reconciler) heldUnschedulableByAnother(ctx context.Context, op *v1alpha1.NodeOperation) (bool, error) {
	others, err := r.operationsOfNode(ctx, op.Spec.NodeName)
	if err != nil {
		return false, fmt.Errorf("list the operations of %s: %w", op.Spec.NodeName, err)
	}
	lineage := lineageUID(op)
	for i := range others {
		other := &others[i]
		if lineageUID(other) == lineage || terminal(other) {
			continue
		}
		if other.Spec.Type == v1alpha1.NodeOperationTypeDrain || drainRequested(other) {
			return true, nil
		}
	}
	return false, nil
}
