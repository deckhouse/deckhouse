/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8type "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"

	eeCrd "github.com/deckhouse/deckhouse/egress-gateway-agent/pkg/apis/v1alpha1"
)

const (
	cniNamespace          = "d8-cni-cilium"
	memberNodeLabelKey    = "egress-gateway.network.deckhouse.io/member"
	activeNodeLabelPrefix = "egress-gateway.network.deckhouse.io/active-for-"
)

var deleteFinalizersPatch = map[string]interface{}{
	"metadata": map[string]interface{}{
		"finalizers": nil,
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/cni-cilium/egress-discovery",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "egressgateways",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "EgressGateway",
			FilterFunc: applyEgressGatewayFilter,
		},
		{
			Name:       "egressgatewayinstances",
			ApiVersion: "network.deckhouse.io/v1alpha1",
			Kind:       "EgressGatewayInstance",
			FilterFunc: applyEgressGatewayInstanceFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodeFilter,
		},
		{
			Name:       "cilium_agent_pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{cniNamespace},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":    "agent",
					"module": "cni-cilium",
				},
			},
			FilterFunc: applyCiliumAgentPodFilter,
		},
		{
			Name:       "egress_agent_pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{cniNamespace},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "egress-gateway-agent",
				},
			},
			FilterFunc: applyEgressAgentPodFilter,
		},
	},
}, handleEgressGateways)

func applyEgressGatewayFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var eg eeCrd.EgressGateway

	err := sdk.FromUnstructured(obj, &eg)
	if err != nil {
		return nil, err
	}

	return EgressGatewayInfo{
		Generation:   eg.GetGeneration(),
		Name:         eg.Name,
		UID:          eg.GetUID(),
		NodeSelector: eg.Spec.NodeSelector,
		SourceIP: EgressGatewaySourceIP{
			Mode: string(eg.Spec.SourceIP.Mode),
			VirtualIPAddress: VirtualIPAddress{
				IP:               eg.Spec.SourceIP.VirtualIPAddress.IP,
				RoutingTableName: eg.Spec.SourceIP.VirtualIPAddress.RoutingTableName,
			},
			PrimaryIPFromEgressGatewayNodeInterface: PrimaryIPFromEgressGatewayNodeInterface{
				InterfaceName: eg.Spec.SourceIP.PrimaryIPFromEgressGatewayNodeInterface.InterfaceName,
			},
		},
	}, nil
}

func applyEgressGatewayInstanceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var egi eeCrd.EgressGatewayInstance

	err := sdk.FromUnstructured(obj, &egi)
	if err != nil {
		return nil, err
	}

	return EgressGatewayInstanceInfo{
		Name:            egi.GetName(),
		NodeName:        egi.Spec.NodeName,
		IsDeleted:       !egi.GetDeletionTimestamp().IsZero(),
		OwnerReferences: egi.GetOwnerReferences(),
		Conditions:      egi.Status.Conditions,
	}, nil
}

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	var isMemberLabeled bool
	activeForEGs := make([]string, 0, 4)
	for key := range node.Labels {
		if key == memberNodeLabelKey { // is node labeled as member
			isMemberLabeled = true
		}

		if strings.HasPrefix(key, activeNodeLabelPrefix) { // is node lablled as active of EG
			activeForEGs = append(activeForEGs, strings.TrimPrefix(key, activeNodeLabelPrefix))
		}
	}

	var isReady = true
	for _, cond := range node.Status.Conditions {
		if cond.Type == v1.NodeReady && cond.Status != v1.ConditionTrue {
			isReady = false
			break
		}
	}

	return NodeInfo{
		Name:            node.Name,
		Labels:          node.Labels,
		IsMemberLabeled: isMemberLabeled,
		ActiveForEGs:    activeForEGs,
		IsReady:         isReady,
		IsCordoned:      node.Spec.Unschedulable,
	}, nil
}

func applyCiliumAgentPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &v1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert pod to struct: %v", err)
	}

	var isReady = true
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status != v1.ConditionTrue {
			isReady = false
			break
		}
	}

	return PodInfo{
		Name:      pod.Name,
		Node:      pod.Spec.NodeName,
		IsReady:   isReady,
		IsDeleted: pod.DeletionTimestamp != nil,
	}, nil
}

func applyEgressAgentPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &v1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert pod to struct: %v", err)
	}

	var isReady = true
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status != v1.ConditionTrue {
			isReady = false
			break
		}
	}

	return PodInfo{
		Name:      pod.Name,
		Node:      pod.Spec.NodeName,
		IsReady:   isReady,
		IsDeleted: pod.DeletionTimestamp != nil,
	}, nil
}

func handleEgressGateways(input *go_hook.HookInput) error {
	EgressGatewayStates := egressGatewayStatesFromSnapshots(input.Snapshots["egressgateways"])
	nodesToLabel := make(map[string][]string)
	nodesToUnlabel := make(map[string][]string)

	for _, nodeSnapshot := range input.Snapshots["nodes"] {
		node := nodeSnapshot.(NodeInfo)
		// Node is NotReady or cordoned
		if !node.IsReady || node.IsCordoned {
			if node.IsMemberLabeled {
				nodesToUnlabel[node.Name] = appendToSliceUniqString(nodesToUnlabel[node.Name], memberNodeLabelKey)
			}
			for _, activeLabel := range node.ActiveForEGs {
				nodesToUnlabel[node.Name] = appendToSliceUniqString(nodesToUnlabel[node.Name], activeNodeLabelPrefix+activeLabel)
			}
			continue
		}

		var nodeMatchedAnyEG bool

		for egName, egState := range EgressGatewayStates {
			// node's labels match with egress gateway nodeSelector
			if nodeMatchesNodeSelector(node.Labels, egState.NodeSelector) {
				nodeMatchedAnyEG = true
				egState.AllNodes = appendToSliceUniqString(egState.AllNodes, node.Name)

				if !node.IsMemberLabeled {
					nodesToLabel[node.Name] = appendToSliceUniqString(nodesToLabel[node.Name], memberNodeLabelKey)
				}

				if slices.Contains(node.ActiveForEGs, egName) {
					egState.CurrentActiveNodes = appendToSliceUniqString(egState.CurrentActiveNodes, node.Name)
				}
				EgressGatewayStates[egName] = egState
			}
		}

		// Node doesn't match any EG NodeSelector
		if !nodeMatchedAnyEG {
			if node.IsMemberLabeled {
				nodesToUnlabel[node.Name] = appendToSliceUniqString(nodesToUnlabel[node.Name], memberNodeLabelKey)
			}
			for _, egName := range node.ActiveForEGs {
				nodesToUnlabel[node.Name] = appendToSliceUniqString(nodesToUnlabel[node.Name], activeNodeLabelPrefix+egName)
			}
		}
	}

	// Collect info about nodes with healthy cilium agent pod
	for _, podSnapshot := range input.Snapshots["cilium_agent_pods"] {
		pod := podSnapshot.(PodInfo)
		// Pod is NotReady or deleted
		if !pod.IsReady || pod.IsDeleted {
			continue
		}

		// Check if pod's Node is in AllNodes of any EG
		for egName, egState := range EgressGatewayStates {
			for _, oneOfAllNodes := range egState.AllNodes {
				if pod.Node == oneOfAllNodes {
					egState.HealthyNodes = appendToSliceUniqString(egState.HealthyNodes, pod.Node)
					EgressGatewayStates[egName] = egState
				}
			}
		}
	}

	// Collect info about nodes with healthy egress gateway agent pod
	for _, podSnapshot := range input.Snapshots["egress_agent_pods"] {
		pod := podSnapshot.(PodInfo)
		// Pod is NotReady or deleted
		if !pod.IsReady || pod.IsDeleted {
			continue
		}

		// Check if pod's Node is in AllNodes of any EG and in HealthyNode
		for egName, egState := range EgressGatewayStates {
			for _, oneOfHealthyNodes := range egState.HealthyNodes {
				if pod.Node == oneOfHealthyNodes {
					egState.HealthyNodesWithEgressAgent = appendToSliceUniqString(egState.HealthyNodesWithEgressAgent, pod.Node)
					EgressGatewayStates[egName] = egState
				}
			}
		}
	}

	// Evaluate desired nodes
	egressInternalMap := egressGatewayMapFromSnapshots(input.Snapshots["egressgateways"])
	for egName, egState := range EgressGatewayStates {
		if egState.Mode == string(eeCrd.VirtualIPAddress) {
			// for VirtualIP ready node is one with cilium agent and egress gateway agent
			egState.ReadyNodes = egState.HealthyNodesWithEgressAgent
		} else {
			// otherwise ready node is one with cilium agent
			egState.ReadyNodes = egState.HealthyNodes
		}

		egState.DesiredActiveNode = egState.electDesiredActiveNode()
		var isNodeFoundInCurrentActiveNodes bool
		for _, currentActiveNode := range egState.CurrentActiveNodes {
			if currentActiveNode == egState.DesiredActiveNode {
				isNodeFoundInCurrentActiveNodes = true
			} else {
				nodesToUnlabel[currentActiveNode] = appendToSliceUniqString(nodesToUnlabel[currentActiveNode], activeNodeLabelPrefix+egName)
			}
		}
		if !isNodeFoundInCurrentActiveNodes && egState.DesiredActiveNode != "" {
			nodesToLabel[egState.DesiredActiveNode] = appendToSliceUniqString(nodesToLabel[egState.DesiredActiveNode], activeNodeLabelPrefix+egName)
		}

		eg := egressInternalMap[egName]
		eg.DesiredNode = egState.DesiredActiveNode
		eg.InstanceName = egName + "-" + generateShortHash(egName+"#"+eg.DesiredNode)
		egressInternalMap[egName] = eg

		egressGatewayInstanceInfos := make([]EgressGatewayInstanceInfo, len(input.Snapshots["egressgatewayinstances"]))
		for i, egiSnapshot := range input.Snapshots["egressgatewayinstances"] {
			egressGatewayInstanceInfos[i] = egiSnapshot.(EgressGatewayInstanceInfo)
		}

		patchStatus := makeEGStatusPatchForState(egState, egressGatewayInstanceInfos)
		input.PatchCollector.MergePatch(
			patchStatus,
			"network.deckhouse.io/v1alpha1",
			"EgressGateway",
			"",
			egName,
			object_patch.WithSubresource("/status"))

		EgressGatewayStates[egName] = egState
	}

	input.Values.Set(
		"cniCilium.internal.egressGatewaysMap",
		egressInternalMap,
	)

	processAddingLabels(input, nodesToLabel)
	processRemovingLabels(input, nodesToUnlabel)

	// clean finalizer for orphaned EGI
	egNodes := loadAllNodesFromEgressGatewayStates(EgressGatewayStates)
	for _, egiSnapshot := range input.Snapshots["egressgatewayinstances"] {
		egi := egiSnapshot.(EgressGatewayInstanceInfo)
		// Check if pod's Node is in AllNodes of any EG
		if !egi.IsDeleted {
			continue
		}

		if slices.Contains(egNodes, egi.NodeName) {
			continue
		}

		input.PatchCollector.MergePatch(deleteFinalizersPatch,
			"network.deckhouse.io/v1alpha1",
			"EgressGatewayInstance",
			"",
			egi.Name,
			object_patch.IgnoreMissingObject(),
		)
	}

	return nil
}

