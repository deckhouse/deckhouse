/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

type SeaweedfsNodeManager interface {
	// Info
	GetNodeClusterStatus() (*SeaweedfsNodeClusterStatus, error)
	GetNodeRunningStatus() (*SeaweedfsNodeRunningStatus, error)
	GetNodeIP() string

	// Cluster actions
	AddNodeToCluster(newNodeIP string) error
	RemoveNodeFromCluster(removeNodeIP string) error

	// Runtime actions
	CreateNodeManifests(request *SeaweedfsCreateNodeRequest) error
	UpdateNodeManifests(request *SeaweedfsUpdateNodeRequest) error
	DeleteNodeManifests() error
}

type SeaweedfsNodeClusterStatus struct {
	IsMaster        bool
	ClusterNodesIPs []string
}

type SeaweedfsNodeRunningStatus struct {
	IsExist            bool
	IsRunning          bool
	NeedUpdateManifest bool
	NeedUpdateCerts    bool
	NeedUpdateCaCerts  bool
}

type SeaweedfsCreateNodeRequest struct {
	CreateManifestsData struct {
		MasterPeers []string
	}
}

type SeaweedfsUpdateNodeRequest struct {
	UpdateCert          bool
	UpdateCaCerts       bool
	UpdateManifests     bool
	UpdateManifestsData struct {
		MasterPeers []string
	}
}

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

func SelectByRunningStatus(nodes []*SeaweedfsNodeManager, cmpFuncs ...func(status *SeaweedfsNodeRunningStatus) bool) ([]*SeaweedfsNodeManager, []*SeaweedfsNodeManager, error) {
	if nodes == nil {
		return nil, nil, nil
	}

	selected := []*SeaweedfsNodeManager{}
	other := []*SeaweedfsNodeManager{}

	for _, node := range nodes {
		status, err := (*node).GetNodeRunningStatus()
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

func SortByStatus(nodes []*SeaweedfsNodeManager) ([]*SeaweedfsNodeManager, error) {
	if nodes == nil {
		return nil, nil
	}

	isRunning := make([]*SeaweedfsNodeManager, 0, len(nodes))
	isExist := []*SeaweedfsNodeManager{}
	other := []*SeaweedfsNodeManager{}

	for _, node := range nodes {
		nodeRunningStatus, err := (*node).GetNodeRunningStatus()
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

func GetMasters(nodes []*SeaweedfsNodeManager) ([]*SeaweedfsNodeManager, error) {
	visited := make(map[string]bool)
	nodeMap := make(map[string][]string)

	// Заполнение карты связей между узлами
	for _, node := range nodes {
		nodeInfo, err := (*node).GetNodeClusterStatus()
		if err != nil {
			return nil, err
		}

		nodeIP := (*node).GetNodeIP()

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

	masters := []*SeaweedfsNodeManager{}
	for _, cluster := range clusters {
		master := GetFirstNodeByIPs(nodes, cluster)
		if master != nil {
			masters = append(masters, master)
		}
	}
	return masters, nil
}

func GetFirstNodeByIPs(nodes []*SeaweedfsNodeManager, ips []string) *SeaweedfsNodeManager {
	for _, ip := range ips {
		if node := GetNodeByIP(nodes, ip); node != nil {
			return node
		}
	}
	return nil
}

func GetNodeByIP(nodes []*SeaweedfsNodeManager, ip string) *SeaweedfsNodeManager {
	for _, node := range nodes {
		if (*node).GetNodeIP() == ip {
			return node
		}
	}
	return nil
}

func GetExpectedNodeCount(expectedNodeCount int) int {
	if expectedNodeCount == 0 || expectedNodeCount == 1 {
		return expectedNodeCount
	}
	if expectedNodeCount < 0 {
		return 0
	}
	if expectedNodeCount%2 != 0 {
		// если четное - взять (ExpectedNodeCount - 1), чтобы получилось нечетное
		return expectedNodeCount - 1
	}
	return expectedNodeCount
}
