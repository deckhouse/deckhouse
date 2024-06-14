/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"context"
	"fmt"
	"sort"
	"strings"
	pkg_cfg "system-registry-manager/pkg/cfg"
	"system-registry-manager/pkg/utils"
	pkg_utils "system-registry-manager/pkg/utils"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	IpForEvenNodesNumber = "192.0.2.1"
)

type CpmFuncNodeClusterStatus = func(*SeaweedfsNodeClusterStatus) bool
type CpmFuncNodeRunningStatus = func(*SeaweedfsNodeRunningStatus) bool

type ClusterMembers struct {
	Members []NodeManager
	Leader  NodeManager
}

type NodeManagerCache struct {
	data map[string]NodeStatus
}

type NodeStatus struct {
	RunningStatus *SeaweedfsNodeRunningStatus
	ClusterStatus *SeaweedfsNodeClusterStatus
	NodeIP        string
}

func NewNodeManagerCache() *NodeManagerCache {
	return &NodeManagerCache{
		data: make(map[string]NodeStatus),
	}
}

func (cache *NodeManagerCache) getNodeStatus(nodeName string) NodeStatus {
	status, ok := cache.data[nodeName]
	if !ok {
		status = NodeStatus{}
		cache.data[nodeName] = status
	}
	return status
}

func (cache *NodeManagerCache) loadNodeStatus(nodeName string, nodeStatus NodeStatus) {
	cache.data[nodeName] = nodeStatus
}

func (cache *NodeManagerCache) GetNodeManagerRunningStatus(nodeManager NodeManager) (*SeaweedfsNodeRunningStatus, error) {
	nodeName := nodeManager.GetNodeName()
	status := cache.getNodeStatus(nodeName)

	if status.RunningStatus != nil {
		return status.RunningStatus, nil
	}
	runningStatus, err := nodeManager.GetNodeRunningStatus()
	if err != nil {
		return nil, err
	}
	status.RunningStatus = runningStatus
	cache.loadNodeStatus(nodeName, status)
	return runningStatus, nil
}

func (cache *NodeManagerCache) GetNodeManagerClusterStatus(nodeManager NodeManager) (*SeaweedfsNodeClusterStatus, error) {
	nodeName := nodeManager.GetNodeName()
	status := cache.getNodeStatus(nodeName)

	if status.ClusterStatus != nil {
		return status.ClusterStatus, nil
	}
	clusterStatus, err := nodeManager.GetNodeClusterStatus()
	if err != nil {
		return nil, err
	}
	status.ClusterStatus = clusterStatus
	cache.loadNodeStatus(nodeName, status)
	return clusterStatus, nil
}

func (cache *NodeManagerCache) GetNodeManagerIP(nodeManager NodeManager) (*string, error) {
	nodeName := nodeManager.GetNodeName()
	status := cache.getNodeStatus(nodeName)

	if status.NodeIP != "" {
		return &status.NodeIP, nil
	}
	nodeIP, err := nodeManager.GetNodeIP()
	if err != nil {
		return nil, err
	}
	status.NodeIP = nodeIP
	cache.loadNodeStatus(nodeName, status)
	return &nodeIP, nil
}

func CmpIsNeedUpdateCerts(nodeRunningStatus *SeaweedfsNodeRunningStatus) bool {
	return nodeRunningStatus.NeedUpdateCerts
}

func CmpIsNotExist(nodeRunningStatus *SeaweedfsNodeRunningStatus) bool {
	return !nodeRunningStatus.IsExist
}

func CmpIsExist(nodeRunningStatus *SeaweedfsNodeRunningStatus) bool {
	return nodeRunningStatus.IsExist
}

func CmpIsNotRunning(nodeRunningStatus *SeaweedfsNodeRunningStatus) bool {
	return !nodeRunningStatus.IsRunning
}

func CmpIsRunning(nodeRunningStatus *SeaweedfsNodeRunningStatus) bool {
	return nodeRunningStatus.IsRunning
}

func CpmIsLeader(nodeClusterStatus *SeaweedfsNodeClusterStatus) bool {
	return nodeClusterStatus.IsLeader
}

