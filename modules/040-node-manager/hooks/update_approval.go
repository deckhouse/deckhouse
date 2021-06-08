package hooks

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha2"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       3,
	},
	Queue: "/modules/node-manager/update_approval",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "configuration_checksums_secret",
			WaitForSynchronization: &waitForSync,
			ApiVersion:             "v1",
			Kind:                   "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"configuration-checksums"},
			},
			FilterFunc: updateApprovalSecretFilter,
		},
		{
			Name:                   "ngs",
			WaitForSynchronization: &waitForSync,
			ApiVersion:             "deckhouse.io/v1alpha2",
			Kind:                   "NodeGroup",
			FilterFunc:             updateApprovalNodeGroupFilter,
		},
		{
			Name:                   "nodes",
			WaitForSynchronization: &waitForSync,
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
		finished:   false,
		nodes:      make(map[string]updateApprovalNode),
		nodeGroups: make(map[string]updateNodeGroup),
	}

	snap := input.Snapshots["ngs"]
	for _, s := range snap {
		ng := s.(updateNodeGroup)
		approver.nodeGroups[ng.Name] = ng
	}

	snap = input.Snapshots["nodes"]
	for _, s := range snap {
		n := s.(updateApprovalNode)
		approver.nodes[n.Name] = n
	}

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

	nodes      map[string]updateApprovalNode
	nodeGroups map[string]updateNodeGroup
}

// Approve updates
//  * Only one node from node group can be approved for update
//  * If there are not ready nodes in the group, they'll be updated first
func (ar *updateApprover) approveUpdates(input *go_hook.HookInput) error {
ngLoop:
	for _, ng := range ar.nodeGroups {
		nodeGroupNodes := make([]updateApprovalNode, 0)

		for _, node := range ar.nodes {
			if node.NodeGroup == ng.Name {
				nodeGroupNodes = append(nodeGroupNodes, node)
			}
		}
		// Skip ng, if it already has approved nodes
		for _, ngn := range nodeGroupNodes {
			if ngn.IsApproved {
				continue ngLoop
			}
		}

		// Skip ng, if it has no waiting nodes
		var hasWaitingForApproval bool
		for _, nn := range nodeGroupNodes {
			if nn.IsWaitingForApproval {
				hasWaitingForApproval = true
				break
			}
		}

		if !hasWaitingForApproval {
			continue
		}

		approvedNodeName := ""

		//     Allow one node, if 100% nodes in NodeGroup are ready
		if ng.Status.Desired == ng.Status.Ready || ng.NodeType != "Cloud" {
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
						approvedNodeName = ngn.Name
						break
					}
				}
			}
		}

		//    Allow one of not ready nodes, if any
		for _, ngn := range nodeGroupNodes {
			if !ngn.IsReady && ngn.IsWaitingForApproval {
				approvedNodeName = ngn.Name
				break
			}
		}

		if approvedNodeName == "" {
			continue
		}

		err := input.ObjectPatcher.MergePatchObject(approvedPatch, "v1", "Node", "", approvedNodeName, "")
		if err != nil {
			return err
		}
		ar.finished = true
	}

	return nil
}

var (
	approvedPatch, _ = json.Marshal(
		map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"update.node.deckhouse.io/approved":             "",
					"update.node.deckhouse.io/waiting-for-approval": nil,
				},
			},
		},
	)
)

// Approve disruption updates for NodeGroups with approvalMode == Automatic
// We don't limit number of Nodes here, because it's already limited
func (ar *updateApprover) approveDisruptions(input *go_hook.HookInput) error {
	for _, node := range ar.nodes {
		if !(node.IsDisruptionRequired && !node.IsDraining) {
			continue
		}

		ngName := node.NodeGroup

		ng := ar.nodeGroups[ngName]

		// Skip nodes in NodeGroup not allowing disruptive updates
		if !(ng.Disruptions.ApprovalMode == "Automatic") {
			continue
		}

		ar.finished = true

		var patch map[string]interface{}

		switch {
		case !*ng.Disruptions.Automatic.DrainBeforeApproval:
			// Skip draining if it's disabled in the NodeGroup
			patch = map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"update.node.deckhouse.io/disruption-approved": "",
						"update.node.deckhouse.io/disruption-required": nil,
					},
				},
			}

		case !node.IsUnschedulable:
			// If node is not unschedulable – mark it for draining
			patch = map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"update.node.deckhouse.io/draining": "",
					},
				},
			}

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
		}

		patchData, _ := json.Marshal(patch)
		err := input.ObjectPatcher.MergePatchObject(patchData, "v1", "Node", "", node.Name, "")
		if err != nil {
			return err
		}

	}

	return nil
}

