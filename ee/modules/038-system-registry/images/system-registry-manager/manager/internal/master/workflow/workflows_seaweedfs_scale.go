/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"context"
	"fmt"
	pkg_logs "system-registry-manager/pkg/logs"
	"system-registry-manager/pkg/utils"
	pkg_utils "system-registry-manager/pkg/utils"

	"github.com/sirupsen/logrus"
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

func (w *SeaweedfsScaleWorkflow) Start() (err error) {
	expectedNodesCount := GetExpectedNodeCount(len(w.NodeManagers), w.ExpectedNodeCount)
	w.log.Infof("ScaleWorkflow :: Start :: Starting with expected node count: %d", expectedNodesCount)
	defer func() {
		if err != nil {
			w.log.Info("ScaleWorkflow :: Start :: completed successfully")
		} else {
			w.log.Errorf("ScaleWorkflow :: Start :: error: %s", err.Error())
		}
	}()

	if expectedNodesCount == 0 {
		w.log.Info("ScaleWorkflow :: Start :: Expected node count is 0, deleting all nodes.")
		return DeleteNodes(w.ctx, w.log, w.NodeManagers)
	}
	if expectedNodesCount%2 == 0 {
		expectedNodesCount--
		w.log.Warnf("ScaleWorkflow :: Start :: !!!!Expected node count is even, removing one node. new expected node count: %d", expectedNodesCount)
	}

	nodesWithManager, nodesWithoutManager, err := SelectBy(w.NodeManagers, CmpIsExist)
	w.log.Infof("ScaleWorkflow :: Start :: Nodes with Registry Manager: %s, nodes without Registry Manager: %s", GetNodeNames(nodesWithManager), GetNodeNames(nodesWithoutManager))
	if err != nil {
		return err
	}
	if len(nodesWithManager) == 0 {
		clusterNodes, _ := SplitNodesByCount(nodesWithoutManager, expectedNodesCount)
		return w.createCluster(clusterNodes)
	}

	if len(nodesWithManager) == expectedNodesCount {
		w.log.Infof("ScaleWorkflow :: Start :: nodesWithManager equal expectedNodesCount, nodesWithManager: %d, expectedNodesCount: %d", len(nodesWithManager), expectedNodesCount)
		return w.syncCluster(nodesWithManager)
	}

	if len(nodesWithManager) < expectedNodesCount {
		w.log.Infof("ScaleWorkflow :: Start :: nodesWithManager less than expectedNodesCount, nodesWithManager: %d, expectedNodesCount: %d", len(nodesWithManager), expectedNodesCount)
		newClusterNodes, _ := SplitNodesByCount(nodesWithoutManager, expectedNodesCount-len(nodesWithManager))
		return w.scaleUpCluster(nodesWithManager, newClusterNodes)
	}

	if len(nodesWithManager) > expectedNodesCount {
		w.log.Infof("ScaleWorkflow :: Start :: nodesWithManager more than expectedNodesCount, nodesWithManager: %d, expectedNodesCount: %d", len(nodesWithManager), expectedNodesCount)
		sortedExistNodes, err := SortBy(nodesWithManager, CmpIsRunning)
		if err != nil {
			return err
		}

		clusterNodes, clusterNodesToDelete := SplitNodesByCount(sortedExistNodes, len(nodesWithManager)-expectedNodesCount)
		w.log.Infof("ScaleWorkflow :: Start :: cluster nodes: %s, Cluster nodes to delete: %s", GetNodeNames(clusterNodes), GetNodeNames(clusterNodesToDelete))
		return w.scaleDownCluster(clusterNodes, clusterNodesToDelete)
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) syncCluster(clusterNodes []RegistryNodeManager) (err error) {
	w.log.Infof("ScaleWorkflow :: syncCluster :: Starting syncCluster with nodes: %s", GetNodeNames(clusterNodes))
	defer func() {
		if err != nil {
			w.log.Info("ScaleWorkflow :: syncCluster :: syncCluster completed successfully")
		} else {
			w.log.Errorf("ScaleWorkflow :: syncCluster :: error: %s", err.Error())
		}
	}()

	// Prepare leader and ips
	w.log.Infof("ScaleWorkflow :: syncCluster :: GetNewAndUnusedClusterIP")
	leader, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, clusterNodes, []RegistryNodeManager{})
	w.log.Infof("ScaleWorkflow :: syncCluster :: RAW!!! error: %v, leader: %v, newClusterIPs: %v, unUsedIPs: %v", err, leader, newClusterIPs, unUsedIPs)
	if err != nil {
		return err
	}

	leaderIP, _ := leader.GetNodeIP()
	w.log.Infof("ScaleWorkflow :: syncCluster :: RAW!!! error: %s, leaderIP: %v, newClusterIPs: %v, unUsedIPs: %v", err, leaderIP, newClusterIPs, unUsedIPs)
	if err != nil {
		return err
	}

	// Prepare requests
	checkRequest := SeaweedfsCheckNodeRequest{
		MasterPeers:          newClusterIPs,
		CheckWithMasterPeers: true,
	}

	updateRequest := SeaweedfsUpdateNodeRequest{
		Certs: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		Manifests: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		StaticPods: struct {
			MasterPeers    []string `json:"masterPeers"`
			UpdateOrCreate bool     `json:"updateOrCreate"`
		}{
			MasterPeers:    newClusterIPs,
			UpdateOrCreate: true,
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
	err = RollingUpgradeNodes(w.ctx, w.log, needUpgrade, &updateRequest)
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
			w.log.Infof("ScaleWorkflow :: syncCluster :: Adding IP %s to cluster", newClusterIP)
			if err := leader.AddNodeToCluster(newClusterIP); err != nil {
				return err
			}
		}
	}

	for _, ip := range unUsedIPs {
		w.log.Infof("ScaleWorkflow :: syncCluster :: Remove unused IP %s from cluster", ip)
		if err := leader.RemoveNodeFromCluster(ip); err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) createCluster(clusterNodes []RegistryNodeManager) (err error) {
	w.log.Infof("ScaleWorkflow :: createCluster :: Creating new cluster with nodes: %s", GetNodeNames(clusterNodes))
	defer func() {
		if err != nil {
			w.log.Info("ScaleWorkflow :: createCluster :: createCluster completed successfully")
		} else {
			w.log.Errorf("ScaleWorkflow :: createCluster :: error: %s", err.Error())
		}
	}()

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

func (w *SeaweedfsScaleWorkflow) scaleUpCluster(oldClusterNodes, newClusterNodes []RegistryNodeManager) (err error) {
	w.log.Infof("ScaleWorkflow :: scaleUpCluster :: Scaling up cluster %s with new nodes: %s", GetNodeNames(oldClusterNodes), GetNodeNames(newClusterNodes))
	defer func() {
		if err != nil {
			w.log.Info("ScaleWorkflow :: scaleUpCluster :: scaleUpCluster completed successfully")
		} else {
			w.log.Errorf("ScaleWorkflow :: scaleUpCluster :: error: %s", err.Error())
		}
	}()

	// Prepare leader and ips
	w.log.Infof("ScaleWorkflow :: syncCluster :: GetNewAndUnusedClusterIP")
	leader, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, append(oldClusterNodes, newClusterNodes...), []RegistryNodeManager{})
	w.log.Infof("ScaleWorkflow :: scaleUpCluster :: RAW!!! error: %v, leader: %v, newClusterIPs: %v, unUsedIPs: %v", err, leader, newClusterIPs, unUsedIPs)
	if err != nil {
		return err
	}

	leaderIP, _ := leader.GetNodeIP()
	w.log.Infof("ScaleWorkflow :: scaleUpCluster :: RAW!!! error: %s, leaderIP: %v, newClusterIPs: %v, unUsedIPs: %v", err, leaderIP, newClusterIPs, unUsedIPs)
	if err != nil {
		return err
	}

	// Prepare requests
	createRequest := SeaweedfsCreateNodeRequest{
		MasterPeers: newClusterIPs,
	}

	checkRequest := SeaweedfsCheckNodeRequest{
		MasterPeers:          newClusterIPs,
		CheckWithMasterPeers: true,
	}

	updateRequest := SeaweedfsUpdateNodeRequest{
		Certs: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		Manifests: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		StaticPods: struct {
			MasterPeers    []string `json:"masterPeers"`
			UpdateOrCreate bool     `json:"updateOrCreate"`
		}{
			MasterPeers:    newClusterIPs,
			UpdateOrCreate: true,
		},
	}

	// Add node to cluster, create manifests and wait
	err = CreateNodes(w.ctx, w.log, newClusterNodes, &createRequest)
	if err != nil {
		return err
	}

	for _, ip := range newClusterIPs {
		w.log.Infof("ScaleWorkflow :: scaleUpCluster :: Adding IP %s to cluster", ip)
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
	err = RollingUpgradeNodes(w.ctx, w.log, needUpgrade, &updateRequest)
	if err != nil {
		return err
	}

	// Remove unused nodes from cluster
	for _, ip := range unUsedIPs {
		w.log.Infof("ScaleWorkflow :: scaleUpCluster :: Remove unused IP %s from cluster", ip)
		if err := leader.RemoveNodeFromCluster(ip); err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) scaleDownCluster(clusterNodes, clusterNodesToDelete []RegistryNodeManager) (err error) {
	w.log.Infof("ScaleWorkflow :: scaleDownCluster :: Scaling down cluster %s, with nodes: %s", GetNodeNames(append(clusterNodes, clusterNodesToDelete...)), GetNodeNames(clusterNodesToDelete))
	defer func() {
		if err != nil {
			w.log.Info("ScaleWorkflow :: scaleDownCluster :: scaleDownCluster completed successfully")
		} else {
			w.log.Errorf("ScaleWorkflow :: scaleDownCluster :: error: %s", err.Error())
		}
	}()

	for len(clusterNodesToDelete) > 0 {
		var deleteNode RegistryNodeManager
		deleteNode, clusterNodesToDelete = clusterNodesToDelete[0], clusterNodesToDelete[1:]
		err := w.scaleDownClusterPerNode(append(clusterNodes, clusterNodesToDelete...), deleteNode)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) scaleDownClusterPerNode(clusterNodes []RegistryNodeManager, clusterNodeToRemove RegistryNodeManager) (err error) {
	w.log.Infof("ScaleWorkflow :: scaleDownClusterPerNode :: Deleting node: %s", GetNodeNames([]RegistryNodeManager{clusterNodeToRemove}))
	defer func() {
		if err != nil {
			w.log.Info("ScaleWorkflow :: scaleDownClusterPerNode :: scaleDownCluster completed successfully")
		} else {
			w.log.Errorf("ScaleWorkflow :: scaleDownClusterPerNode :: error: %s", err.Error())
		}
	}()

	// Change leader
	w.log.Infof("ScaleWorkflow :: scaleDownClusterPerNode :: emoveLeaderStatusForNode)")
	if err := RemoveLeaderStatusForNode(w.ctx, w.log, clusterNodes, clusterNodeToRemove); err != nil {
		return err
	}

	// Prepare leader and ips
	w.log.Infof("ScaleWorkflow :: scaleDownClusterPerNode :: !!!! POTENTIAL PROBLEM. GetNewAndUnusedClusterIP\n")

	leader, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, clusterNodes, []RegistryNodeManager{clusterNodeToRemove})
	if err != nil {
		return err
	}

	if pkg_utils.IsEvenNumber(len(clusterNodes)) {
		return fmt.Errorf("the number of nodes is even")
	}

	// Prepare requests
	checkRequest := SeaweedfsCheckNodeRequest{
		MasterPeers:          newClusterIPs,
		CheckWithMasterPeers: true,
	}

	updateRequest := SeaweedfsUpdateNodeRequest{
		Certs: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		Manifests: struct {
			UpdateOrCreate bool `json:"updateOrCreate"`
		}{true},
		StaticPods: struct {
			MasterPeers    []string `json:"masterPeers"`
			UpdateOrCreate bool     `json:"updateOrCreate"`
		}{
			MasterPeers:    newClusterIPs,
			UpdateOrCreate: true,
		},
	}

	// Remove unused nodes from cluster
	for _, ip := range unUsedIPs {
		w.log.Infof("ScaleWorkflow :: scaleDownClusterPerNode :: Remove unused IP %s from cluster", ip)
		if err := leader.RemoveNodeFromCluster(ip); err != nil {
			return err
		}
		if _, err := WaitLeaderElectionForNodes(w.ctx, w.log, clusterNodes); err != nil {
			return err
		}
	}

	// RollingUpgrade old nodes
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
	w.log.Infof("ScaleWorkflow :: scaleDownClusterPerNode :: RollingUpgradeNodes: %v, %+v", needUpgrade, updateRequest)
	err = RollingUpgradeNodes(w.ctx, w.log, needUpgrade, &updateRequest)
	if err != nil {
		return err
	}

	if err := DeleteNodes(w.ctx, w.log, []RegistryNodeManager{clusterNodeToRemove}); err != nil {
		return err
	}
	return nil
}
