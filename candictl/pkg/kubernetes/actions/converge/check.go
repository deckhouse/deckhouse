package converge

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"flant/candictl/pkg/config"
	"flant/candictl/pkg/kubernetes/client"
	"flant/candictl/pkg/terraform"
)

const (
	OKStatus      = "ok"
	ChangedStatus = "changed"
	ErrorStatus   = "error"

	InsufficientStatus = "insufficient"
	ExcessiveStatus    = "excessive"
)

type ClusterCheckResult struct {
	Status string `json:"status,omitempty"`
}

type NodeCheckResult struct {
	Group  string `json:"group,omitempty"`
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

type NodeGroupCheckResult struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

type Statistics struct {
	Node       []NodeCheckResult      `json:"nodes,omitempty"`
	NodeGroups []NodeGroupCheckResult `json:"node_groups,omitempty"`
	Cluster    ClusterCheckResult     `json:"cluster,omitempty"`
}

func checkClusterState(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) (bool, error) {
	clusterState, err := GetClusterStateFromCluster(kubeCl)
	if err != nil {
		return false, fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err)
	}

	if clusterState == nil {
		return false, fmt.Errorf("kubernetes cluster has no state")
	}

	baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure").
		WithVariables(metaConfig.MarshalConfig()).
		WithState(clusterState).
		WithAutoApprove(true)
	defer baseRunner.Close()

	return terraform.CheckPipeline(baseRunner, "Kubernetes cluster")
}

func checkNodeState(metaConfig *config.MetaConfig, nodeGroup *NodeGroupGroupOptions, nodeName string) (bool, error) {
	index, ok := getIndexFromNodeName(nodeName)
	if !ok {
		return false, fmt.Errorf("can't extract index from terraform state secret, skip %s", nodeName)
	}

	nodeRunner := terraform.NewRunnerFromConfig(metaConfig, nodeGroup.Step).
		WithVariables(metaConfig.NodeGroupConfig(nodeGroup.Name, int(index), nodeGroup.CloudConfig)).
		WithState(nodeGroup.State[nodeName]).
		WithName(nodeName)
	defer nodeRunner.Close()

	return terraform.CheckPipeline(nodeRunner, nodeName)
}

func CheckState(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) (*Statistics, error) {
	statistics := Statistics{
		Node:       make([]NodeCheckResult, 0),
		NodeGroups: make([]NodeGroupCheckResult, 0),
		Cluster:    ClusterCheckResult{Status: OKStatus},
	}

	var allErrs *multierror.Error

	clusterChanged, err := checkClusterState(kubeCl, metaConfig)
	if err != nil {
		statistics.Cluster.Status = ErrorStatus
		allErrs = multierror.Append(allErrs, err)
	} else if clusterChanged {
		statistics.Cluster.Status = ChangedStatus
	}

	nodesState, err := GetNodesStateFromCluster(kubeCl)
	if err != nil {
		allErrs = multierror.Append(allErrs, fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err))
		return &statistics, allErrs.ErrorOrNil()
	}

	var nodeGroupsWithStateInCluster []string
	for _, group := range metaConfig.GetStaticNodeGroups() {
		// Skip if node group terraform state exists, we will update node group state below
		if _, ok := nodesState[group.Name]; ok {
			nodeGroupsWithStateInCluster = append(nodeGroupsWithStateInCluster, group.Name)
			continue
		}

		// track missed
		statistics.NodeGroups = append(statistics.NodeGroups, NodeGroupCheckResult{Name: group.Name, Status: InsufficientStatus})
	}

	for _, nodeGroupName := range sortNodeGroupsStateKeys(nodesState, nodeGroupsWithStateInCluster) {
		nodeGroupState := nodesState[nodeGroupName]
		replicas := getReplicasByNodeGroupName(metaConfig, nodeGroupName)
		step := GetStepByNodeGroupName(nodeGroupName)

		nodeGroupCheckResult := NodeGroupCheckResult{Name: nodeGroupName, Status: OKStatus}
		if replicas > len(nodeGroupState.State) {
			nodeGroupCheckResult.Status = InsufficientStatus
		} else if replicas < len(nodeGroupState.State) {
			nodeGroupCheckResult.Status = ExcessiveStatus
		}

		statistics.NodeGroups = append(statistics.NodeGroups, nodeGroupCheckResult)
		nodeGroup := NodeGroupGroupOptions{
			Name:     nodeGroupName,
			Step:     step,
			Replicas: replicas,
			State:    nodeGroupState.State,
		}

		for name := range nodeGroupState.State {
			// track changed and ok
			checkResult := NodeCheckResult{Group: nodeGroupName, Name: name, Status: OKStatus}
			changed, err := checkNodeState(metaConfig, &nodeGroup, name)
			if err != nil {
				checkResult.Status = ErrorStatus
				allErrs = multierror.Append(allErrs, fmt.Errorf("node %s: %v", name, err))
			} else if changed {
				checkResult.Status = ChangedStatus
			}

			statistics.Node = append(statistics.Node, checkResult)
		}
	}
	return &statistics, allErrs.ErrorOrNil()
}
