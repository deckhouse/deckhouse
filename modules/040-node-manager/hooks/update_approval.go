/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/shared"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Queue: "/modules/node-manager/update_approval",
	Kubernetes: []go_hook.KubernetesConfig{
		// snapshot: "configuration_checksums_secret"
		// api: "v1",
		// kind: "Secret",
		// ns: "d8-cloud-instance-manager"
		// name: "configuration-checksums"
		shared.ConfigurationChecksumHookConfig(),
		{
			Name:                   "ngs",
			WaitForSynchronization: ptr.To(false),
			ApiVersion:             "deckhouse.io/v1",
			Kind:                   "NodeGroup",
			FilterFunc:             updateApprovalNodeGroupFilter,
		},
		{
			Name:                   "nodes",
			WaitForSynchronization: ptr.To(false),
			ApiVersion:             "v1",
			Kind:                   "Node",
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "node.deckhouse.io/group",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: updateApprovalFilterNode,
		},
	},
}, handleUpdateApproval)

func handleUpdateApproval(ctx context.Context, input *go_hook.HookInput) error {
	approver := &updateApprover{
		finished: false,

		nodes:      make(map[string]updateApprovalNode),
		nodeGroups: make(map[string]updateNodeGroup),
	}

	snaps := input.Snapshots.Get("configuration_checksums_secret")
	if len(snaps) == 0 {
		input.Logger.Warn("no configuration_checksums_secret snapshot found. Skipping run")
		return nil
	}
	err := snaps[0].UnmarshalTo(&approver.ngChecksums)
	if err != nil {
		return fmt.Errorf("failed to unmarshal start 'configuration_checksums_secret' snapshot: %w", err)
	}

	snaps = input.Snapshots.Get("ngs")
	for ng, err := range sdkobjectpatch.SnapshotIter[updateNodeGroup](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'ngs' snapshots: %w", err)
		}

		approver.nodeGroups[ng.Name] = ng
	}

	snaps = input.Snapshots.Get("nodes")
	for node, err := range sdkobjectpatch.SnapshotIter[updateApprovalNode](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes' snapshots: %w", err)
		}

		approver.nodes[node.Name] = node

		setNodeMetric(input, node, approver.nodeGroups[node.NodeGroup], approver.ngChecksums[node.NodeGroup])
	}

	approver.deckhouseNodeName = os.Getenv("DECKHOUSE_NODE_NAME")

	err = approver.processUpdatedNodes(ctx, input)
	if err != nil {
		return err
	}
	if approver.finished {
		return nil
	}

	err = approver.approveDisruptions(ctx, input)
	if err != nil {
		return err
	}
	if approver.finished {
		return nil
	}

	err = approver.approveUpdates(ctx, input)
	if err != nil {
		return err
	}

	return nil
}

type updateApprover struct {
	finished bool

	ngChecksums       shared.ConfigurationChecksum
	nodes             map[string]updateApprovalNode
	nodeGroups        map[string]updateNodeGroup
	deckhouseNodeName string
}

func calculateConcurrency(ngCon *intstr.IntOrString, totalNodes int) int {
	concurrency := 1
	switch ngCon.Type {
	case intstr.Int:
		concurrency = ngCon.IntValue()

	case intstr.String:
		if strings.HasSuffix(ngCon.String(), "%") {
			percentStr := strings.TrimSuffix(ngCon.String(), "%")
			percent, _ := strconv.Atoi(percentStr)
			concurrency = totalNodes * percent / 100
			if concurrency == 0 {
				concurrency = 1
			}
		} else {
			concurrency = ngCon.IntValue()
		}
	}

	return concurrency
}

