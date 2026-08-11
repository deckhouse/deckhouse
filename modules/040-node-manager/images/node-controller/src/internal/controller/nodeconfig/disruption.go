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

package nodeconfig

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// reconcileDisruption answers a node that cannot apply its config without a
// restart, by creating a NodeOperation — the same resource an operator uses.
// The operation names the config revision, so it authorises one change only.
func (r *Reconciler) reconcileDisruption(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, nc *internalv1alpha1.NodeConfig, logger logr.Logger) error {
	if !disruptionRequested(nc) {
		return nil
	}

	existing, err := r.findApproval(ctx, nc)
	if err != nil {
		return err
	}
	if existing != nil {
		// An in-flight operation needs nothing. A completed one means the node is
		// asking again for a revision already carried out — it will be refused
		// forever, so surface it instead of silently holding the rollout slot.
		if existing.Status.Phase == v1alpha1.NodeOperationCompleted {
			logger.V(1).Info("node is asking again for a disruption already carried out",
				"node", node.Name, "nodeGroup", ng.Name, "configGeneration", nc.Generation, "operation", existing.Name)
			r.Recorder.Event(ng, corev1.EventTypeWarning, "DisruptionAlreadyDone",
				fmt.Sprintf("Node %s is still asking to be interrupted for config generation %d, which NodeOperation %s already completed",
					node.Name, nc.Generation, existing.Name))
		}
		return nil
	}

	if approvalMode(ng) == v1.DisruptionApprovalModeManual {
		logger.Info("node needs a disruption an operator has to approve",
			"node", node.Name, "nodeGroup", ng.Name, "configGeneration", nc.Generation)
		r.Recorder.Event(ng, corev1.EventTypeNormal, "DisruptionRequired",
			fmt.Sprintf("Node %s is waiting for a NodeOperation of type ApproveDisruption for config generation %d",
				node.Name, nc.Generation))
		return nil
	}

	return r.createApproval(ctx, ng, node, nc, logger)
}

// findApproval looks for the operation that already covers this revision. The
// read goes straight to the API server: a cached list that missed the previous
// approval would mint a second one for the same revision.
func (r *Reconciler) findApproval(ctx context.Context, nc *internalv1alpha1.NodeConfig) (*v1alpha1.NodeOperation, error) {
	ops := &v1alpha1.NodeOperationList{}
	if err := r.sources.Reader.List(ctx, ops, client.MatchingLabels{v1alpha1.NodeOperationNodeLabel: nc.Name}); err != nil {
		return nil, fmt.Errorf("list NodeOperations of %s: %w", nc.Name, err)
	}
	for i := range ops.Items {
		op := &ops.Items[i]
		if op.Spec.Type != v1alpha1.NodeOperationApproveDisruption || op.Spec.NodeName != nc.Name {
			continue
		}
		if op.Spec.ConfigGeneration == nil || *op.Spec.ConfigGeneration != nc.Generation {
			continue
		}
		// A failed operation allows a fresh attempt. A completed one is returned:
		// it stops a second approval, and a second drain, while the node is still
		// applying the config and has not cleared DisruptionRequired.
		if op.Status.Phase == v1alpha1.NodeOperationFailed {
			continue
		}
		return op, nil
	}
	return nil, nil
}

func (r *Reconciler) createApproval(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, nc *internalv1alpha1.NodeConfig, logger logr.Logger) error {
	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			// Generated rather than derived from the generation: a retry after
			// a failed attempt needs a name of its own, and the history of what
			// was tried is worth keeping.
			GenerateName: fmt.Sprintf("approve-%s-", nc.Name),
			Labels: map[string]string{
				nodecommon.NodeGroupLabel:       ng.Name,
				managedByLabel:                  managedByValue,
				v1alpha1.NodeOperationNodeLabel: nc.Name,
			},
			// Owned by the node: when the node goes, so does the record of what
			// was done to it, instead of accumulating forever.
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Node",
				Name:       node.Name,
				UID:        node.UID,
			}},
		},
		Spec: v1alpha1.NodeOperationSpec{
			Type:             v1alpha1.NodeOperationApproveDisruption,
			NodeName:         nc.Name,
			ConfigGeneration: ptr.To(nc.Generation),
			Drain:            &v1alpha1.NodeOperationDrainSpec{Skip: !needDrain(ng)},
		},
	}
	// No IsAlreadyExists branch: the name is generated, and the API server
	// retries the generation itself until it lands on a free one.
	if err := r.Client.Create(ctx, op); err != nil {
		return fmt.Errorf("ask for a disruption of %s: %w", nc.Name, err)
	}
	logger.Info("asked to interrupt the node for its new config",
		"node", nc.Name, "nodeGroup", ng.Name, "configGeneration", nc.Generation, "operation", op.Name)
	r.Recorder.Event(ng, corev1.EventTypeNormal, "DisruptionRequested",
		fmt.Sprintf("Created NodeOperation %s to interrupt node %s", op.Name, nc.Name))
	return nil
}

// disruptionRequested reports whether the agent is waiting for permission to
// interrupt the node, for the config revision it currently has.
func disruptionRequested(nc *internalv1alpha1.NodeConfig) bool {
	cond := meta.FindStatusCondition(nc.Status.Conditions, disruptionRequiredCondition)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		return false
	}
	return cond.ObservedGeneration == nc.Generation
}

func approvalMode(ng *v1.NodeGroup) v1.DisruptionApprovalMode {
	if ng.Spec.Disruptions == nil || ng.Spec.Disruptions.ApprovalMode == "" {
		return v1.DisruptionApprovalModeAutomatic
	}
	return ng.Spec.Disruptions.ApprovalMode
}

// needDrain mirrors the update-approval rule: a group of exactly one is
// interrupted without a drain. Not "one or fewer" — status.nodes is 0 until its
// controller runs, and reading that as "one" skipped drains on whole groups.
func needDrain(ng *v1.NodeGroup) bool {
	if ng.Status.Nodes == 1 {
		return false
	}
	if ng.Spec.Disruptions == nil || ng.Spec.Disruptions.Automatic == nil {
		return true
	}
	if ng.Spec.Disruptions.Automatic.DrainBeforeApproval == nil {
		return true
	}
	return *ng.Spec.Disruptions.Automatic.DrainBeforeApproval
}