func CpmIsNotLeader(nodeClusterStatus *SeaweedfsNodeClusterStatus) bool {
	return !nodeClusterStatus.IsLeader
}

func WaitBy(ctx context.Context, log *logrus.Entry, nodeManagers []NodeManager, cmpFuncs ...interface{}) (bool, error) {
	if len(nodeManagers) == 0 {
		return true, nil
	}

	for i := 0; i < pkg_cfg.MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			log.Infof("wait by retry count %d/%d", i, pkg_cfg.MaxRetries)
			time.Sleep(5 * time.Second)

			isWaited := true
			nodeManagersCache := NewNodeManagerCache()

			for _, nodeManager := range nodeManagers {
				if !isWaited {
					break
				}
				for _, cmpFunc := range cmpFuncs {
					if !isWaited {
						break
					}

					switch f := cmpFunc.(type) {
					case CpmFuncNodeClusterStatus:
						status, err := nodeManagersCache.GetNodeManagerClusterStatus(nodeManager)
						if err != nil || status == nil {
							isWaited = false
							break
						}
						isWaited = isWaited && f(status)
					case CpmFuncNodeRunningStatus:
						status, err := nodeManagersCache.GetNodeManagerRunningStatus(nodeManager)
						if err != nil || status == nil {
							isWaited = false
							break
						}
						isWaited = isWaited && f(status)
					default:
						return false, fmt.Errorf("error, unknown function format: %v", cmpFunc)
					}
				}
			}
			if isWaited {
				return true, nil
			}
		}
	}
	return false, nil
}

func SelectBy(nodeManagers []NodeManager, cmpFuncs ...interface{}) ([]NodeManager, []NodeManager, error) {
	if len(nodeManagers) == 0 {
		return []NodeManager{}, []NodeManager{}, nil
	}

	nodeManagersCache := NewNodeManagerCache()
	selectedNodes := []NodeManager{}
	otherNodes := []NodeManager{}

	for _, nodeManager := range nodeManagers {
		isSelected := true

		for _, cmpFunc := range cmpFuncs {
			switch f := cmpFunc.(type) {
			case CpmFuncNodeClusterStatus:
				status, err := nodeManagersCache.GetNodeManagerClusterStatus(nodeManager)
				if err != nil {
					return nil, nil, err
				}
				if status == nil {
					return nil, nil, fmt.Errorf("empty status")
				}
				isSelected = isSelected && f(status)
			case CpmFuncNodeRunningStatus:
				status, err := nodeManagersCache.GetNodeManagerRunningStatus(nodeManager)
				if err != nil {
					return nil, nil, err
				}
				if status == nil {
					return nil, nil, fmt.Errorf("empty status")
				}
				isSelected = isSelected && f(status)
			default:
				return nil, nil, fmt.Errorf("error, unknown function format: %T", cmpFunc)
			}

			if !isSelected {
				break
			}
		}

		if isSelected {
			selectedNodes = append(selectedNodes, nodeManager)
		} else {
			otherNodes = append(otherNodes, nodeManager)
		}
	}
	return selectedNodes, otherNodes, nil
}

func SortBy(nodeManagers []NodeManager, cmpFuncs ...interface{}) ([]NodeManager, error) {
	if len(nodeManagers) == 0 {
		return nil, nil
	}

	nodeManagersStatusCache := NewNodeManagerCache()

	sortedNodesMap := map[int][]NodeManager{}
	other := []NodeManager{}

	for len(nodeManagers) > 0 {
		var nodeManager NodeManager
		nodeManager, nodeManagers = nodeManagers[0], nodeManagers[1:]

		addedToSorted := false
		for cmpFuncPriority, cmpFunc := range cmpFuncs {
			if addedToSorted {
				break
			}
			switch f := cmpFunc.(type) {
			case CpmFuncNodeClusterStatus:
				status, err := nodeManagersStatusCache.GetNodeManagerClusterStatus(nodeManager)
				if err != nil {
					return nil, err
				}
				if status == nil {
					return nil, fmt.Errorf("Empty status")
				}
				if f(status) {
					addedToSorted = true
					sortedNodesMap[cmpFuncPriority] = append(sortedNodesMap[cmpFuncPriority], nodeManager)
				}
			case CpmFuncNodeRunningStatus:
				status, err := nodeManagersStatusCache.GetNodeManagerRunningStatus(nodeManager)
				if err != nil {
					return nil, err
				}
				if status == nil {
					return nil, fmt.Errorf("Empty status")
				}
				if f(status) {
					addedToSorted = true
					sortedNodesMap[cmpFuncPriority] = append(sortedNodesMap[cmpFuncPriority], nodeManager)
				}
			default:
				return nil, fmt.Errorf("error, unknown func format %v", cmpFunc)
			}
		}
		if !addedToSorted {
			other = append(other, nodeManager)
		}
	}

	sortedNodesKeys := make([]int, 0, len(sortedNodesMap))
	for key := range sortedNodesMap {
		sortedNodesKeys = append(sortedNodesKeys, key)
	}
	sort.Ints(sortedNodesKeys)

	sortedNodes := []NodeManager{}
	for _, key := range sortedNodesKeys {
		sortedNodes = append(sortedNodes, sortedNodesMap[key]...)
	}
	sortedNodes = append(sortedNodes, other...)
	return sortedNodes, nil
}

