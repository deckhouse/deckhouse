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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

// reconcileDisruption answers a node that cannot apply its config without
// restarting kubelet, containerd or the system extensions. The answer is a
// NodeOperation: the node is drained and interrupted through the same resource
// an operator would use to reboot it by hand, so what is being done to a node —
// and who asked for it — is visible in one place instead of an annotation.
//
// The operation names the config revision it covers, so it authorises one
// particular change and not everything that follows.
func (r *Reconciler) reconcileDisruption(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, nc *internalv1alpha1.NodeConfig, logger logr.Logger) error {
	if !disruptionRequested(nc) {
		return nil
	}

	existing, err := r.findApproval(ctx, nc)
	if err != nil {
		return err
	}
	if existing != nil {
		// The operation is already on its way; the nodeoperation controller
		// drains the node and hands it over.
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

	return r.createApproval(ctx, ng, nc, logger)
}

// findApproval looks for the operation that already covers this revision, so a
// node is asked about once rather than on every pass.
func (r *Reconciler) findApproval(ctx context.Context, nc *internalv1alpha1.NodeConfig) (*v1alpha1.NodeOperation, error) {
	ops := &v1alpha1.NodeOperationList{}
	if err := r.Client.List(ctx, ops); err != nil {
		return nil, fmt.Errorf("list NodeOperations: %w", err)
	}
	for i := range ops.Items {
		op := &ops.Items[i]
		if op.Spec.Type != v1alpha1.NodeOperationApproveDisruption ||
			op.Spec.NodeName != nc.Name ||
			op.Spec.ConfigGeneration == nil || *op.Spec.ConfigGeneration != nc.Generation {
			continue
		}
		return op, nil
	}
	return nil, nil
}

func (r *Reconciler) createApproval(ctx context.Context, ng *v1.NodeGroup, nc *internalv1alpha1.NodeConfig, logger logr.Logger) error {
	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("approve-%s-%d", nc.Name, nc.Generation),
			Labels: map[string]string{
				nodeGroupNameLabel: ng.Name,
				managedByLabel:     managedByValue,
			},
		},
		Spec: v1alpha1.NodeOperationSpec{
			Type:             v1alpha1.NodeOperationApproveDisruption,
			NodeName:         nc.Name,
			ConfigGeneration: ptr.To(nc.Generation),
			Drain:            &v1alpha1.NodeOperationDrainSpec{Skip: !needDrain(ng)},
		},
	}
	if err := r.Client.Create(ctx, op); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
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
	return cond != nil && cond.Status == metav1.ConditionTrue && cond.ObservedGeneration == nc.Generation
}

func approvalMode(ng *v1.NodeGroup) v1.DisruptionApprovalMode {
	if ng.Spec.Disruptions == nil || ng.Spec.Disruptions.ApprovalMode == "" {
		return v1.DisruptionApprovalModeAutomatic
	}
	return ng.Spec.Disruptions.ApprovalMode
}

// needDrain mirrors the update-approval rule: a group that would lose its only
// node to the drain is interrupted without one.
func needDrain(ng *v1.NodeGroup) bool {
	if ng.Status.Nodes <= 1 {
		return false
	}
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.Automatic != nil &&
		ng.Spec.Disruptions.Automatic.DrainBeforeApproval != nil {
		return *ng.Spec.Disruptions.Automatic.DrainBeforeApproval
	}
	return true
}
