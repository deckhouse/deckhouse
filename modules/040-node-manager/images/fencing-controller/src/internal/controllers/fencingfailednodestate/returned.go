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

package fencingfailednodestate

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
)

// releaseReturnedNode removes the out-of-service taint and deletes the object
// when the node came back after a reboot. node is the Node that the reconcile
// read, or nil when the Node is gone. It returns true when it deleted the
// object.
func (r *Reconciler) releaseReturnedNode(ctx context.Context, incident *v1alpha1.FencingFailedNodeState, node *corev1.Node) (bool, error) {
	// A Node that is gone did not come back.
	if node == nil || !returned(incident, node) {
		return false, nil
	}

	removed, err := r.removeTaint(ctx, node)
	if err != nil {
		return false, err
	}

	if removed {
		r.announceTaintRemoved(incident)
	}

	// The UID keeps a new object for the same Node.
	uid := incident.UID

	err = r.client.Delete(ctx, incident, client.Preconditions{UID: &uid})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("delete fencingfailednodestate %q: %w", incident.Name, err)
	}

	logf.FromContext(ctx).Info("node came back after a reboot, fencingfailednodestate is deleted",
		append([]any{
			"taint_removed", removed,
			"boot_id", incident.Spec.BootID,
			"node_boot_id", node.Status.NodeInfo.BootID,
			"node_ready", nodeReady(node),
		}, observedFields(incident)...)...)

	r.dropTraceOf(incident.Name)
	clearPhase(incident.Name)

	return true, nil
}

// returned tells if the node came back after a reboot: the Node is Ready, and
// it has a boot ID that is not the boot ID in the object.
func returned(incident *v1alpha1.FencingFailedNodeState, node *corev1.Node) bool {
	bootID := node.Status.NodeInfo.BootID

	return bootID != "" && bootID != incident.Spec.BootID && nodeReady(node)
}

// removeTaint removes the out-of-service taint from the Node that the
// reconcile read. After a conflict it reads the Node again. It returns true
// when it removed the taint, and false when the Node has no such taint.
func (r *Reconciler) removeTaint(ctx context.Context, read *corev1.Node) (bool, error) {
	removed := false
	name := read.Name

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node := read
		read = nil

		if node == nil {
			var err error
			if node, err = r.getNode(ctx, name); err != nil {
				return fmt.Errorf("get node %q: %w", name, err)
			}
		}

		untainted := node.DeepCopy()
		untainted.Spec.Taints = untainted.Spec.Taints[:0]

		for _, taint := range node.Spec.Taints {
			if !taint.MatchTaint(&outOfServiceTaint) {
				untainted.Spec.Taints = append(untainted.Spec.Taints, taint)
			}
		}

		if len(untainted.Spec.Taints) == len(node.Spec.Taints) {
			return nil
		}

		// The version check keeps the taints that other writers change at the
		// same time.
		patch := client.MergeFromWithOptions(node, client.MergeFromWithOptimisticLock{})
		if err := r.client.Patch(ctx, untainted, patch); err != nil {
			return fmt.Errorf("remove out-of-service taint from node %q: %w", name, err)
		}

		removed = true

		logf.FromContext(ctx).Info("out-of-service taint is removed", "node", name)

		return nil
	})

	return removed, err
}
