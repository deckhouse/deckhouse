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
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// reconcileDisruption answers the node's request to interrupt itself. An agent
// that cannot apply a config without restarting kubelet, containerd or the
// system extensions says so in its status and waits; this is the cluster side
// of that conversation — the same trade bashible nodes make through the
// disruption-required/-approved annotations on the Node.
//
// The permission is an annotation naming the config revision it covers, so it
// authorises one particular change and not everything that follows. It also
// keeps the spec untouched: writing to the spec would bump the generation the
// permission refers to.
func (r *Reconciler) reconcileDisruption(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, nc *internalv1alpha1.NodeConfig, logger logr.Logger) error {
	if !disruptionRequested(nc) {
		// Nothing pending: give the node back to the scheduler if this
		// controller was the one that took it away.
		return r.finishDisruption(ctx, node, logger)
	}

	if approvalMode(ng) == v1.DisruptionApprovalModeManual {
		logger.Info("node needs a disruption an operator has to approve",
			"node", node.Name, "nodeGroup", ng.Name,
			"annotation", fmt.Sprintf("%s=%d", disruptionApprovedAnnotation, nc.Generation))
		r.Recorder.Event(ng, corev1.EventTypeNormal, "DisruptionRequired",
			"Node "+node.Name+" is waiting for a manual disruption approval")
		return nil
	}

	if r.needDrain(ng) && !drained(node) {
		return r.startDrain(ctx, node, logger)
	}

	return r.approveDisruption(ctx, nc, logger)
}

// startDrain hands the node to the draining controller, which evicts the pods
// and reports back through the drained annotation.
func (r *Reconciler) startDrain(ctx context.Context, node *corev1.Node, logger logr.Logger) error {
	if node.Annotations[nodecommon.DrainingAnnotation] == drainingSource {
		return nil
	}
	patch := client.MergeFrom(node.DeepCopy())
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	node.Annotations[nodecommon.DrainingAnnotation] = drainingSource
	if err := r.Client.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("start drain of %s: %w", node.Name, err)
	}
	logger.Info("draining the node before the disruption it asked for", "node", node.Name)
	return nil
}

func (r *Reconciler) approveDisruption(ctx context.Context, nc *internalv1alpha1.NodeConfig, logger logr.Logger) error {
	revision := strconv.FormatInt(nc.Generation, 10)
	if nc.Annotations[disruptionApprovedAnnotation] == revision {
		return nil
	}
	patch := client.MergeFrom(nc.DeepCopy())
	if nc.Annotations == nil {
		nc.Annotations = map[string]string{}
	}
	nc.Annotations[disruptionApprovedAnnotation] = revision
	if err := r.Client.Patch(ctx, nc, patch); err != nil {
		return fmt.Errorf("approve disruption for %s: %w", nc.Name, err)
	}
	logger.Info("disruption approved", "node", nc.Name, "revision", revision)
	return nil
}

// finishDisruption returns a node this controller drained to the scheduler.
// The draining controller owns the eviction; the annotations it keys on are
// removed here so the node is not left cordoned after the config was applied.
func (r *Reconciler) finishDisruption(ctx context.Context, node *corev1.Node, logger logr.Logger) error {
	if node.Annotations[nodecommon.DrainedAnnotation] != drainingSource &&
		node.Annotations[nodecommon.DrainingAnnotation] != drainingSource {
		return nil
	}
	patch := client.MergeFrom(node.DeepCopy())
	delete(node.Annotations, nodecommon.DrainingAnnotation)
	delete(node.Annotations, nodecommon.DrainedAnnotation)
	node.Spec.Unschedulable = false
	if err := r.Client.Patch(ctx, node, patch); err != nil {
		return fmt.Errorf("finish drain of %s: %w", node.Name, err)
	}
	logger.Info("node returned to the scheduler after its disruption", "node", node.Name)
	return nil
}

// disruptionRequested reports whether the agent is waiting for permission to
// interrupt the node, for the config revision it currently has.
func disruptionRequested(nc *internalv1alpha1.NodeConfig) bool {
	cond := meta.FindStatusCondition(nc.Status.Conditions, disruptionRequiredCondition)
	return cond != nil && cond.Status == metav1.ConditionTrue && cond.ObservedGeneration == nc.Generation
}

func drained(node *corev1.Node) bool {
	return node.Annotations[nodecommon.DrainedAnnotation] == drainingSource
}

func approvalMode(ng *v1.NodeGroup) v1.DisruptionApprovalMode {
	if ng.Spec.Disruptions == nil || ng.Spec.Disruptions.ApprovalMode == "" {
		return v1.DisruptionApprovalModeAutomatic
	}
	return ng.Spec.Disruptions.ApprovalMode
}

// needDrain mirrors the update-approval rule: a group that would lose its only
// node to the drain is interrupted without one.
func (r *Reconciler) needDrain(ng *v1.NodeGroup) bool {
	if ng.Status.Nodes <= 1 {
		return false
	}
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.Automatic != nil &&
		ng.Spec.Disruptions.Automatic.DrainBeforeApproval != nil {
		return *ng.Spec.Disruptions.Automatic.DrainBeforeApproval
	}
	return true
}