// Approve updates
//   - Only one node from node group can be approved for update
//   - If there are not ready nodes in the group, they'll be updated first
//
// TODO (core): fix linter
//
//nolint:unparam
func (ar *updateApprover) approveUpdates(_ context.Context, input *go_hook.HookInput) error {
	for _, ng := range ar.nodeGroups {
		nodeGroupNodes := make([]updateApprovalNode, 0)
		currentUpdates := 0

		for _, node := range ar.nodes {
			if node.NodeGroup == ng.Name {
				nodeGroupNodes = append(nodeGroupNodes, node)
			}
		}

		concurrency := calculateConcurrency(ng.Concurrency, len(nodeGroupNodes))

		var hasWaitingForApproval bool

		// Count already approved nodes
		for _, ngn := range nodeGroupNodes {
			if ngn.IsApproved {
				currentUpdates++
			}

			if !hasWaitingForApproval && ngn.IsWaitingForApproval {
				hasWaitingForApproval = true
			}
		}

		// Skip ng, if maxConcurrent is already reached
		if currentUpdates >= concurrency {
			continue
		}
		// Skip ng, if it has no waiting nodes
		if !hasWaitingForApproval {
			continue
		}

		countToApprove := concurrency - currentUpdates
		approvedNodes := make(map[updateApprovalNode]struct{}, countToApprove)

		//     Allow one node, if 100% nodes in NodeGroup are ready
		if ng.Status.Desired <= ng.Status.Ready || ng.NodeType != ngv1.NodeTypeCloudEphemeral {
			allReady := true
			for _, ngn := range nodeGroupNodes {
				if !ngn.IsReady {
					allReady = false
					break
				}
			}

			if allReady {
				for _, ngn := range nodeGroupNodes {
					if ngn.IsWaitingForApproval {
						approvedNodes[ngn] = struct{}{}
						if len(approvedNodes) == countToApprove {
							break
						}
					}
				}
			}
		}

		if len(approvedNodes) < countToApprove {
			//    Allow one of not ready nodes, if any
			for _, ngn := range nodeGroupNodes {
				if !ngn.IsReady && ngn.IsWaitingForApproval {
					approvedNodes[ngn] = struct{}{}
					if len(approvedNodes) == countToApprove {
						break
					}
				}
			}
		}

		if len(approvedNodes) == 0 {
			continue
		}

		for approvedNode := range approvedNodes {
			ar.nodeApproved(input, &approvedNode)
		}
	}
	return nil
}

func (ar *updateApprover) needDrainNode(input *go_hook.HookInput, node *updateApprovalNode, nodeNg *updateNodeGroup) bool {
	// we can not drain single control-plane node because deckhouse webhook will evict
	// and deckhouse will malfunction and drain single node does not matter we always reboot
	// single control plane node without problem
	if nodeNg.Name == "master" && nodeNg.Status.Nodes == 1 {
		input.Logger.Warn("Skip drain single control-plane node")
		return false
	}

	// we can not drain single node with deckhouse
	if node.Name == ar.deckhouseNodeName && nodeNg.Status.Ready < 2 {
		input.Logger.Warn("Skip drain node with deckhouse pod because node-group contains single node and deckhouse will not run after drain", slog.String("node", node.Name), slog.String("node_group", nodeNg.Name))
		return false
	}

	return *nodeNg.Disruptions.Automatic.DrainBeforeApproval
}

// Approve disruption updates for NodeGroups with approvalMode == Automatic
// We don't limit number of Nodes here, because it's already limited
//
// TODO (core): fix linter
//
//nolint:unparam
func (ar *updateApprover) approveDisruptions(_ context.Context, input *go_hook.HookInput) error {
	now := time.Now()

	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		now = time.Date(2021, 01, 01, 13, 30, 00, 00, time.UTC)
	}

	for _, node := range ar.nodes {
		if !node.IsApproved {
			continue
		}
		if node.IsDraining || (!node.IsDisruptionRequired && !node.IsRollingUpdate) || node.IsDisruptionApproved {
			continue
		}

		ngName := node.NodeGroup
		ng := ar.nodeGroups[ngName]

		switch ng.Disruptions.ApprovalMode {
		// Skip nodes in NodeGroup not allowing disruptive updates
		case "Manual":
			continue

		// Skip node if update is not permitted in the current time window
		case "Automatic":
			if !ng.Disruptions.Automatic.Windows.IsAllowed(now) {
				continue
			}

		case "RollingUpdate":
			if !ng.Disruptions.RollingUpdate.Windows.IsAllowed(now) {
				continue
			}
		}

		switch {
		case ng.Disruptions.ApprovalMode == "RollingUpdate":
			// If approvalMode == RollingUpdate simply delete machine
			ar.nodeDeleteRollingUpdate(input, &node)
		case !ar.needDrainNode(input, &node, &ng) || node.IsDrained:
			ar.nodeDisruptionApproved(input, &node)
		case !node.IsUnschedulable:
			ar.nodeDrainingForDisruption(input, &node)
		}
	}

	return nil
}

// Process updated nodes: remove approved and disruption-approved annotations, if:
//   - Node is ready
//   - Node checksum is equal to NodeGroup checksum
//
// TODO (core): fix linter
//
//nolint:unparam
func (ar *updateApprover) processUpdatedNodes(_ context.Context, input *go_hook.HookInput) error {
	for _, node := range ar.nodes {
		if !node.IsApproved {
			continue
		}

		nodeChecksum := node.ConfigurationChecksum
		ngName := node.NodeGroup
		ngChecksum := ar.ngChecksums[ngName]

		if nodeChecksum == "" || ngChecksum == "" {
			continue
		}
		if nodeChecksum != ngChecksum {
			continue
		}
		if !node.IsReady {
			continue
		}

		ar.nodeUpToDate(input, &node)
	}

	return nil
}