func loadAllNodesFromEgressGatewayStates(egs map[string]egressGatewayState) []string {
	allNodes := make([]string, 0, 8)
	for _, eg := range egs {
		allNodes = append(allNodes, eg.AllNodes...)
	}
	return allNodes
}

func makeEGStatusPatchForState(egState egressGatewayState, egInstances []EgressGatewayInstanceInfo) map[string]interface{} {
	ownedInstances := Filter(egInstances, func(instance EgressGatewayInstanceInfo) bool {
		return len(instance.OwnerReferences) > 0 &&
			instance.OwnerReferences[0].UID == egState.UID
	})

	readyOwnedInstances := Filter(ownedInstances, func(instance EgressGatewayInstanceInfo) bool {
		for _, cond := range instance.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				return true
			}
		}
		return false
	})

	var c ConditionTypeChecker

	if egState.Mode == "VirtualIPAddress" {
		c = *conditionlTypeCheckerWithDefaults(metav1.ConditionTrue, "ElectionSucceedAndVirtualIPAnnounced", fmt.Sprintf("Node %s was elected as active node and VirtualIP is announced", egState.DesiredActiveNode)).
			WithDesiredNodeCheck(egState).
			WithReadyNodesCountCheck(len(readyOwnedInstances))
	} else if egState.Mode == "PrimaryIPFromEgressGatewayNodeInterface" {
		c = *conditionlTypeCheckerWithDefaults(metav1.ConditionTrue, "ElectionSucceed", "Node was elected as active node").
			WithDesiredNodeCheck(egState)
	}

	cond := eeCrd.ExtendedCondition{
		Condition: metav1.Condition{
			Type:               "Ready",
			LastTransitionTime: metav1.Time{Time: time.Now()},
			Reason:             c.Reason,
			Message:            c.Message,
			Status:             c.Status,
		},
		LastHeartbeatTime: metav1.Time{Time: time.Now()},
	}

	patch := map[string]interface{}{
		"status": map[string]interface{}{
			"readyNodes":         len(egState.ReadyNodes),
			"observedGeneration": egState.Generation,
			"activeNodeName":     egState.DesiredActiveNode,
			"conditions":         []eeCrd.ExtendedCondition{cond},
		},
	}
	return patch
}