// Process updated nodes: remove approved and disruption-approved annotations, if:
//   * Node is ready
//   * Node checksum is equal to NodeGroup checksum
func (ar *updateApprover) processUpdatedNodes(input *go_hook.HookInput) error {
	for _, node := range ar.nodes {
		if !node.IsApproved {
			continue
		}

		nodeChecksum := node.ConfigurationChecksum
		ngName := node.NodeGroup
		snap := input.Snapshots["configuration_checksums_secret"]
		if len(snap) == 0 {
			return fmt.Errorf("no configuration_checksums_secret snapshot found")
		}
		ngChecksum, ok := snap[0].(confChecksumSecret).Data[ngName]
		if !ok {
			ngChecksum = ""
		}

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
					"update.node.deckhouse.io/drained":              nil,
				},
			},
		}
		if node.IsDrained {
			patch["spec"] = map[string]interface{}{
				"unschedulable": nil,
			}
		}
		data, _ := json.Marshal(patch)
		err := input.ObjectPatcher.MergePatchObject(data, "v1", "Node", "", node.Name, "")
		if err != nil {
			return err
		}
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
	IsWaitingForApproval bool

	IsDisruptionRequired bool
	IsUnschedulable      bool
	IsDraining           bool
	IsDrained            bool
}

type updateNodeGroup struct {
	Name        string
	NodeType    string
	Disruptions v1alpha2.Disruptions
	Status      v1alpha2.NodeGroupStatus
}

type confChecksumSecret struct {
	Data map[string]string
}

func updateApprovalSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return nil, err
	}

	data := make(map[string]string, len(sec.Data))
	for k, v := range sec.Data {
		data[k] = string(v)
	}

	return confChecksumSecret{Data: data}, nil
}

func updateApprovalNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng v1alpha2.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	ung := updateNodeGroup{
		Name:     ng.Name,
		NodeType: ng.Spec.NodeType,
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
			ung.Disruptions.Automatic.DrainBeforeApproval = pointer.BoolPtr(true)
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

	var isApproved, isWaitingForApproval, isDisruptionRequired, isDraining, isReady, isDrained bool

	if _, ok := node.Annotations["update.node.deckhouse.io/approved"]; ok {
		isApproved = true
	}
	if _, ok := node.Annotations["update.node.deckhouse.io/waiting-for-approval"]; ok {
		isWaitingForApproval = true
	}
	if _, ok := node.Annotations["update.node.deckhouse.io/disruption-required"]; ok {
		isDisruptionRequired = true
	}
	if _, ok := node.Annotations["update.node.deckhouse.io/draining"]; ok {
		isDraining = true
	}
	configChecksum, ok := node.Annotations["node.deckhouse.io/configuration-checksum"]
	if !ok {
		configChecksum = ""
	}
	nodeGroup, ok := node.Labels["node.deckhouse.io/group"]
	if !ok {
		nodeGroup = ""
	}
	if _, ok := node.Annotations["update.node.deckhouse.io/drained"]; ok {
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
		ConfigurationChecksum: configChecksum,
		NodeGroup:             nodeGroup,
		IsReady:               isReady,
		IsDisruptionRequired:  isDisruptionRequired,
		IsDraining:            isDraining,
		IsUnschedulable:       node.Spec.Unschedulable,
		IsWaitingForApproval:  isWaitingForApproval,
		IsDrained:             isDrained,
	}

	return n, nil
}