func GetExpectedNodeCount(nodeCount, expectedNodeCount int) int {
	if nodeCount < expectedNodeCount {
		return nodeCount
	}
	return expectedNodeCount
}

func GetNodesByCount(nodes []NodeManager, count int) ([]NodeManager, []NodeManager) {
	if len(nodes) < count {
		return nodes, []NodeManager{}
	}
	return nodes[:count], nodes[count:]
}

func GetNodeNames(nodes []NodeManager) string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.GetNodeName())
	}
	return fmt.Sprintf("[%s]", strings.Join(names, ","))
}

func DeleteNodes(ctx context.Context, log *logrus.Entry, nodes []NodeManager) error {
	log.Infof("Deleting nodes %s", GetNodeNames(nodes))
	for _, node := range nodes {
		status, err := node.GetNodeRunningStatus()
		if err == nil && !status.IsExist {
			log.Infof("Node %s has already been deleted", node.GetNodeName())
			return nil
		}

		log.Infof("Deleting manifests for node %s", node.GetNodeName())
		if err := node.DeleteNodeManifests(); err != nil {
			return err
		}
	}
	return nil
}

func CreateNodes(ctx context.Context, log *logrus.Entry, nodes []NodeManager, createRequest *SeaweedfsCreateNodeRequest) error {
	for _, node := range nodes {
		log.Infof("Creating manifests for node %s", node.GetNodeName())
		if err := node.CreateNodeManifests(createRequest); err != nil {
			return err
		}
	}
	{
		log.Infof("Waiting nodes: %s", GetNodeNames(nodes))
		wait, err := WaitBy(ctx, log, nodes, CmpIsRunning)
		if err != nil {
			return err
		}
		if !wait {
			return fmt.Errorf("error waitig nodes: %s", GetNodeNames(nodes))
		}
	}
	return nil
}

func RollingUpgradeNodes(ctx context.Context, log *logrus.Entry, nodes []NodeManager, updateRequest *SeaweedfsUpdateNodeRequest) error {
	for _, node := range nodes {
		nodeIP, err := node.GetNodeIP()
		if err != nil {
			return err
		}

		log.Infof("Updating manifests for node %s", node.GetNodeName())
		if err := node.UpdateNodeManifests(updateRequest); err != nil {
			return err
		}

		log.Infof("Waiting node %s", node.GetNodeName())

		haveLeader := false
		var cpmFuncLeaderElection CpmFuncNodeClusterStatus = func(status *SeaweedfsNodeClusterStatus) bool {
			if status.IsLeader {
				haveLeader = true
			}
			return haveLeader
		}

		var cpmFuncNodeConnectToCluster CpmFuncNodeClusterStatus = func(status *SeaweedfsNodeClusterStatus) bool {
			return pkg_utils.IsStringInSlice(nodeIP, &status.ClusterNodesIPs)
		}

		wait, err := WaitBy(ctx, log, []NodeManager{node}, CmpIsRunning, cpmFuncLeaderElection, cpmFuncNodeConnectToCluster)
		if err != nil {
			return err
		}
		if !wait {
			return fmt.Errorf("error waitig node %s", node.GetNodeName())
		}
	}
	return nil
}