func processRemovingLabels(input *go_hook.HookInput, nodeToLabel map[string][]string) {
	for keyName, labels := range nodeToLabel {
		input.PatchCollector.Filter(removeLabels(labels), "v1", "Node", "", keyName)
	}
}

func removeLabels(labels []string) func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var node *v1.Node

		err := sdk.FromUnstructured(obj, &node)
		if err != nil {
			return nil, err
		}
		nodeLabels := node.GetLabels()

		for _, label := range labels {
			delete(nodeLabels, label)
		}

		node.Labels = nodeLabels
		return sdk.ToUnstructured(node)
	}
}

func processAddingLabels(input *go_hook.HookInput, nodeToLabel map[string][]string) {
	for keyName, labels := range nodeToLabel {
		input.PatchCollector.Filter(appendLabels(labels), "v1", "Node", "", keyName)
	}
}

func appendLabels(labels []string) func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var node *v1.Node

		err := sdk.FromUnstructured(obj, &node)
		if err != nil {
			return nil, err
		}
		nodeLabels := node.GetLabels()

		for _, label := range labels {
			nodeLabels[label] = ""
		}

		node.Labels = nodeLabels
		return sdk.ToUnstructured(node)
	}
}

func egressGatewayStatesFromSnapshots(snapshots []go_hook.FilterResult) map[string]egressGatewayState {
	// map[<eg name>]egressGatewayState
	result := make(map[string]egressGatewayState)
	for _, snapshot := range snapshots {
		eg := snapshot.(EgressGatewayInfo)
		result[eg.Name] = egressGatewayState{
			Generation:                  eg.Generation,
			Name:                        eg.Name,
			Mode:                        eg.SourceIP.Mode,
			IP:                          eg.SourceIP.VirtualIPAddress.IP,
			RoutingTableName:            eg.SourceIP.VirtualIPAddress.RoutingTableName,
			UID:                         eg.UID,
			NodeSelector:                eg.NodeSelector,
			AllNodes:                    make([]string, 0, 4),
			ReadyNodes:                  make([]string, 0, 4),
			HealthyNodes:                make([]string, 0, 4),
			HealthyNodesWithEgressAgent: make([]string, 0, 4),
			CurrentActiveNodes:          make([]string, 0, 4),
		}
	}
	return result
}

