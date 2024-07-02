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
	Members []RegistryNodeManager
	Leader  RegistryNodeManager
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

func (cache *NodeManagerCache) GetNodeManagerRunningStatus(nodeManager RegistryNodeManager) (*SeaweedfsNodeRunningStatus, error) {
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

func (cache *NodeManagerCache) GetNodeManagerClusterStatus(nodeManager RegistryNodeManager) (*SeaweedfsNodeClusterStatus, error) {
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

func (cache *NodeManagerCache) GetNodeManagerIP(nodeManager RegistryNodeManager) (*string, error) {
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

func WaitByAllNodes(ctx context.Context, log *logrus.Entry, nodeManagers []RegistryNodeManager, cmpFuncs ...interface{}) (bool, error) {
	if len(nodeManagers) == 0 {
		return true, nil
	}

	for i := 0; i < pkg_cfg.MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			log.Infof("wait by all nodes, retry count %d/%d", i, pkg_cfg.MaxRetries)
			time.Sleep(5 * time.Second)

			isWaited := true
			nodeManagersCache := NewNodeManagerCache()

			for _, nodeManager := range nodeManagers {
				log.Infof("Checking node manager %s", nodeManager.GetNodeName())
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

func WaitByAnyNode(ctx context.Context, log *logrus.Entry, nodeManagers []RegistryNodeManager, cmpFuncs ...interface{}) (RegistryNodeManager, bool, error) {
	if len(nodeManagers) == 0 {
		return nil, false, nil
	}

	for i := 0; i < pkg_cfg.MaxRetries; i++ {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		default:
			log.Infof("wait by any node, retry count %d/%d", i, pkg_cfg.MaxRetries)
			time.Sleep(5 * time.Second)

			nodeManagersCache := NewNodeManagerCache()

			for _, nodeManager := range nodeManagers {
				isWaited := true

				log.Infof("Checking node manager %s", nodeManager.GetNodeName())
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
						return nil, false, fmt.Errorf("error, unknown function format: %v", cmpFunc)
					}
				}
				if isWaited {
					return nodeManager, true, nil
				}
			}
		}
	}
	return nil, false, nil
}

func SelectBy(nodeManagers []RegistryNodeManager, cmpFuncs ...interface{}) ([]RegistryNodeManager, []RegistryNodeManager, error) {
	if len(nodeManagers) == 0 {
		return nil, nil, nil
	}

	nodeManagersCache := NewNodeManagerCache()
	var selectedNodes []RegistryNodeManager
	var otherNodes []RegistryNodeManager

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

func SortBy(nodeManagers []RegistryNodeManager, cmpFuncs ...interface{}) ([]RegistryNodeManager, error) {
	if len(nodeManagers) == 0 {
		return nil, nil
	}

	nodeManagersStatusCache := NewNodeManagerCache()

	sortedNodesMap := map[int][]RegistryNodeManager{}
	other := []RegistryNodeManager{}

	for len(nodeManagers) > 0 {
		var nodeManager RegistryNodeManager
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

	sortedNodes := []RegistryNodeManager{}
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

func SplitNodesByCount(nodes []RegistryNodeManager, count int) ([]RegistryNodeManager, []RegistryNodeManager) {
	if len(nodes) < count {
		return nodes, []RegistryNodeManager{}
	}
	return nodes[:count], nodes[count:]
}

func GetNodeNames(nodes []RegistryNodeManager) string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.GetNodeName())
	}
	return fmt.Sprintf("[%s]", strings.Join(names, ","))
}

func DeleteNodes(ctx context.Context, log *logrus.Entry, nodes []RegistryNodeManager) error {
	log.Infof("Deleting nodes %s", GetNodeNames(nodes))
	for _, node := range nodes {
		status, err := node.GetNodeRunningStatus()
		if err == nil && !status.IsExist {
			log.Infof("Node %s has already been deleted", node.GetNodeName())
			continue
		}

		log.Infof("Deleting manifests for node %s", node.GetNodeName())
		if err := node.DeleteNodeManifests(); err != nil {
			return err
		}
	}
	return nil
}

func CreateNodes(ctx context.Context, log *logrus.Entry, nodes []RegistryNodeManager, createRequest *SeaweedfsCreateNodeRequest) error {
	for _, node := range nodes {
		log.Infof("Creating manifests for node %s", node.GetNodeName())
		if err := node.CreateNodeManifests(createRequest); err != nil {
			return err
		}
	}
	{
		log.Infof("CreateNodes :: WaitBy for: %s", GetNodeNames(nodes))
		wait, err := WaitByAllNodes(ctx, log, nodes, CmpIsRunning)
		if err != nil {
			return err
		}
		if !wait {
			return fmt.Errorf("error waitig nodes: %s", GetNodeNames(nodes))
		}
	}
	return nil
}

func RollingUpgradeNodes(ctx context.Context, log *logrus.Entry, nodes []RegistryNodeManager, updateRequest *SeaweedfsUpdateNodeRequest) error {
	for _, node := range nodes {
		nodeIP, err := node.GetNodeIP()
		if err != nil {
			return err
		}

		log.Infof("RollingUpgradeNodes :: UpdateNodeManifests for: %s", node.GetNodeName())
		if err := node.UpdateNodeManifests(updateRequest); err != nil {
			return err
		}

		log.Infof("RollingUpgradeNodes :: WaitByAllNodes (CmpIsRunning) for: %s", node.GetNodeName())
		isWait, err := WaitByAllNodes(ctx, log, []RegistryNodeManager{node}, CmpIsRunning)
		if err != nil {
			return fmt.Errorf("error waitig node %s: %s", node.GetNodeName(), err.Error())
		}
		if !isWait {
			return fmt.Errorf("error waitig node %s", node.GetNodeName())
		}

		log.Infof("RollingUpgradeNodes :: WaitLeaderElectionForNodes for: %s", GetNodeNames(nodes))
		leader, err := WaitLeaderElectionForNodes(ctx, log, nodes)
		if err != nil {
			return err
		}

		log.Infof("RollingUpgradeNodes :: WaitNodesConnection for: %s", GetNodeNames(nodes))
		err = WaitNodesConnection(ctx, log, leader, []string{nodeIP})
		if err != nil {
			return fmt.Errorf("error waitig node %s: %s", node.GetNodeName(), err.Error())
		}
	}
	return nil
}

func WaitLeaderElectionForNodes(ctx context.Context, log *logrus.Entry, nodes []RegistryNodeManager) (RegistryNodeManager, error) {
	log.Infof("WaitLeaderElectionForNodes :: WaitByAnyNode (CmpIsRunning, CpmIsLeader) for: %s", GetNodeNames(nodes))
	leader, wait, err := WaitByAnyNode(ctx, log, nodes, CmpIsRunning, CpmIsLeader)
	if err != nil {
		return nil, err
	}

	if !wait {
		return nil, fmt.Errorf("error wait leader election for: %s", GetNodeNames(nodes))
	}
	return leader, nil
}

func WaitNodesConnection(ctx context.Context, log *logrus.Entry, leader RegistryNodeManager, nodesIps []string) error {
	log.Infof("Waiting connection for nodes: [%s]", strings.Join(nodesIps, ","))
	var cmpFunc CpmFuncNodeClusterStatus = func(status *SeaweedfsNodeClusterStatus) bool {
		newIPsInCluster := true
		for _, ip := range nodesIps {
			newIPsInCluster = newIPsInCluster && pkg_utils.IsStringInSlice(ip, &status.ClusterNodesIPs)
		}
		return newIPsInCluster
	}

	log.Infof("WaitNodesConnection :: WaitBy for leader: %s, nodesIps: %s", leader.GetNodeName(), nodesIps)
	wait, err := WaitByAllNodes(ctx, log, []RegistryNodeManager{leader}, cmpFunc)
	if err != nil {
		return err
	}
	if !wait {
		return fmt.Errorf("error waitig connection for nodes: [%s]", strings.Join(nodesIps, ","))
	}
	return nil
}

func RemoveLeaderStatusForNode(ctx context.Context, log *logrus.Entry, clusterNodes []RegistryNodeManager, removeLeaderNode RegistryNodeManager) error {
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
	if _, err := WaitLeaderElectionForNodes(ctx, log, clusterNodes); err != nil {
		return err
	}

	// Remove node from cluster
	if err := clusterNode.RemoveNodeFromCluster(removedLeaderNodeIp); err != nil {
		return err
	}
	if _, err := WaitLeaderElectionForNodes(ctx, log, clusterNodes); err != nil {
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

func GetClustersMembers(nodeManagers []RegistryNodeManager) ([]ClusterMembers, error) {
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

	getNodeByIP := func(ip string) (RegistryNodeManager, error) {
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
		members := make([]RegistryNodeManager, 0, len(cluster))
		var leader RegistryNodeManager
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

func GetCurrentClustersMembers(ctx context.Context, log *logrus.Entry, clusterNodes []RegistryNodeManager) ([]ClusterMembers, error) {
	clustersMembers, err := GetClustersMembers(clusterNodes)
	if clustersMembers != nil {
		log.Infof("Clusters count %d", len(clustersMembers))
		log.Infof("Clusters members %+v", clustersMembers)
		log.Infof("Cluster[0] leader %+v", clustersMembers[0].Leader.GetNodeName())
	}
	return clustersMembers, err
}

func GetClustersLeaders(ctx context.Context, log *logrus.Entry, clusterNodes []RegistryNodeManager) ([]RegistryNodeManager, error) {
	clustersMembers, err := GetCurrentClustersMembers(ctx, log, clusterNodes)
	if err != nil {
		return nil, err
	}
	var leaders []RegistryNodeManager

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

func GetNewAndUnusedClusterIP(ctx context.Context, log *logrus.Entry, clusterNodes []RegistryNodeManager, removedNodes []RegistryNodeManager) (RegistryNodeManager, []string, []string, error) {
	leaders, err := GetClustersLeaders(ctx, log, clusterNodes)
	if err != nil {
		return nil, nil, nil, err
	}

	// Check if we have more than one leader, it's an error
	if len(leaders) > 1 {
		log.Infof("GetNewAndUnusedClusterIP: Have more than one cluster leaders")
		return nil, nil, nil, fmt.Errorf("len(*leaders) > 1")
	}

	leader := leaders[0]

	//	log.Infof("GetNewAndUnusedClusterIP: Current Leader: %s", leaders[0].GetNodeName())

	var ipsForCreate []string
	var ipsForDelete []string
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
		return leader, ipsForCreate, ipsForDelete, nil
	}

	var ipsFromCluster []string
	if leaderInfo, err := leader.GetNodeClusterStatus(); err != nil {
		return leader, nil, nil, err
	} else {
		ipsFromCluster = leaderInfo.ClusterNodesIPs
	}

	for _, ipFromCluster := range ipsFromCluster {
		if !pkg_utils.IsStringInSlice(ipFromCluster, &ipsForCreate) {
			ipsForDelete = pkg_utils.InsertString(ipFromCluster, ipsForDelete)
		}
	}

	return leader, ipsForCreate, ipsForDelete, nil
}