func WaitLeaderElectionForNodes(ctx context.Context, log *logrus.Entry, nodes []NodeManager) error {
	log.Infof("Waiting leader election for nodes: %s", GetNodeNames(nodes))
	haveLeader := false
	var cpmFunc CpmFuncNodeClusterStatus = func(status *SeaweedfsNodeClusterStatus) bool {
		if status.IsLeader {
			haveLeader = true
		}
		return haveLeader
	}
	wait, err := WaitBy(ctx, log, nodes, CmpIsRunning, cpmFunc)
	if err != nil {
		return err
	}
	if !wait {
		return fmt.Errorf("error waitig cluster status for nodes: %s", GetNodeNames(nodes))
	}
	return nil
}

func WaitNodesConnection(ctx context.Context, log *logrus.Entry, leader NodeManager, nodesIps []string) error {
	log.Infof("Waiting connection for nodes: [%s]", strings.Join(nodesIps, ","))
	var cpmFunc CpmFuncNodeClusterStatus = func(status *SeaweedfsNodeClusterStatus) bool {
		newIPsInCluster := true
		for _, ip := range nodesIps {
			newIPsInCluster = newIPsInCluster && pkg_utils.IsStringInSlice(ip, &status.ClusterNodesIPs)
		}
		return newIPsInCluster
	}

	wait, err := WaitBy(ctx, log, []NodeManager{leader}, cpmFunc)
	if err != nil {
		return err
	}
	if !wait {
		return fmt.Errorf("error waitig connection for nodes: [%s]", strings.Join(nodesIps, ","))
	}
	return nil
}

func RemoveLeaderStatusForNode(ctx context.Context, log *logrus.Entry, clusterNodes []NodeManager, removeLeaderNode NodeManager) error {
	// Check if leader
	if nodeStatus, err := removeLeaderNode.GetNodeClusterStatus(); err != nil {
		return err
	} else if !nodeStatus.IsLeader {
		return nil
	}

	// Else - change leader
	// Get cluster node for run remote commands
	if len(clusterNodes) == 0 {
		return fmt.Errorf("len(clusterNodes) == 0")
	}
	clusterNode := clusterNodes[0]

	removedLeaderNodeIp, err := removeLeaderNode.GetNodeIP()
	if err != nil {
		return err
	}

	// Remove IpForEvenNodesNumber from cluster
	if err := clusterNode.RemoveNodeFromCluster(IpForEvenNodesNumber); err != nil {
		return err
	}
	if err := WaitLeaderElectionForNodes(ctx, log, clusterNodes); err != nil {
		return err
	}

	// Remove node from cluster
	if err := clusterNode.RemoveNodeFromCluster(removedLeaderNodeIp); err != nil {
		return err
	}
	if err := WaitLeaderElectionForNodes(ctx, log, clusterNodes); err != nil {
		return err
	}

	// Add nodes to cluster
	if utils.IsEvenNumber(len(clusterNodes) + 1) {
		clusterNode.AddNodeToCluster(IpForEvenNodesNumber)
	}
	if err := clusterNode.AddNodeToCluster(removedLeaderNodeIp); err != nil {
		return err
	}
	return nil
}