func egressGatewayMapFromSnapshots(snapshots []go_hook.FilterResult) map[string]EgressGatewayInfo {
	result := make(map[string]EgressGatewayInfo)
	for _, snapshot := range snapshots {
		eg := snapshot.(EgressGatewayInfo)
		result[eg.Name] = eg
	}
	return result
}

func appendToSliceUniqString(slice []string, value string) []string {
	for _, item := range slice {
		if value == item {
			return slice
		}
	}
	return append(slice, value)
}

func nodeMatchesNodeSelector(nodeLabels, selectorLabels map[string]string) bool {
	for selectorKey, selectorValue := range selectorLabels {
		nodeLabelValue, exists := nodeLabels[selectorKey]
		if !exists {
			return false
		}
		if selectorValue != nodeLabelValue {
			return false
		}
	}
	return true
}

func generateShortHash(input string) string {
	fullHash := fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
	if len(fullHash) > 10 {
		return fullHash[:10]
	}
	return fullHash
}

type EgressGatewayInfo struct {
	UID          k8type.UID            `json:"uid,omitempty" protobuf:"bytes,5,opt,name=uid,casttype=k8s.io/kubernetes/pkg/types.UID"`
	Generation   int64                 `json:"generation"`
	DesiredNode  string                `json:"desiredNode"`
	InstanceName string                `json:"instanceName"`
	Name         string                `json:"name"`
	NodeSelector map[string]string     `json:"nodeSelector"`
	SourceIP     EgressGatewaySourceIP `json:"sourceIP"`
}

type EgressGatewayInstanceInfo struct {
	IsDeleted       bool
	Name            string
	NodeName        string
	OwnerReferences []metav1.OwnerReference
	Conditions      []eeCrd.ExtendedCondition
}

type EgressGatewaySourceIP struct {
	Mode                                    string                                  `json:"mode"`
	VirtualIPAddress                        VirtualIPAddress                        `json:"virtualIPAddress"`
	PrimaryIPFromEgressGatewayNodeInterface PrimaryIPFromEgressGatewayNodeInterface `json:"primaryIPFromEgressGatewayNodeInterface"`
}

type VirtualIPAddress struct {
	IP               string `json:"ip"`
	RoutingTableName string `json:"routingTableName"`
}

type PrimaryIPFromEgressGatewayNodeInterface struct {
	InterfaceName string `json:"interfaceName"`
}

type PodInfo struct {
	Name      string
	Node      string
	IsReady   bool
	IsDeleted bool
}

type NodeInfo struct {
	IsMemberLabeled bool
	IsReady         bool
	IsCordoned      bool
	Name            string
	ActiveForEGs    []string
	Labels          map[string]string
}

type ConditionTypeChecker struct {
	Status  metav1.ConditionStatus
	Reason  string
	Message string
}

func conditionlTypeCheckerWithDefaults(status metav1.ConditionStatus, reason, msg string) *ConditionTypeChecker {
	return &ConditionTypeChecker{
		Status:  status,
		Reason:  reason,
		Message: msg,
	}
}

func (ctc *ConditionTypeChecker) WithDesiredNodeCheck(egState egressGatewayState) *ConditionTypeChecker {
	if egState.DesiredActiveNode == "" {
		ctc.Status = metav1.ConditionFalse
		ctc.Reason = "ElectionFailed"
		ctc.Message = "There aren't ready Nodes"
	}
	return ctc
}

func (ctc *ConditionTypeChecker) WithReadyNodesCountCheck(readyNodesCount int) *ConditionTypeChecker {
	if readyNodesCount == 0 {
		ctc.Status = metav1.ConditionFalse
		ctc.Reason = "VirtualIPAnnouncingFailed"
		ctc.Message = "Can't announce VirtualIP"
	}
	return ctc
}
