/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	pkg_logs "system-registry-manager/pkg/logs"
	"system-registry-manager/pkg/utils"
)

type SeaweedfsScaleWorkflow struct {
	log               *logrus.Entry
	ctx               context.Context
	ExpectedNodeCount int
	NodeManagers      []RegistryNodeManager
}

func NewSeaweedfsScaleWorkflow(ctx context.Context, nodeManagers []RegistryNodeManager, expectedNodeCount int) *SeaweedfsScaleWorkflow {
	log := pkg_logs.GetLoggerFromContext(ctx)
	return &SeaweedfsScaleWorkflow{
		log:               log,
		ctx:               ctx,
		ExpectedNodeCount: expectedNodeCount,
		NodeManagers:      nodeManagers,
	}
}

func (w *SeaweedfsScaleWorkflow) Start() error {
	expectedNodesCount := GetExpectedNodeCount(len(w.NodeManagers), w.ExpectedNodeCount)
	w.log.Infof("▶️ ScaleWorkflow :: Start :: Starting with expected node count: %d", expectedNodesCount)

	if expectedNodesCount == 0 {
		w.log.Info("Start :: Expected node count is 0, deleting all nodes.")
		return DeleteNodes(w.ctx, w.log, w.NodeManagers)
	}
	if expectedNodesCount%2 == 0 {
		expectedNodesCount--
		w.log.Warnf("Start :: !!!!Expected node count is even, removing one node. new expected node count: %d", expectedNodesCount)
	}

	nodesWithManager, nodesWithoutManager, err := SelectBy(w.NodeManagers, CmpIsExist)
	w.log.Infof("Start :: Nodes with Registry Manager: %s, nodes without Registry Manager: %s", GetNodeNames(nodesWithManager), GetNodeNames(nodesWithoutManager))
	if err != nil {
		return err
	}
	if len(nodesWithManager) == 0 {
		clusterNodes, _ := SplitNodesByCount(nodesWithoutManager, expectedNodesCount)
		return w.createCluster(clusterNodes)
	}

	// if len(nodesWithManager) == expectedNodesCount {
	// 	w.log.Infof("Start :: nodesWithManager equal expectedNodesCount, nodesWithManager: %d, expectedNodesCount: %d", len(nodesWithManager), expectedNodesCount)
	// 	return w.syncCluster(nodesWithManager)
	// }

	if len(nodesWithManager) < expectedNodesCount {
		w.log.Infof("Start :: nodesWithManager less than expectedNodesCount, nodesWithManager: %d, expectedNodesCount: %d", len(nodesWithManager), expectedNodesCount)
		newClusterNodes, _ := SplitNodesByCount(nodesWithoutManager, expectedNodesCount-len(nodesWithManager))
		return w.scaleUpCluster(nodesWithManager, newClusterNodes)
	}

	if len(nodesWithManager) > expectedNodesCount {
		w.log.Infof("Start :: nodesWithManager more than expectedNodesCount, nodesWithManager: %d, expectedNodesCount: %d", len(nodesWithManager), expectedNodesCount)
		sortedExistNodes, err := SortBy(nodesWithManager, CmpIsRunning)
		if err != nil {
			return err
		}

		clusterNodes, clusterNodesToDelete := SplitNodesByCount(sortedExistNodes, expectedNodesCount)
		w.log.Infof("Start :: cluster nodes: %s, Cluster nodes to delete: %s", GetNodeNames(clusterNodes), GetNodeNames(clusterNodesToDelete))
		return w.scaleDownCluster(clusterNodes, clusterNodesToDelete)
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) syncCluster(clusterNodes []RegistryNodeManager) error {
	w.log.Infof("ScaleWorkflow :: syncCluster :: Starting syncCluster with nodes: %s", GetNodeNames(clusterNodes))

	// Prepare leader and ips
	w.log.Infof("syncCluster :: GetNewAndUnusedClusterIP")
	leader, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, clusterNodes, []RegistryNodeManager{})
	//w.log.Infof("syncCluster :: RAW!!! error: %v, leaderName: %s, newClusterIPs: %v, unUsedIPs: %v", err, leader.GetNodeName(), newClusterIPs, unUsedIPs)
	if err != nil {
		return err
	}

	//leaderIP, _ := leader.GetNodeIP()
	//w.log.Infof("syncCluster :: RAW!!! error: %s, leaderIP: %v, newClusterIPs: %v, unUsedIPs: %v", err, leaderIP, newClusterIPs, unUsedIPs)
	if err != nil {
		return err
	}

	// Prepare requests
	checkRequest := SeaweedfsCheckNodeRequest{
		Options: struct {
			MasterPeers     []string "json:\"masterPeers\""
			IsRaftBootstrap bool     "json:\"isRaftBootstrap\""
		}{
			MasterPeers:     newClusterIPs,
			IsRaftBootstrap: false,
		},
		Check: struct {
			WithMasterPeers     bool "json:\"withMasterPeers\""
			WithIsRaftBootstrap bool "json:\"withIsRaftBootstrap\""
		}{
			WithMasterPeers:     true,
			WithIsRaftBootstrap: true,
		},
	}

	updateRequest := SeaweedfsUpdateNodeRequest{
		Certs: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		Manifests: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		StaticPods: struct {
			MasterPeers     []string "json:\"masterPeers\""
			IsRaftBootstrap bool     "json:\"isRaftBootstrap\""
			UpdateOrCreate  bool     "json:\"updateOrCreate\""
		}{
			MasterPeers:     newClusterIPs,
			IsRaftBootstrap: false,
			UpdateOrCreate:  true,
		},
	}

	// RollingUpgrade all nodes if need
	needUpgrade := []RegistryNodeManager{}
	for _, node := range clusterNodes {
		request, err := node.CheckNodeManifests(&checkRequest)
		if err != nil {
			return err
		}
		if request.NeedSomethingCreateOrUpdate() {
			needUpgrade = append(needUpgrade, node)
		}
	}
	err = RollingUpgradeNodes(w.ctx, w.log, clusterNodes, needUpgrade, &updateRequest)
	if err != nil {
		return err
	}

	err = WaitNodesConnection(w.ctx, w.log, leader, newClusterIPs)
	if err != nil {
		return err
	}

	// Add used and remove unused nodes from cluster
	clusterStatus, err := leader.GetNodeClusterStatus()
	if err != nil {
		return err
	}
	for _, newClusterIP := range newClusterIPs {
		if !utils.IsStringInSlice(newClusterIP, &clusterStatus.ClusterNodesIPs) {
			w.log.Infof("syncCluster :: Adding IP %s to cluster", newClusterIP)
			if err := leader.AddNodeToCluster(newClusterIP); err != nil {
				return err
			}
		}
	}

	for _, ip := range unUsedIPs {
		w.log.Infof("syncCluster :: Remove unused IP %s from cluster", ip)
		if err := leader.RemoveNodeFromCluster(ip); err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) createCluster(clusterNodes []RegistryNodeManager) error {
	w.log.Infof("ScaleWorkflow :: createCluster :: Creating new cluster with nodes: %s", GetNodeNames(clusterNodes))

	createRequest := SeaweedfsCreateNodeRequest{
		MasterPeers: make([]string, 0, len(clusterNodes)),
	}

	for _, node := range clusterNodes {
		nodeIp, err := node.GetNodeIP()
		if err != nil {
			return err
		}
		createRequest.MasterPeers = append(createRequest.MasterPeers, nodeIp)
	}

	if err := CreateNodes(w.ctx, w.log, clusterNodes, &createRequest); err != nil {
		return err
	}

	if _, err := WaitLeaderElectionForNodes(w.ctx, w.log, clusterNodes); err != nil {
		return err
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) scaleUpCluster(oldClusterNodes, newClusterNodes []RegistryNodeManager) error {
	w.log.Infof("ScaleWorkflow :: scaleUpCluster :: Scaling up cluster %s with new nodes: %s", GetNodeNames(oldClusterNodes), GetNodeNames(newClusterNodes))

	// Prepare leader and ips
	w.log.Infof("scaleUpCluster :: GetNewAndUnusedClusterIP")
	leader, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, append(oldClusterNodes, newClusterNodes...), []RegistryNodeManager{})
	//w.log.Infof("scaleUpCluster :: RAW!!! error: %v, leaderName: %s, newClusterIPs: %v, unUsedIPs: %v", err, leader.GetNodeName(), newClusterIPs, unUsedIPs)
	if err != nil {
		return err
	}

	//leaderIP, _ := leader.GetNodeIP()
	//w.log.Infof("scaleUpCluster :: RAW!!! error: %s, leaderIP: %v, newClusterIPs: %v, unUsedIPs: %v", err, leaderIP, newClusterIPs, unUsedIPs)
	if err != nil {
		return err
	}

	// Prepare requests
	createRequest := SeaweedfsCreateNodeRequest{
		MasterPeers: newClusterIPs,
	}

	checkRequest := SeaweedfsCheckNodeRequest{
		Options: struct {
			MasterPeers     []string "json:\"masterPeers\""
			IsRaftBootstrap bool     "json:\"isRaftBootstrap\""
		}{
			MasterPeers:     newClusterIPs,
			IsRaftBootstrap: false,
		},
		Check: struct {
			WithMasterPeers     bool "json:\"withMasterPeers\""
			WithIsRaftBootstrap bool "json:\"withIsRaftBootstrap\""
		}{
			WithMasterPeers:     true,
			WithIsRaftBootstrap: true,
		},
	}

	updateRequest := SeaweedfsUpdateNodeRequest{
		Certs: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		Manifests: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		StaticPods: struct {
			MasterPeers     []string "json:\"masterPeers\""
			IsRaftBootstrap bool     "json:\"isRaftBootstrap\""
			UpdateOrCreate  bool     "json:\"updateOrCreate\""
		}{
			MasterPeers:     newClusterIPs,
			IsRaftBootstrap: false,
			UpdateOrCreate:  true,
		},
	}

	// Add node to cluster, create manifests and wait
	err = CreateNodes(w.ctx, w.log, newClusterNodes, &createRequest)
	if err != nil {
		return err
	}

	for _, ip := range newClusterIPs {
		w.log.Infof("scaleUpCluster :: Adding IP %s to cluster", ip)
		if err := leader.AddNodeToCluster(ip); err != nil {
			return err
		}
	}

	err = WaitNodesConnection(w.ctx, w.log, leader, newClusterIPs)
	if err != nil {
		return err
	}

	// RollingUpgrade old nodes
	needUpgrade := []RegistryNodeManager{}
	for _, node := range oldClusterNodes {
		request, err := node.CheckNodeManifests(&checkRequest)
		if err != nil {
			return err
		}
		if request.NeedSomethingCreateOrUpdate() {
			needUpgrade = append(needUpgrade, node)
		}
	}
	err = RollingUpgradeNodes(w.ctx, w.log, append(oldClusterNodes, newClusterNodes...), needUpgrade, &updateRequest)
	if err != nil {
		return err
	}

	// Remove unused nodes from cluster
	for _, ip := range unUsedIPs {
		w.log.Infof("scaleUpCluster :: Remove unused IP %s from cluster", ip)
		if err := leader.RemoveNodeFromCluster(ip); err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) scaleDownCluster(clusterNodes, clusterNodesToDelete []RegistryNodeManager) error {
	w.log.Infof("ScaleWorkflow :: scaleDownCluster :: Scaling down cluster %s, with nodes: %s", GetNodeNames(append(clusterNodes, clusterNodesToDelete...)), GetNodeNames(clusterNodesToDelete))

	if len(clusterNodes) != 1 {
		return fmt.Errorf("len(clusterNodes) != 1")
	}

	clusterNode := clusterNodes[0]
	clusterNodeIP, err := clusterNode.GetNodeIP()
	if err != nil {
		return err
	}

	// Prepare requests
	checkRequest := SeaweedfsCheckNodeRequest{
		Options: struct {
			MasterPeers     []string "json:\"masterPeers\""
			IsRaftBootstrap bool     "json:\"isRaftBootstrap\""
		}{
			MasterPeers:     []string{clusterNodeIP},
			IsRaftBootstrap: true,
		},
		Check: struct {
			WithMasterPeers     bool "json:\"withMasterPeers\""
			WithIsRaftBootstrap bool "json:\"withIsRaftBootstrap\""
		}{
			WithMasterPeers:     true,
			WithIsRaftBootstrap: true,
		},
	}

	updateRequest := SeaweedfsUpdateNodeRequest{
		Certs: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		Manifests: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		StaticPods: struct {
			MasterPeers     []string "json:\"masterPeers\""
			IsRaftBootstrap bool     "json:\"isRaftBootstrap\""
			UpdateOrCreate  bool     "json:\"updateOrCreate\""
		}{
			MasterPeers:     []string{clusterNodeIP},
			IsRaftBootstrap: true,
			UpdateOrCreate:  true,
		},
	}

	for _, nodeToDelete := range clusterNodesToDelete {
		if err := nodeToDelete.DeleteNodeManifests(); err != nil {
			return err
		}
	}

	resp, err := clusterNode.CheckNodeManifests(&checkRequest)
	if err != nil {
		return err
	}

	if resp.NeedSomethingCreateOrUpdate() {
		if err := clusterNode.UpdateNodeManifests(&updateRequest); err != nil {
			return err
		}
	}

	if isWait, err := WaitByAllNodes(w.ctx, w.log, []RegistryNodeManager{clusterNode}, CmpIsExist, CmpIsRunning); err != nil {
		return err
	} else {
		if !isWait {
			return fmt.Errorf("!isWait WaitByAllNodes(w.ctx, w.log, []RegistryNodeManager{clusterNode}, CmpIsExist, CmpIsRunning)")
		}
	}
	return nil
}
