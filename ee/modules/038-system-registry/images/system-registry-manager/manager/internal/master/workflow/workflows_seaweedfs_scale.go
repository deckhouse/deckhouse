/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"context"
	"fmt"
	pkg_logs "system-registry-manager/pkg/logs"
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
	expectedNodeCount := GetExpectedNodeCount(w.ExpectedNodeCount)
	w.log.Infof("Starting scale workflow with expected node count: %d", expectedNodeCount)

	if expectedNodeCount > len(w.NodeManagers) {
		return fmt.Errorf("expectedNodeCount > len(w.NodeManagers)")
	}

	if expectedNodeCount == 0 {
		w.log.Info("Expected node count is 0, deleting all nodes.")
		return w.deleteNodes(w.NodeManagers)
	}

	w.log.Infof("Sorting nodes by running and exist status")
	sortedNodes, err := SortBy(w.NodeManagers, CmpIsRunning, CmpIsExist)
	if err != nil {
		return err
	}

	clusterNodes := sortedNodes[:expectedNodeCount]
	deleteNodes := sortedNodes[expectedNodeCount:]
	if err := w.needCluster(clusterNodes); err != nil {
		return err
	}

	w.log.Infof("Deleting nodes: %s", GetNodeNames(deleteNodes))
	return w.deleteNodes(deleteNodes)
}

func (w *SeaweedfsScaleWorkflow) needCluster(clusterNodes []NodeManager) error {
	w.log.Infof("Ensuring cluster for nodes: %s", GetNodeNames(clusterNodes))
	existNodes, _, err := SelectBy(clusterNodes, CmpIsExist)
	if err != nil {
		return err
	}

	if len(existNodes) == 0 {
		w.log.Infof("Creating new cluster with nodes %s", GetNodeNames(clusterNodes))
		return w.createCluster(clusterNodes)
	}

	w.log.Infof("Scaling existing cluster with nodes %s", GetNodeNames(clusterNodes))
	return w.scaleCluster(clusterNodes)
}

