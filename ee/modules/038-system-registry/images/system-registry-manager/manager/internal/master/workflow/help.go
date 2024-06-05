/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"fmt"
	"strings"
)

func CmpSelectIsNeedUpdateCaCerts(status *SeaweedfsNodeRunningStatus) bool {
	if status == nil {
		return false
	}
	return status.NeedUpdateCaCerts
}

func CmpSelectIsNotExist(status *SeaweedfsNodeRunningStatus) bool {
	if status == nil {
		return false
	}
	return !status.IsExist
}

func CmpSelectIsExist(status *SeaweedfsNodeRunningStatus) bool {
	if status == nil {
		return false
	}
	return status.IsExist
}

func CmpSelectIsNotRunning(status *SeaweedfsNodeRunningStatus) bool {
	if status == nil {
		return false
	}
	return !status.IsRunning
}

func CmpSelectIsRunning(status *SeaweedfsNodeRunningStatus) bool {
	if status == nil {
		return false
	}
	return status.IsRunning
}

func SelectByRunningStatus(nodes []NodeManager, cmpFuncs ...func(status *SeaweedfsNodeRunningStatus) bool) ([]NodeManager, []NodeManager, error) {
	if nodes == nil {
		return nil, nil, nil
	}

	selected := []NodeManager{}
	other := []NodeManager{}

	for _, node := range nodes {
		status, err := node.GetNodeRunningStatus()
		cmpResult := true

		if err != nil {
			return nil, nil, err
		}
		for _, cmpF := range cmpFuncs {
			cmpResult = cmpResult && cmpF(status)
		}
		if cmpResult {
			selected = append(selected, node)
		} else {
			other = append(other, node)
		}
	}
	return selected, other, nil
}

func GetNodeNames(nodes []NodeManager) string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.GetNodeName())
	}
	return fmt.Sprintf("[%s]", strings.Join(names, ","))
}

func SortByStatus(nodes []NodeManager) ([]NodeManager, error) {
	if nodes == nil {
		return nil, nil
	}

	isRunning := make([]NodeManager, 0, len(nodes))
	isExist := []NodeManager{}
	other := []NodeManager{}

	for _, node := range nodes {
		nodeRunningStatus, err := node.GetNodeRunningStatus()
		if err != nil {
			return nil, err
		}
		switch {
		case nodeRunningStatus.IsRunning:
			isRunning = append(isRunning, node)
		case nodeRunningStatus.IsExist:
			isExist = append(isExist, node)
		default:
			other = append(other, node)
		}
	}
	isRunning = append(isRunning, isExist...)
	isRunning = append(isRunning, other...)
	return isRunning, nil
}

func GetMasters(nodes []NodeManager) ([]NodeManager, error) {
	visited := make(map[string]bool)
	nodeMap := make(map[string][]string)

	// Заполнение карты связей между узлами
	for _, node := range nodes {
		nodeInfo, err := node.GetNodeClusterStatus()
		if err != nil {
			return nil, err
		}

		nodeIP, err := node.GetNodeIP()
		if err != nil {
			return nil, err
		}

		nodeMap[nodeIP] = nodeInfo.ClusterNodesIPs
		for _, ip := range nodeInfo.ClusterNodesIPs {
			nodeMap[ip] = append(nodeMap[ip], nodeIP)
		}
	}

	var clusters [][]string

	// Вспомогательная функция для поиска в глубину (DFS)
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

	// Поиск всех кластеров
	for ip := range nodeMap {
		if !visited[ip] {
			cluster := dfs(ip, []string{})
			clusters = append(clusters, cluster)
		}
	}

	masters := []NodeManager{}
	for _, cluster := range clusters {
		master, err := GetFirstNodeByIPs(nodes, cluster)
		if err != nil {
			return nil, err
		}
		if master != nil {
			masters = append(masters, master)
		}
	}
	return masters, nil
}

func GetFirstNodeByIPs(nodes []NodeManager, ips []string) (NodeManager, error) {
	for _, ip := range ips {
		node, err := GetNodeByIP(nodes, ip)
		if err != nil {
			return nil, err
		}
		if node != nil {
			return node, nil
		}
	}
	return nil, nil
}

func GetNodeByIP(nodes []NodeManager, ip string) (NodeManager, error) {
	for _, node := range nodes {
		nodeIp, err := node.GetNodeIP()
		if err != nil {
			return nil, err
		}
		if nodeIp == ip {
			return node, nil
		}
	}
	return nil, nil
}

func GetExpectedNodeCount(expectedNodeCount int) int {
	if expectedNodeCount == 0 || expectedNodeCount == 1 {
		return expectedNodeCount
	}
	if expectedNodeCount < 0 {
		return 0
	}
	if expectedNodeCount%2 == 0 {
		// если четное - взять (ExpectedNodeCount - 1), чтобы получилось нечетное
		return expectedNodeCount - 1
	}
	return expectedNodeCount
}