func GetClustersMembers(nodeManagers []NodeManager) ([]ClusterMembers, error) {
	cache := NewNodeManagerCache()
	visited := make(map[string]bool)
	nodeMap := make(map[string][]string)

	for _, node := range nodeManagers {
		clusterStatus, err := cache.GetNodeManagerClusterStatus(node)
		if err != nil {
			// Рафт не работает, возможно узел не добавлен в кластер, пропустить узел
			continue
		}

		nodeIP, err := cache.GetNodeManagerIP(node)
		if err != nil {
			return nil, err
		}

		nodeMap[*nodeIP] = clusterStatus.ClusterNodesIPs
		for _, ip := range clusterStatus.ClusterNodesIPs {
			nodeMap[ip] = append(nodeMap[ip], *nodeIP)
		}
	}

	var clusters [][]string
	var dfs func(string, []string) []string
	dfs = func(ip string, cluster []string) []string {
		if visited[ip] {
			return cluster
		}
		visited[ip] = true
		cluster = append(cluster, ip)
		for _, neighbor := range nodeMap[ip] {
			cluster = dfs(neighbor, cluster)
		}
		return cluster
	}

	for ip := range nodeMap {
		if !visited[ip] {
			cluster := dfs(ip, []string{})
			clusters = append(clusters, cluster)
		}
	}

	getNodeByIP := func(ip string) (NodeManager, error) {
		for _, node := range nodeManagers {
			nodeIP, err := cache.GetNodeManagerIP(node)
			if err != nil {
				return nil, err
			}
			if ip == *nodeIP {
				return node, nil
			}
		}
		return nil, nil
	}

	clusterMembersList := []ClusterMembers{}

	for _, cluster := range clusters {
		members := make([]NodeManager, 0, len(cluster))
		var leader NodeManager
		leaderFound := false

		for _, ip := range cluster {
			node, err := getNodeByIP(ip)
			if err != nil {
				return nil, err
			}
			if node == nil {
				continue
			}
			members = append(members, node)
			if !leaderFound {
				clusterStatus, err := cache.GetNodeManagerClusterStatus(node)
				if err == nil && clusterStatus != nil && clusterStatus.IsLeader {
					leader = node
					leaderFound = true
				}
			}
		}

		if leaderFound {
			clusterMembersList = append(clusterMembersList, ClusterMembers{
				Members: members,
				Leader:  leader,
			})
		}
	}

	return clusterMembersList, nil
}

func GetCurrentClustersMembers(ctx context.Context, log *logrus.Entry, clusterNodes []NodeManager) ([]ClusterMembers, error) {
	log.Infof("Get clusters members")
	clustersMembers, err := GetClustersMembers(clusterNodes)
	if clustersMembers != nil {
		log.Infof("Clusters count %d", len(clustersMembers))
	}
	return clustersMembers, err
}

func GetClustersLeaders(ctx context.Context, log *logrus.Entry, clusterNodes []NodeManager) ([]NodeManager, error) {
	log.Infof("Get clusters leaders")
	clustersMembers, err := GetCurrentClustersMembers(ctx, log, clusterNodes)
	if err != nil {
		return nil, err
	}
	leaders := []NodeManager{}

	if clustersMembers == nil {
		return leaders, nil
	}

	for _, clusterMembers := range clustersMembers {
		if clusterMembers.Leader != nil {
			leaders = append(leaders, clusterMembers.Leader)
		}
	}
	return leaders, nil
}

func GetNewAndUnusedClusterIP(ctx context.Context, log *logrus.Entry, clusterNodes []NodeManager, removedNodes []NodeManager) ([]NodeManager, []string, []string, error) {
	leaders, err := GetClustersLeaders(ctx, log, clusterNodes)
	if err != nil {
		return nil, nil, nil, err
	}

	if len(leaders) > 1 {
		log.Infof("Have more than one cluster leaders")
		return nil, nil, nil, fmt.Errorf("len(*leaders) > 1")
	}

	ipsForCreate := []string{}
	ipsForDelete := []string{}
	for _, node := range clusterNodes {
		nodeIp, err := node.GetNodeIP()
		if err != nil {
			return nil, nil, nil, err
		}
		ipsForCreate = append(ipsForCreate, nodeIp)
	}
	for _, node := range removedNodes {
		nodeIp, err := node.GetNodeIP()
		if err != nil {
			return nil, nil, nil, err
		}
		ipsForDelete = append(ipsForDelete, nodeIp)
	}

	if len(leaders) < 1 {
		return leaders, ipsForCreate, ipsForDelete, nil
	}

	ipsFromCluster := []string{}
	if leaderInfo, err := leaders[0].GetNodeClusterStatus(); err != nil {
		return leaders, nil, nil, err
	} else {
		ipsFromCluster = leaderInfo.ClusterNodesIPs
	}

	for _, ipFromCluster := range ipsFromCluster {
		if !pkg_utils.IsStringInSlice(ipFromCluster, &ipsForCreate) {
			ipsForDelete = pkg_utils.InsertString(ipFromCluster, ipsForDelete)
		}
	}

	return leaders, ipsForCreate, ipsForDelete, nil
}