func (ar *updateApprover) nodeUpToDate(input *go_hook.HookInput, node *updateApprovalNode) {
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"update.node.deckhouse.io/approved":             nil,
				"update.node.deckhouse.io/waiting-for-approval": nil,
				"update.node.deckhouse.io/disruption-required":  nil,
				"update.node.deckhouse.io/disruption-approved":  nil,
				drainedAnnotationKey:                            nil,
			},
		},
	}
	if node.IsDrained {
		patch["spec"] = map[string]interface{}{
			"unschedulable": nil,
		}
	}
	input.Logger.Info("Node UpToDate", slog.String("node", node.Name), slog.String("ng", node.NodeGroup))
	input.PatchCollector.PatchWithMerge(patch, "v1", "Node", "", node.Name)
	setNodeStatusesMetrics(input, node.Name, node.NodeGroup, "UpToDate")
	ar.finished = true
}

func (ar *updateApprover) nodeDeleteRollingUpdate(input *go_hook.HookInput, node *updateApprovalNode) {
	input.Logger.Info("Delete instances due to RollingUpdate strategy", slog.String("node", node.Name), slog.String("ng", node.NodeGroup))
	input.PatchCollector.DeleteInBackground("deckhouse.io/v1alpha1", "Instance", "", node.Name)
	ar.finished = true
}

func (ar *updateApprover) nodeDisruptionApproved(input *go_hook.HookInput, node *updateApprovalNode) {
	input.Logger.Info("Node DisruptionApproved", slog.String("node", node.Name), slog.String("ng", node.NodeGroup))
	input.PatchCollector.PatchWithMerge(disruptionApprovedPatch, "v1", "Node", "", node.Name)
	setNodeStatusesMetrics(input, node.Name, node.NodeGroup, "DisruptionApproved")
	ar.finished = true
}

func (ar *updateApprover) nodeDrainingForDisruption(input *go_hook.HookInput, node *updateApprovalNode) {
	input.Logger.Info("Node DrainingForDisruption", slog.String("node", node.Name), slog.String("ng", node.NodeGroup))
	input.PatchCollector.PatchWithMerge(drainingPatch, "v1", "Node", "", node.Name)
	setNodeStatusesMetrics(input, node.Name, node.NodeGroup, "DrainingForDisruption")
	ar.finished = true
}

func (ar *updateApprover) nodeApproved(input *go_hook.HookInput, node *updateApprovalNode) {
	input.Logger.Info("Node Approved", slog.String("node", node.Name), slog.String("ng", node.NodeGroup))
	input.PatchCollector.PatchWithMerge(approvedPatch, "v1", "Node", "", node.Name)
	setNodeStatusesMetrics(input, node.Name, node.NodeGroup, "Approved")
	ar.finished = true
}

type updateApprovalNode struct {
	Name      string
	NodeGroup string

	ConfigurationChecksum string

	IsReady              bool
	IsApproved           bool
	IsDisruptionApproved bool
	IsWaitingForApproval bool

	IsDisruptionRequired bool
	IsUnschedulable      bool
	IsDraining           bool
	IsDrained            bool
	IsRollingUpdate      bool
}

type updateNodeGroup struct {
	Name        string
	NodeType    ngv1.NodeType
	Disruptions ngv1.Disruptions
	Status      ngv1.NodeGroupStatus

	Concurrency *intstr.IntOrString
}

var (
	approvedPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"update.node.deckhouse.io/approved":             "",
				"update.node.deckhouse.io/waiting-for-approval": nil,
			},
		},
	}
	disruptionApprovedPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				"update.node.deckhouse.io/disruption-approved": "",
				"update.node.deckhouse.io/disruption-required": nil,
			},
		},
	}
	drainingPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				drainingAnnotationKey: "bashible",
			},
		},
	}
)

func updateApprovalNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	ung := updateNodeGroup{
		Name:     ng.Name,
		NodeType: ng.Spec.NodeType,
	}

	if ng.Spec.Update.MaxConcurrent != nil {
		ung.Concurrency = ng.Spec.Update.MaxConcurrent
	} else {
		concurrency := intstr.FromInt(1)
		ung.Concurrency = &concurrency
	}

	if len(ng.Spec.Disruptions.Automatic.Windows) > 0 {
		ung.Disruptions.Automatic.Windows = ng.Spec.Disruptions.Automatic.Windows
	}

	if len(ng.Spec.Disruptions.RollingUpdate.Windows) > 0 {
		ung.Disruptions.RollingUpdate.Windows = ng.Spec.Disruptions.RollingUpdate.Windows
	}

	if ng.Spec.Disruptions.ApprovalMode != "" {
		ung.Disruptions.ApprovalMode = ng.Spec.Disruptions.ApprovalMode
	} else {
		ung.Disruptions.ApprovalMode = "Automatic"
	}

	ung.Status = ng.Status

	if ung.Disruptions.ApprovalMode == "Automatic" {
		if ng.Spec.Disruptions.Automatic.DrainBeforeApproval != nil {
			ung.Disruptions.Automatic.DrainBeforeApproval = ng.Spec.Disruptions.Automatic.DrainBeforeApproval
		} else {
			ung.Disruptions.Automatic.DrainBeforeApproval = ptr.To(true)
		}
	}

	return ung, nil
}

func updateApprovalFilterNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	var isApproved, isWaitingForApproval, isDisruptionRequired, isDraining, isReady, isDrained, isDisruptionApproved, isRollingUpdate bool

	if _, ok := node.Annotations["update.node.deckhouse.io/approved"]; ok {
		isApproved = true
	}
	if _, ok := node.Annotations["update.node.deckhouse.io/waiting-for-approval"]; ok {
		isWaitingForApproval = true
	}
	if _, ok := node.Annotations["update.node.deckhouse.io/rolling-update"]; ok {
		isRollingUpdate = true
	}
	if _, ok := node.Annotations["update.node.deckhouse.io/disruption-required"]; ok {
		isDisruptionRequired = true
	}
	// This annotation is now only used by bashible, there are other means to drain the node manually.
	if v, ok := node.Annotations[drainingAnnotationKey]; ok && v == "bashible" {
		isDraining = true
	}
	if _, ok := node.Annotations["update.node.deckhouse.io/disruption-approved"]; ok {
		isDisruptionApproved = true
	}
	configChecksum, ok := node.Annotations["node.deckhouse.io/configuration-checksum"]
	if !ok {
		configChecksum = ""
	}
	nodeGroup, ok := node.Labels["node.deckhouse.io/group"]
	if !ok {
		nodeGroup = ""
	}
	// This annotation is now only used by bashible, there are other means to drain the node manually.
	if v, ok := node.Annotations[drainedAnnotationKey]; ok && v == "bashible" {
		isDrained = true
	}

	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}

	n := updateApprovalNode{
		Name:                  node.Name,
		IsApproved:            isApproved,
		IsDisruptionApproved:  isDisruptionApproved,
		ConfigurationChecksum: configChecksum,
		NodeGroup:             nodeGroup,
		IsReady:               isReady,
		IsDisruptionRequired:  isDisruptionRequired,
		IsDraining:            isDraining,
		IsUnschedulable:       node.Spec.Unschedulable,
		IsWaitingForApproval:  isWaitingForApproval,
		IsDrained:             isDrained,
		IsRollingUpdate:       isRollingUpdate,
	}

	return n, nil
}

func setNodeMetric(input *go_hook.HookInput, node updateApprovalNode, ng updateNodeGroup, desiredChecksum string) {
	nodeStatus := calculateNodeStatus(node, ng, desiredChecksum)
	setNodeStatusesMetrics(input, node.Name, node.NodeGroup, nodeStatus)
}

func calculateNodeStatus(node updateApprovalNode, ng updateNodeGroup, desiredChecksum string) string {
	switch {
	case node.IsWaitingForApproval:
		return "WaitingForApproval"

	case node.IsApproved && node.IsDisruptionRequired && node.IsDraining:
		return "DrainingForDisruption"

	case node.IsDraining:
		return "Draining"

	case node.IsDrained:
		return "Drained"

	case node.IsApproved && node.IsDisruptionRequired && ng.Disruptions.ApprovalMode == "Automatic":
		return "WaitingForDisruptionApproval"

	case node.IsApproved && node.IsDisruptionRequired && ng.Disruptions.ApprovalMode == "Manual":
		return "WaitingForManualDisruptionApproval"

	case node.IsApproved && node.IsDisruptionApproved:
		return "DisruptionApproved"

	case node.IsApproved:
		return "Approved"

	case node.ConfigurationChecksum == "":
		return "UpdateFailedNoConfigChecksum"

	case node.ConfigurationChecksum != desiredChecksum:
		return "ToBeUpdated"

	case node.ConfigurationChecksum == desiredChecksum:
		return "UpToDate"

	case node.IsRollingUpdate:
		return "RollingUpdate"

	default:
		return "Unknown"
	}
}

var metricStatuses = []string{
	"WaitingForApproval", "Approved", "DrainingForDisruption", "Draining", "Drained", "WaitingForDisruptionApproval",
	"WaitingForManualDisruptionApproval", "DisruptionApproved", "ToBeUpdated", "UpToDate", "UpdateFailedNoConfigChecksum",
}

func setNodeStatusesMetrics(input *go_hook.HookInput, nodeName, nodeGroup, nodeStatus string) {
	for _, status := range metricStatuses {
		var value float64
		if status == nodeStatus {
			value = 1
		}
		labels := map[string]string{
			"node":       nodeName,
			"node_group": nodeGroup,
			"status":     status,
		}
		input.MetricsCollector.Set("node_group_node_status", value, labels)
	}
}
