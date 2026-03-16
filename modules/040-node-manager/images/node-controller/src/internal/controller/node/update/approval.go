/*
Copyright 2025 Flant JSC

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

package update

import (
	"context"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

// nodeInfo holds pre-extracted annotation/label state for a node.
type nodeInfo struct {
	isReady              bool
	isApproved           bool
	isDisruptionApproved bool
	isWaitingForApproval bool
	isDisruptionRequired bool
	isUnschedulable      bool
	isDraining           bool
	isDrained            bool
	isRollingUpdate      bool
	configChecksum       string
}

// extractNodeInfo reads update-relevant annotations and conditions from a node.
func extractNodeInfo(node *corev1.Node) nodeInfo {
	annotations := node.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	info := nodeInfo{
		isUnschedulable: node.Spec.Unschedulable,
		configChecksum:  annotations[annotationConfigChecksum],
	}

	if _, ok := annotations[annotationApproved]; ok {
		info.isApproved = true
	}
	if _, ok := annotations[annotationWaitingForApproval]; ok {
		info.isWaitingForApproval = true
	}
	if _, ok := annotations[annotationDisruptionRequired]; ok {
		info.isDisruptionRequired = true
	}
	if _, ok := annotations[annotationDisruptionApproved]; ok {
		info.isDisruptionApproved = true
	}
	if _, ok := annotations[annotationRollingUpdate]; ok {
		info.isRollingUpdate = true
	}

	// Only consider draining/drained annotations set by bashible source.
	if v, ok := annotations[annotationDraining]; ok && v == drainingSourceBashible {
		info.isDraining = true
	}
	if v, ok := annotations[annotationDrained]; ok && v == drainingSourceBashible {
		info.isDrained = true
	}

	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			info.isReady = true
			break
		}
	}

	return info
}

// approveUpdate decides whether to approve the update for a node that is waiting-for-approval.
//
// Logic (mirrors update_approval.go approveUpdates):
//   - Count how many nodes in the NodeGroup are already approved (currentUpdates).
//   - Calculate concurrency from NodeGroup spec (maxConcurrent).
//   - If currentUpdates >= concurrency, skip.
//   - If all nodes are ready (or NodeType != CloudEphemeral and desired <= ready), approve waiting nodes.
//   - Otherwise prefer to approve not-ready nodes first.
func (r *Reconciler) approveUpdate(ctx context.Context, node *corev1.Node, ng *deckhousev1.NodeGroup) error {
	log := logf.FromContext(ctx)

	nodes, err := r.listNodeGroupNodes(ctx, ng.Name)
	if err != nil {
		return err
	}

	concurrency := calculateConcurrency(ng, len(nodes))
	currentUpdates := 0
	allReady := true

	for i := range nodes {
		ni := extractNodeInfo(&nodes[i])
		if ni.isApproved {
			currentUpdates++
		}
		if !ni.isReady {
			allReady = false
		}
	}

	if currentUpdates >= concurrency {
		log.V(1).Info("concurrency limit reached, skipping approval",
			"node", node.Name, "nodeGroup", ng.Name,
			"currentUpdates", currentUpdates, "concurrency", concurrency)
		return nil
	}

	countToApprove := concurrency - currentUpdates

	// Determine if we can approve ready nodes.
	// For CloudEphemeral: only approve if desired <= ready or all are ready.
	// For other types: always eligible (non-cloud groups do not track desired).
	canApproveReady := allReady
	if ng.Spec.NodeType == deckhousev1.NodeTypeCloudEphemeral {
		canApproveReady = canApproveReady || ng.Status.Desired <= ng.Status.Ready
	} else {
		// Non-cloud types are always eligible for approval.
		canApproveReady = true
	}

	nodeNI := extractNodeInfo(node)

	// Prefer not-ready nodes (they should be updated first).
	if !nodeNI.isReady {
		log.Info("approving not-ready node for update", "node", node.Name, "nodeGroup", ng.Name)
		return r.setNodeApproved(ctx, node)
	}

	// If we can approve ready nodes and have capacity, approve this one.
	if canApproveReady && countToApprove > 0 {
		log.Info("approving node for update", "node", node.Name, "nodeGroup", ng.Name)
		return r.setNodeApproved(ctx, node)
	}

	return nil
}

// approveDisruption handles the disruption approval phase for a node that is already approved
// and has disruption-required or rolling-update annotation set.
//
// Logic (mirrors update_approval.go approveDisruptions):
//   - Check NodeGroup disruption approval mode (Manual/Automatic/RollingUpdate).
//   - For Manual mode: do nothing (user must approve manually).
//   - For Automatic mode: check disruption windows, then drain if needed or approve disruption.
//   - For RollingUpdate mode: check windows, then delete the Instance resource.
func (r *Reconciler) approveDisruption(
	ctx context.Context,
	node *corev1.Node,
	ng *deckhousev1.NodeGroup,
	info *nodeInfo,
) (*ctrl.Result, error) {
	log := logf.FromContext(ctx)

	approvalMode := getDisruptionApprovalMode(ng)

	switch approvalMode {
	case deckhousev1.DisruptionApprovalModeManual:
		// Manual mode: user must set disruption-approved annotation manually.
		log.V(1).Info("disruption approval mode is Manual, skipping", "node", node.Name)
		return nil, nil

	case deckhousev1.DisruptionApprovalModeAutomatic:
		windows := getAutomaticDisruptionWindows(ng)
		if !isDisruptionWindowAllowed(windows, time.Now()) {
			log.V(1).Info("not in disruption window, requeueing", "node", node.Name)
			result := ctrl.Result{RequeueAfter: 1 * time.Minute}
			return &result, nil
		}

		drainBeforeApproval := getDrainBeforeApproval(ng)
		needDrain := drainBeforeApproval && r.needDrainNode(ng)

		switch {
		case !needDrain || info.isDrained:
			// No drain needed or already drained — approve disruption.
			log.Info("approving disruption", "node", node.Name, "nodeGroup", ng.Name)
			if err := r.setDisruptionApproved(ctx, node); err != nil {
				return nil, err
			}
			return &ctrl.Result{}, nil

		case !info.isUnschedulable:
			// Need to drain — set draining annotation to start the drain process.
			log.Info("setting draining annotation for disruption", "node", node.Name, "nodeGroup", ng.Name)
			if err := r.setDrainingForDisruption(ctx, node); err != nil {
				return nil, err
			}
			return &ctrl.Result{}, nil
		}

	case deckhousev1.DisruptionApprovalModeRollingUpdate:
		windows := getRollingUpdateDisruptionWindows(ng)
		if !isDisruptionWindowAllowed(windows, time.Now()) {
			log.V(1).Info("not in rolling update window, requeueing", "node", node.Name)
			result := ctrl.Result{RequeueAfter: 1 * time.Minute}
			return &result, nil
		}

		// RollingUpdate mode: delete the Instance to trigger replacement.
		log.Info("deleting instance for rolling update", "node", node.Name, "nodeGroup", ng.Name)
		if err := r.deleteInstance(ctx, node.Name); err != nil {
			return nil, err
		}
		return &ctrl.Result{}, nil
	}

	return nil, nil
}

// needDrainNode determines if a node should be drained before disruption approval.
// Single control-plane nodes cannot be drained (deckhouse webhook would fail).
func (r *Reconciler) needDrainNode(ng *deckhousev1.NodeGroup) bool {
	// Single master node — skip drain (same as original hook logic).
	if ng.Name == "master" && ng.Status.Nodes == 1 {
		return false
	}

	return true
}

// setNodeApproved sets the approved annotation and removes waiting-for-approval.
func (r *Reconciler) setNodeApproved(ctx context.Context, node *corev1.Node) error {
	patch := client.MergeFrom(node.DeepCopy())

	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[annotationApproved] = ""
	delete(node.Annotations, annotationWaitingForApproval)

	return r.Client.Patch(ctx, node, patch)
}

// setDisruptionApproved sets disruption-approved and removes disruption-required.
func (r *Reconciler) setDisruptionApproved(ctx context.Context, node *corev1.Node) error {
	patch := client.MergeFrom(node.DeepCopy())

	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[annotationDisruptionApproved] = ""
	delete(node.Annotations, annotationDisruptionRequired)

	return r.Client.Patch(ctx, node, patch)
}

// setDrainingForDisruption sets the draining annotation with bashible source.
func (r *Reconciler) setDrainingForDisruption(ctx context.Context, node *corev1.Node) error {
	patch := client.MergeFrom(node.DeepCopy())

	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[annotationDraining] = drainingSourceBashible

	return r.Client.Patch(ctx, node, patch)
}

// deleteInstance deletes the Instance resource to trigger a rolling replacement.
func (r *Reconciler) deleteInstance(ctx context.Context, nodeName string) error {
	instance := &deckhousev1alpha1.Instance{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: nodeName}, instance); err != nil {
		return client.IgnoreNotFound(err)
	}

	return r.Client.Delete(ctx, instance)
}

// calculateConcurrency computes how many nodes can be updated concurrently,
// based on the NodeGroup update.maxConcurrent setting.
func calculateConcurrency(ng *deckhousev1.NodeGroup, totalNodes int) int {
	concurrency := 1

	if ng.Spec.Update == nil || ng.Spec.Update.MaxConcurrent == nil {
		return concurrency
	}

	mc := ng.Spec.Update.MaxConcurrent

	switch mc.Type {
	case intstr.Int:
		concurrency = mc.IntValue()

	case intstr.String:
		s := mc.String()
		if strings.HasSuffix(s, "%") {
			percentStr := strings.TrimSuffix(s, "%")
			percent, _ := strconv.Atoi(percentStr)
			concurrency = totalNodes * percent / 100
			if concurrency == 0 {
				concurrency = 1
			}
		} else {
			concurrency = mc.IntValue()
		}
	}

	if concurrency < 1 {
		concurrency = 1
	}

	return concurrency
}

// getDisruptionApprovalMode returns the effective disruption approval mode for a NodeGroup.
// Defaults to Automatic if not specified.
func getDisruptionApprovalMode(ng *deckhousev1.NodeGroup) deckhousev1.DisruptionApprovalMode {
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.ApprovalMode != "" {
		return ng.Spec.Disruptions.ApprovalMode
	}
	return deckhousev1.DisruptionApprovalModeAutomatic
}

// getDrainBeforeApproval returns whether nodes should be drained before disruption approval.
// Defaults to true if not specified.
func getDrainBeforeApproval(ng *deckhousev1.NodeGroup) bool {
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.Automatic != nil &&
		ng.Spec.Disruptions.Automatic.DrainBeforeApproval != nil {
		return *ng.Spec.Disruptions.Automatic.DrainBeforeApproval
	}
	return true
}

// getAutomaticDisruptionWindows safely returns the automatic disruption windows.
func getAutomaticDisruptionWindows(ng *deckhousev1.NodeGroup) []deckhousev1.DisruptionWindow {
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.Automatic != nil {
		return ng.Spec.Disruptions.Automatic.Windows
	}
	return nil
}

// getRollingUpdateDisruptionWindows safely returns the rolling update disruption windows.
func getRollingUpdateDisruptionWindows(ng *deckhousev1.NodeGroup) []deckhousev1.DisruptionWindow {
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.RollingUpdate != nil {
		return ng.Spec.Disruptions.RollingUpdate.Windows
	}
	return nil
}

// isDisruptionWindowAllowed checks whether the current time falls within any of the
// configured disruption windows. If no windows are configured, disruptions are always allowed.
func isDisruptionWindowAllowed(windows []deckhousev1.DisruptionWindow, now time.Time) bool {
	if len(windows) == 0 {
		return true
	}

	now = now.UTC()

	for _, w := range windows {
		if windowAllowed(w, now) {
			return true
		}
	}

	return false
}

// windowAllowed checks a single disruption window against the current time.
func windowAllowed(w deckhousev1.DisruptionWindow, now time.Time) bool {
	const layout = "15:04"

	fromParsed, err := time.Parse(layout, w.From)
	if err != nil {
		return false
	}
	toParsed, err := time.Parse(layout, w.To)
	if err != nil {
		return false
	}

	fromTime := time.Date(now.Year(), now.Month(), now.Day(), fromParsed.Hour(), fromParsed.Minute(), 0, 0, time.UTC)
	toTime := time.Date(now.Year(), now.Month(), now.Day(), toParsed.Hour(), toParsed.Minute(), 0, 0, time.UTC)

	if !isDayAllowed(w.Days, now) {
		return false
	}

	return now.Equal(fromTime) || now.Equal(toTime) || (now.After(fromTime) && now.Before(toTime))
}

// isDayAllowed checks whether the current weekday is in the allowed days list.
// If no days are specified, all days are allowed.
func isDayAllowed(days []string, now time.Time) bool {
	if len(days) == 0 {
		return true
	}

	weekday := now.Weekday()
	for _, d := range days {
		switch strings.ToLower(d) {
		case "mon":
			if weekday == time.Monday {
				return true
			}
		case "tue":
			if weekday == time.Tuesday {
				return true
			}
		case "wed":
			if weekday == time.Wednesday {
				return true
			}
		case "thu":
			if weekday == time.Thursday {
				return true
			}
		case "fri":
			if weekday == time.Friday {
				return true
			}
		case "sat":
			if weekday == time.Saturday {
				return true
			}
		case "sun":
			if weekday == time.Sunday {
				return true
			}
		}
	}

	return false
}
