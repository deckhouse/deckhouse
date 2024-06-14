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
	NodeManagers      []NodeManager
}

func NewSeaweedfsScaleWorkflow(ctx context.Context, nodeManagers []NodeManager, expectedNodeCount int) *SeaweedfsScaleWorkflow {
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

	w.log.Infof("Starting scale workflow with expected node count: %d", expectedNodesCount)

	if expectedNodesCount == 0 {
		w.log.Info("Expected node count is 0, deleting all nodes.")
		return DeleteNodes(w.ctx, w.log, w.NodeManagers)
	}

	existNodes, notExistNodes, err := SelectBy(w.NodeManagers, CmpIsExist)
	if err != nil {
		return err
	}
	if len(existNodes) == 0 {
		clusterNodes, _ := GetNodesByCount(notExistNodes, expectedNodesCount)
		return w.createCluster(clusterNodes)
	}

	if len(existNodes) == expectedNodesCount {
		return w.checkCluster(existNodes)
	}

	if len(existNodes) < expectedNodesCount {
		newClusterNodes, _ := GetNodesByCount(notExistNodes, expectedNodesCount-len(existNodes))
		return w.scaleUpCluster(existNodes, newClusterNodes)
	}

	if len(existNodes) > expectedNodesCount {
		sortedExistNodes, err := SortBy(existNodes, CmpIsRunning)
		if err != nil {
			return err
		}

		featureClusterNodes, deleteClusterNodes := GetNodesByCount(sortedExistNodes, len(existNodes)-expectedNodesCount)
		return w.scaleDownCluster(featureClusterNodes, deleteClusterNodes)
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) checkCluster(clusterNodes []NodeManager) error {
	w.log.Infof("Check cluster with nodes: %s", GetNodeNames(clusterNodes))

	// Prepare leader and ips
	w.log.Infof("Prepare IPs for new cluster")
	leaders, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, clusterNodes, []NodeManager{})
	if err != nil {
		return err
	}
	if len(leaders) != 1 {
		w.log.Infof("The number of leaders is not equal to 1")
		return fmt.Errorf("len(leaders) != 1")
	}
	leader := leaders[0]

	if pkg_utils.IsEvenNumber(len(clusterNodes)) {
		newClusterIPs = utils.InsertString(IpForEvenNodesNumber, newClusterIPs)
		unUsedIPs = utils.RemoveStringFromSlice(IpForEvenNodesNumber, unUsedIPs)
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
	needUpgrade := []NodeManager{}
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
	for _, ip := range newClusterIPs {
		if !utils.IsStringInSlice(ip, &clusterStatus.ClusterNodesIPs) {
			w.log.Infof("Adding IP %s to cluster", ip)
			if err := leader.AddNodeToCluster(ip); err != nil {
				return err
			}
		}
	}

	for _, ip := range unUsedIPs {
		w.log.Infof("Remove unused IP %s from cluster", ip)
		if err := leader.RemoveNodeFromCluster(ip); err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) createCluster(clusterNodes []NodeManager) error {
	w.log.Infof("Creating new cluster with nodes: %s", GetNodeNames(clusterNodes))
	createRequest := SeaweedfsCreateNodeRequest{
		MasterPeers: make([]string, 0, len(clusterNodes)),
	}

	if pkg_utils.IsEvenNumber(len(clusterNodes)) {
		createRequest.MasterPeers = append(createRequest.MasterPeers, IpForEvenNodesNumber)
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

	if err := WaitLeaderElectionForNodes(w.ctx, w.log, clusterNodes); err != nil {
		return err
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) scaleUpCluster(oldClusterNodes, newClusterNodes []NodeManager) error {
	w.log.Infof("Scaling up cluster %s with new nodes: %s", GetNodeNames(oldClusterNodes), GetNodeNames(newClusterNodes))

	// Prepare leader and ips
	w.log.Infof("Prepare IPs for new cluster")
	leaders, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, append(oldClusterNodes, newClusterNodes...), []NodeManager{})
	if err != nil {
		return err
	}
	if len(leaders) != 1 {
		w.log.Infof("The number of leaders is not equal to 1")
		return fmt.Errorf("len(leaders) != 1")
	}
	leader := leaders[0]

	if pkg_utils.IsEvenNumber(len(oldClusterNodes) + len(newClusterNodes)) {
		newClusterIPs = utils.InsertString(IpForEvenNodesNumber, newClusterIPs)
		unUsedIPs = utils.RemoveStringFromSlice(IpForEvenNodesNumber, unUsedIPs)
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
		w.log.Infof("Adding IP %s to cluster", ip)
		if err := leader.AddNodeToCluster(ip); err != nil {
			return err
		}
	}

	err = WaitNodesConnection(w.ctx, w.log, leader, newClusterIPs)
	if err != nil {
		return err
	}

	// RollingUpgrade old nodes
	needUpgrade := []NodeManager{}
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
		w.log.Infof("Remove unused IP %s from cluster", ip)
		if err := leader.RemoveNodeFromCluster(ip); err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) scaleDownCluster(futureClusterNodes, deleteClusterNodes []NodeManager) error {
	w.log.Infof("Scaling down cluster %s with nodes: %s", GetNodeNames(append(futureClusterNodes, deleteClusterNodes...)), GetNodeNames(deleteClusterNodes))
	for len(deleteClusterNodes) > 0 {
		var deleteNode NodeManager
		deleteNode, deleteClusterNodes = deleteClusterNodes[0], deleteClusterNodes[1:]
		w.scaleDownClusterPerNode(append(futureClusterNodes, deleteClusterNodes...), deleteNode)
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) scaleDownClusterPerNode(futureClusterNodes []NodeManager, deleteClusterNode NodeManager) error {
	w.log.Infof("Deleting node: %s", GetNodeNames([]NodeManager{deleteClusterNode}))

	// Change leader
	if err := RemoveLeaderStatusForNode(w.ctx, w.log, futureClusterNodes, deleteClusterNode); err != nil {
		return err
	}

	// Prepare leader and ips
	w.log.Infof("Prepare IPs for new cluster")
	leaders, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, futureClusterNodes, []NodeManager{deleteClusterNode})
	if err != nil {
		return err
	}
	if len(leaders) != 1 {
		w.log.Infof("The number of leaders is not equal to 1")
		return fmt.Errorf("len(leaders) != 1")
	}
	leader := leaders[0]

	if pkg_utils.IsEvenNumber(len(futureClusterNodes)) {
		newClusterIPs = utils.InsertString(IpForEvenNodesNumber, newClusterIPs)
		unUsedIPs = utils.RemoveStringFromSlice(IpForEvenNodesNumber, unUsedIPs)
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
		w.log.Infof("Remove unused IP %s from cluster", ip)
		if err := leader.RemoveNodeFromCluster(ip); err != nil {
			return err
		}
		if err := WaitLeaderElectionForNodes(w.ctx, w.log, futureClusterNodes); err != nil {
			return err
		}
	}

	// RollingUpgrade old nodes
	needUpgrade := []NodeManager{}
	for _, node := range futureClusterNodes {
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

	if err := DeleteNodes(w.ctx, w.log, []NodeManager{deleteClusterNode}); err != nil {
		return err
	}
	return nil
}
