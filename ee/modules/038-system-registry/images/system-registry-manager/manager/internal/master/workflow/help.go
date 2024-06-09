/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"sort"
	"strings"
	pkg_cfg "system-registry-manager/pkg/cfg"
	"time"
)

type CpmFuncNodeClusterStatus = func(*SeaweedfsNodeClusterStatus) bool
type CpmFuncNodeRunningStatus = func(*SeaweedfsNodeRunningStatus) bool

type ClusterMembers struct {
	Members []NodeManager
	Leader  NodeManager
}

type NodeManagerCache struct {
	data map[NodeManager]struct {
		RunningStatus *SeaweedfsNodeRunningStatus
		ClusterStatus *SeaweedfsNodeClusterStatus
		NodeIP        *string
	}
}

func NewNodeManagerCache() *NodeManagerCache {
	return &NodeManagerCache{
		data: map[NodeManager]struct {
			RunningStatus *SeaweedfsNodeRunningStatus
			ClusterStatus *SeaweedfsNodeClusterStatus
			NodeIP        *string
		}{},
	}
}

func (cache *NodeManagerCache) GetNodeManagerRunningStatus(nodeManager NodeManager) (*SeaweedfsNodeRunningStatus, error) {
	status, ok := cache.data[nodeManager]
	if ok && status.RunningStatus != nil {
		return status.RunningStatus, nil
	}
	runningStatus, err := nodeManager.GetNodeRunningStatus()
	if err != nil {
		return nil, err
	}
	status.RunningStatus = runningStatus
	cache.data[nodeManager] = status
	return runningStatus, nil
}

func (cache *NodeManagerCache) GetNodeManagerClusterStatus(nodeManager NodeManager) (*SeaweedfsNodeClusterStatus, error) {
	status, ok := cache.data[nodeManager]
	if ok && status.ClusterStatus != nil {
		return status.ClusterStatus, nil
	}
	clusterStatus, err := nodeManager.GetNodeClusterStatus()
	if err != nil {
		return nil, err
	}
	status.ClusterStatus = clusterStatus
	cache.data[nodeManager] = status
	return clusterStatus, nil
}

func (cache *NodeManagerCache) GetNodeManagerIP(nodeManager NodeManager) (*string, error) {
	status, ok := cache.data[nodeManager]
	if ok && status.NodeIP != nil {
		return status.NodeIP, nil
	}
	nodeIP, err := nodeManager.GetNodeIP()
	if err != nil {
		return nil, err
	}
	status.NodeIP = &nodeIP
	cache.data[nodeManager] = status
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

func WaitBy(log *logrus.Entry, nodeManagers []NodeManager, cmpFuncs ...interface{}) (bool, error) {
	if len(nodeManagers) == 0 {
		return true, nil
	}

	for i := 0; i < pkg_cfg.MaxRetries; i++ {
		log.Infof("wait by retry count %d/%d", i, pkg_cfg.MaxRetries)
		defer time.Sleep(5 * time.Second)

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
					if err != nil {
						isWaited = false
						break
					}
					isWaited = isWaited && f(status)
				case CpmFuncNodeRunningStatus:
					status, err := nodeManagersCache.GetNodeManagerRunningStatus(nodeManager)
					if err != nil {
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
					isSelected = false
					break
				}
				isSelected = isSelected && f(status)
			case CpmFuncNodeRunningStatus:
				status, err := nodeManagersCache.GetNodeManagerRunningStatus(nodeManager)
				if err != nil {
					return nil, nil, err
				}
				if status == nil {
					isSelected = false
					break
				}
				isSelected = isSelected && f(status)
			default:
				return nil, nil, fmt.Errorf("error, unknown function format: %v", cmpFunc)
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
	nodeManagersCopy := make([]NodeManager, len(nodeManagers))
	copy(nodeManagersCopy, nodeManagers)

	for len(nodeManagersCopy) > 0 {
		var nodeManager NodeManager
		nodeManager, nodeManagersCopy = nodeManagersCopy[0], nodeManagersCopy[1:]

		addedToSorted := false
		for cmpFuncPriority, cmpFunc := range cmpFuncs {
			switch f := cmpFunc.(type) {
			case CpmFuncNodeClusterStatus:
				status, err := nodeManagersStatusCache.GetNodeManagerClusterStatus(nodeManager)
				if err != nil {
					return nil, err
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
				if err == nil && clusterStatus.IsLeader {
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

func GetExpectedNodeCount(expectedNodeCount int) int {
	if expectedNodeCount == 0 || expectedNodeCount == 1 {
		return expectedNodeCount
	}
	if expectedNodeCount < 0 {
		return 0
	}
	if expectedNodeCount%2 == 0 {
		return expectedNodeCount - 1
	}
	return expectedNodeCount
}

func GetNodeNames(nodes []NodeManager) string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.GetNodeName())
	}
	return fmt.Sprintf("[%s]", strings.Join(names, ","))
}
