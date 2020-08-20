package converge

import (
	"fmt"

	"github.com/flant/logboek"

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/client"
	"flant/deckhouse-candi/pkg/terraform"
)

const (
	OKStatus      = "ok"
	ChangedStatus = "changed"
	ErrorStatus   = "error"

	InsufficientStatus = "insufficient"
	ExcessiveStatus    = "excessive"
)

func checkClusterState(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) (bool, error) {
	/*
	  labels:
	    status
	*/
	clusterState, err := GetClusterStateFromCluster(kubeCl)
	if err != nil {
		return false, fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err)
	}

	if clusterState == nil {
		return false, fmt.Errorf("kubernetes cluster has no state")
	}

	baseRunner := terraform.NewRunnerFromMetaConfig("base-infrastructure", metaConfig).
		WithVariables(metaConfig.MarshalConfig()).
		WithState(clusterState).
		WithAutoApprove(true)
	defer baseRunner.Close()

	return terraform.CheckPipeline(baseRunner)
}

type ClusterCheckResult struct {
	Status string
}

type NodeCheckResult struct {
	Group  string
	Name   string
	Status string
}

type NodeGroupCheckResult struct {
	Name   string
	Status string
}

type ConvergeStatistics struct {
	Node       []NodeCheckResult
	NodeGroups []NodeGroupCheckResult
	Cluster    ClusterCheckResult
}

func checkNodeState(metaConfig *config.MetaConfig, nodeGroup *ConvergeNodeGroupGroupOptions, nodeName string) (bool, error) {
	state := nodeGroup.State[nodeName]
	index := getIndexFromNodeName(nodeName)
	if index == -1 {
		return false, fmt.Errorf("can't extract index from terraform state secret, skip %s\n", nodeName)
	}

	nodeRunner := terraform.NewRunnerFromMetaConfig(nodeGroup.Step, metaConfig).
		WithVariables(metaConfig.PrepareTerraformNodeGroupConfig(nodeGroup.Name, int(index), nodeGroup.CloudConfig)).
		WithState(state)
	defer nodeRunner.Close()

	return terraform.CheckPipeline(nodeRunner)
}

func CheckState(kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) (*ConvergeStatistics, error) {
	/*
	  labels:
	    node_group
	    node_name
	    status

	  labels:
	    name
	    status
	*/
	statistics := ConvergeStatistics{
		Node:       make([]NodeCheckResult, 0),
		NodeGroups: make([]NodeGroupCheckResult, 0),
		Cluster:    ClusterCheckResult{Status: OKStatus},
	}

	clusterChanged, err := checkClusterState(kubeCl, metaConfig)
	if err != nil {
		statistics.Cluster.Status = ErrorStatus
	} else if clusterChanged {
		statistics.Cluster.Status = ChangedStatus
	}

	nodesState, err := GetNodesStateFromCluster(kubeCl)
	if err != nil {
		return nil, fmt.Errorf("terraform cluster state in Kubernetes cluster not found: %w", err)
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
		if replicas > len(nodeGroupState) {
			nodeGroupCheckResult.Status = InsufficientStatus
		} else if replicas < len(nodeGroupState) {
			nodeGroupCheckResult.Status = ExcessiveStatus
		}

		statistics.NodeGroups = append(statistics.NodeGroups, nodeGroupCheckResult)
		nodeGroup := ConvergeNodeGroupGroupOptions{
			Name:     nodeGroupName,
			Step:     step,
			Replicas: replicas,
			State:    nodeGroupState,
		}

		for name := range nodeGroupState {
			// track changed and ok
			checkResult := NodeCheckResult{Group: nodeGroupName, Name: name, Status: OKStatus}
			changed, err := checkNodeState(metaConfig, &nodeGroup, name)
			if err != nil {
				logboek.LogErrorLn(err)
				checkResult.Status = ErrorStatus
			} else if changed {
				checkResult.Status = ChangedStatus
			}

			statistics.Node = append(statistics.Node, checkResult)
		}
	}
	return &statistics, nil
}