func (w *SeaweedfsScaleWorkflow) scaleCluster(allClusterNodes []NodeManager) error {
	w.log.Infof("Check and try sync cluster before scaling")
	if err := w.checkAndSyncCluster(allClusterNodes); err != nil {
		return err
	}

	runningNodes, notRunningNodes, err := SelectBy(allClusterNodes, CmpIsRunning)
	if err != nil {
		return err
	}

	w.log.Infof("Scaling cluster with running nodes: %s and not running nodes: %s", GetNodeNames(runningNodes), GetNodeNames(notRunningNodes))
	newIPs, unUsedIPs, err := w.getNewAndUnusedClusterIP(allClusterNodes)
	if err != nil {
		return err
	}

	w.log.Infof("Get cluster's leaders count")
	leaders, err := w.getClustersLeaders(allClusterNodes)
	if err != nil {
		return err
	}
	if len(leaders) != 1 {
		w.log.Infof("The number of leaders is not equal to 1")
		return fmt.Errorf("len(leaders) != 1")
	}
	leader := leaders[0]

	createRequest := SeaweedfsCreateNodeRequest{
		MasterPeers: newIPs,
	}

	checkRequest := SeaweedfsCheckNodeRequest{
		MasterPeers:          newIPs,
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
			MasterPeers:    unUsedIPs,
			UpdateOrCreate: true,
		},
	}

	// Add node to cluster, create manifests and wait
	for _, notRunningNode := range notRunningNodes {
		w.log.Infof("Creating manifests for new node %s", notRunningNode.GetNodeName())
		if err := notRunningNode.CreateNodeManifests(&createRequest); err != nil {
			return err
		}
	}

	{
		w.log.Infof("Waiting new nodes: %s", GetNodeNames(notRunningNodes))
		wait, err := WaitBy(w.log, notRunningNodes, CmpIsRunning)
		if err != nil {
			return err
		}
		if !wait {
			return fmt.Errorf("error waitig new nodes: %s", GetNodeNames(notRunningNodes))
		}
	}

	for _, notRunningNode := range notRunningNodes {
		nodeIp, err := notRunningNode.GetNodeIP()
		if err != nil {
			return err
		}
		w.log.Infof("Adding new node %s to cluster", notRunningNode.GetNodeName())
		if err := leader.AddNodeToCluster(nodeIp); err != nil {
			return err
		}
	}

	{
		w.log.Infof("Waiting cluster status for new nodes: %s", GetNodeNames(notRunningNodes))
		var cpmFunc CpmFuncNodeClusterStatus = func(status *SeaweedfsNodeClusterStatus) bool {
			newIPsInCluster := true
			for _, newIP := range newIPs {
				newIPsInCluster = newIPsInCluster && pkg_utils.IsStringInSlice(newIP, &status.ClusterNodesIPs)
			}
			return newIPsInCluster
		}

		wait, err := WaitBy(w.log, []NodeManager{leader}, cpmFunc)
		if err != nil {
			return err
		}
		if !wait {
			return fmt.Errorf("error waitig cluster status for new nodes: %s", GetNodeNames(notRunningNodes))
		}
	}

	// Check and update old nodes
	for _, runningNode := range runningNodes {
		nodeIP, err := runningNode.GetNodeIP()
		if err != nil {
			return err
		}

		w.log.Infof("Check manifests for current node %s", runningNode.GetNodeName())
		if checkResp, err := runningNode.CheckNodeManifests(&checkRequest); err != nil {
			return err
		} else {
			if !checkResp.NeedSomethingCreateOrUpdate() {
				continue
			}
		}

		w.log.Infof("Updating manifests for current node %s", runningNode.GetNodeName())
		if err := runningNode.UpdateNodeManifests(&updateRequest); err != nil {
			return err
		}

		w.log.Infof("Waiting current node %s", runningNode.GetNodeName())
		var cpmFunc CpmFuncNodeClusterStatus = func(status *SeaweedfsNodeClusterStatus) bool {
			return pkg_utils.IsStringInSlice(nodeIP, &status.ClusterNodesIPs)
		}
		wait, err := WaitBy(w.log, []NodeManager{runningNode}, CmpIsRunning, cpmFunc)
		if err != nil {
			return err
		}
		if !wait {
			return fmt.Errorf("error waitig current node %s", runningNode.GetNodeName())
		}
	}

	w.log.Infof("Check and try sync cluster after scaling")
	if err := w.checkAndSyncCluster(allClusterNodes); err != nil {
		return err
	}

	for _, oldIP := range unUsedIPs {
		if !pkg_utils.IsStringInSlice(oldIP, &newIPs) {
			w.log.Infof("Removing unused ip `%s` from cluster", oldIP)
			if err := leader.RemoveNodeFromCluster(oldIP); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) createCluster(clusterNodes []NodeManager) error {
	w.log.Infof("Creating new cluster with nodes: %s", GetNodeNames(clusterNodes))
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

	// Create manifests and wait cluster
	for _, node := range clusterNodes {
		w.log.Infof("Creating manifests for new node %s", node.GetNodeName())
		if err := node.CreateNodeManifests(&createRequest); err != nil {
			return err
		}
	}

	{
		w.log.Infof("Waiting new nodes: %s", GetNodeNames(clusterNodes))
		wait, err := WaitBy(w.log, clusterNodes, CmpIsRunning)
		if err != nil {
			return err
		}
		if !wait {
			return fmt.Errorf("error waitig new nodes: %s", GetNodeNames(clusterNodes))
		}
	}

	{
		w.log.Infof("Waiting leader election for new nodes: %s", GetNodeNames(clusterNodes))
		haveLeader := false
		var cpmFunc CpmFuncNodeClusterStatus = func(status *SeaweedfsNodeClusterStatus) bool {
			if status.IsLeader {
				haveLeader = true
			}
			return haveLeader
		}
		wait, err := WaitBy(w.log, clusterNodes, cpmFunc)
		if err != nil {
			return err
		}
		if !wait {
			return fmt.Errorf("error waitig cluster status for new nodes: %s", GetNodeNames(clusterNodes))
		}
	}

	w.log.Infof("Check and try sync new cluster after create")
	if err := w.checkAndSyncCluster(clusterNodes); err != nil {
		return err
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) checkAndSyncCluster(clusterNodes []NodeManager) error {
	return nil
}

func (w *SeaweedfsScaleWorkflow) getCurrentClustersMembers(clusterNodes []NodeManager) ([]ClusterMembers, error) {
	w.log.Infof("Get clusters members")
	clustersMembers, err := GetClustersMembers(clusterNodes)
	if clustersMembers != nil {
		w.log.Infof("Clusters count %d", len(clustersMembers))
	}
	return clustersMembers, err
}

func (w *SeaweedfsScaleWorkflow) getClustersLeaders(clusterNodes []NodeManager) ([]NodeManager, error) {
	w.log.Infof("Get clusters leaders")
	clustersMembers, err := w.getCurrentClustersMembers(clusterNodes)
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

func (w *SeaweedfsScaleWorkflow) getNewAndUnusedClusterIP(clusterNodes []NodeManager) ([]string, []string, error) {
	leaders, err := w.getClustersLeaders(clusterNodes)
	if err != nil {
		return nil, nil, err
	}

	if len(leaders) > 1 {
		w.log.Infof("Have more than one cluster leaders")
		return nil, nil, fmt.Errorf("len(*leaders) > 1")
	}

	ipsFromCurrentNodes := []string{}
	for _, node := range clusterNodes {
		nodeIp, err := node.GetNodeIP()
		if err != nil {
			return nil, nil, err
		}
		ipsFromCurrentNodes = append(ipsFromCurrentNodes, nodeIp)
	}

	if len(leaders) < 1 {
		return ipsFromCurrentNodes, []string{}, nil
	}

	leader := leaders[0]
	ipsFromCluster := []string{}
	unUsedIPs := []string{}

	// TODO
	if leaderInfo, err := leader.GetNodeClusterStatus(); err != nil {
		return nil, nil, err
	} else {
		ipsFromCluster = append(ipsFromCluster, leaderInfo.ClusterNodesIPs...)
	}

	for _, ipFromCluster := range ipsFromCluster {
		if !pkg_utils.IsStringInSlice(ipFromCluster, &ipsFromCurrentNodes) {
			unUsedIPs = append(unUsedIPs, ipFromCluster)
		}
	}
	return ipsFromCurrentNodes, unUsedIPs, nil
}

func (w *SeaweedfsScaleWorkflow) deleteNodes(nodes []NodeManager) error {
	w.log.Infof("Deleting nodes %s", GetNodeNames(nodes))
	for _, node := range nodes {
		status, err := node.GetNodeRunningStatus()
		if err != nil && !status.IsExist {
			w.log.Infof("Node %s has already been deleted", node.GetNodeName())
			return nil
		}

		w.log.Infof("Deleting manifests for node %s", node.GetNodeName())
		if err := node.DeleteNodeManifests(); err != nil {
			return err
		}
	}
	return nil
}
