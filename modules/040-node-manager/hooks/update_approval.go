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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

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
			WaitForSynchronization: pointer.Bool(false),
			ApiVersion:             "deckhouse.io/v1",
			Kind:                   "NodeGroup",
			FilterFunc:             updateApprovalNodeGroupFilter,
		},
		{
			Name:                   "nodes",
			WaitForSynchronization: pointer.Bool(false),
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

func handleUpdateApproval(input *go_hook.HookInput) error {
	approver := &updateApprover{
		finished: false,

		nodes:      make(map[string]updateApprovalNode),
		nodeGroups: make(map[string]updateNodeGroup),
	}

	snap := input.Snapshots["configuration_checksums_secret"]
	if len(snap) == 0 {
		input.LogEntry.Warn("no configuration_checksums_secret snapshot found. Skipping run")
		return nil
	}
	approver.ngChecksums = snap[0].(shared.ConfigurationChecksum)

	snap = input.Snapshots["ngs"]
	for _, s := range snap {
		ng := s.(updateNodeGroup)
		approver.nodeGroups[ng.Name] = ng
	}

	snap = input.Snapshots["nodes"]
	for _, s := range snap {
		n := s.(updateApprovalNode)
		approver.nodes[n.Name] = n

		setNodeMetric(input, n, approver.nodeGroups[n.NodeGroup], approver.ngChecksums[n.NodeGroup])
	}

	approver.deckhouseNodeName = os.Getenv("DECKHOUSE_NODE_NAME")

	err := approver.processUpdatedNodes(input)
	if err != nil {
		return err
	}
	if approver.finished {
		return nil
	}

	err = approver.approveDisruptions(input)
	if err != nil {
		return err
	}
	if approver.finished {
		return nil
	}

	err = approver.approveUpdates(input)
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
	var concurrency = 1
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
func (ar *updateApprover) approveUpdates(input *go_hook.HookInput) error {
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

		approvedNodeNames := make(map[string]struct{}, countToApprove)

		//     Allow one node, if 100% nodes in NodeGroup are ready
		if ng.Status.Desired == ng.Status.Ready || ng.NodeType != ngv1.NodeTypeCloudEphemeral {
			var allReady = true
			for _, ngn := range nodeGroupNodes {
				if !ngn.IsReady {
					allReady = false
					break
				}
			}

			if allReady {
				for _, ngn := range nodeGroupNodes {
					if ngn.IsWaitingForApproval {
						approvedNodeNames[ngn.Name] = struct{}{}
						if len(approvedNodeNames) == countToApprove {
							break
						}
					}
				}
			}
		}

		if len(approvedNodeNames) < countToApprove {
			//    Allow one of not ready nodes, if any
			for _, ngn := range nodeGroupNodes {
				if !ngn.IsReady && ngn.IsWaitingForApproval {
					approvedNodeNames[ngn.Name] = struct{}{}
					if len(approvedNodeNames) == countToApprove {
						break
					}
				}
			}
		}

		if len(approvedNodeNames) == 0 {
			continue
		}

		for approvedNodeName := range approvedNodeNames {
			input.PatchCollector.MergePatch(approvedPatch, "v1", "Node", "", approvedNodeName)
			setNodeStatusesMetrics(input, approvedNodeName, ng.Name, "Approved")
		}

		ar.finished = true
	}

	return nil
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
)

func (ar *updateApprover) needDrainNode(input *go_hook.HookInput, node *updateApprovalNode, nodeNg *updateNodeGroup) bool {
	// we can not drain single control-plane node because deckhouse webhook will evict
	// and deckhouse will malfunction and drain single node does not matter we always reboot
	// single control plane node without problem
	if nodeNg.Name == "master" && nodeNg.Status.Nodes == 1 {
		input.LogEntry.Warn("Skip drain single control-plane node")
		return false
	}

	// we can not drain single node with deckhouse
	if node.Name == ar.deckhouseNodeName && nodeNg.Status.Ready < 2 {
		input.LogEntry.Warnf("Skip drain node %s with deckhouse pod because node-group %s contains single node and deckhouse will not run after drain", node.Name, nodeNg.Name)
		return false
	}

	return *nodeNg.Disruptions.Automatic.DrainBeforeApproval
}

// Approve disruption updates for NodeGroups with approvalMode == Automatic
// We don't limit number of Nodes here, because it's already limited
func (ar *updateApprover) approveDisruptions(input *go_hook.HookInput) error {
	now := time.Now()

	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		now = time.Date(2021, 01, 01, 13, 30, 00, 00, time.UTC)
	}

	for _, node := range ar.nodes {
		if !((node.IsDisruptionRequired || node.IsRollingUpdate) && !node.IsDraining) {
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

		ar.finished = true

		// If approvalMode == RollingUpdate simply delete machine
		if ng.Disruptions.ApprovalMode == "RollingUpdate" {
			input.LogEntry.Infof("Delete machine d8-cloud-instance-manager/%s due to RollingUpdate strategy", node.Name)
			input.PatchCollector.Delete("machine.sapcloud.io/v1alpha1", "Machine", "d8-cloud-instance-manager", node.Name, object_patch.InBackground())
			continue
		}

		var patch map[string]interface{}
		var metricStatus string

		drainBeforeApproval := ar.needDrainNode(input, &node, &ng)

		switch {
		case !drainBeforeApproval:
			// Skip draining if it's disabled in the NodeGroup
			patch = map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"update.node.deckhouse.io/disruption-approved": "",
						"update.node.deckhouse.io/disruption-required": nil,
					},
				},
			}
			metricStatus = "DisruptionApproved"

		case !node.IsUnschedulable:
			// If node is not unschedulable – mark it for draining
			patch = map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						drainingAnnotationKey: "bashible",
					},
				},
			}
			metricStatus = "DrainingForDisruption"

		default:
			// Node is unschedulable (is drained by us, or was marked as unschedulable by someone before), skip draining
			patch = map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"update.node.deckhouse.io/disruption-approved": "",
						"update.node.deckhouse.io/disruption-required": nil,
					},
				},
			}
			metricStatus = "DisruptionApproved"
		}

		input.PatchCollector.MergePatch(patch, "v1", "Node", "", node.Name)
		setNodeStatusesMetrics(input, node.Name, node.NodeGroup, metricStatus)
	}

	return nil
}

// Process updated nodes: remove approved and disruption-approved annotations, if:
//   - Node is ready
//   - Node checksum is equal to NodeGroup checksum
func (ar *updateApprover) processUpdatedNodes(input *go_hook.HookInput) error {
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
		input.PatchCollector.MergePatch(patch, "v1", "Node", "", node.Name)
		setNodeStatusesMetrics(input, node.Name, node.NodeGroup, "UpToDate")
		ar.finished = true
	}

	return nil
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
			ung.Disruptions.Automatic.DrainBeforeApproval = pointer.Bool(true)
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
	if _, ok := node.Annotations[drainingAnnotationKey]; ok {
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
	if _, ok := node.Annotations[drainedAnnotationKey]; ok {
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
