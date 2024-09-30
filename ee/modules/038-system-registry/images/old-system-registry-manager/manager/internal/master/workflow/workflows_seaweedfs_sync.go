/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"context"
	"github.com/sirupsen/logrus"
	pkg_logs "system-registry-manager/pkg/logs"
	"system-registry-manager/pkg/utils"
)

type SeaweedfsSyncWorkflow struct {
	log               *logrus.Entry
	ctx               context.Context
	ExpectedNodeCount int
	NodeManagers      []RegistryNodeManager
}

func NewSeaweedfsSyncWorkflow(ctx context.Context, nodeManagers []RegistryNodeManager, expectedNodeCount int) *SeaweedfsSyncWorkflow {
	log := pkg_logs.GetLoggerFromContext(ctx)
	return &SeaweedfsSyncWorkflow{
		log:               log,
		ctx:               ctx,
		ExpectedNodeCount: expectedNodeCount,
		NodeManagers:      nodeManagers,
	}
}

func (w *SeaweedfsSyncWorkflow) Start() error {
	expectedNodesCount := GetExpectedNodeCount(len(w.NodeManagers), w.ExpectedNodeCount)
	w.log.Infof("▶️ SyncWorkflow :: Start :: Starting with expected node count: %d", expectedNodesCount)

	if expectedNodesCount%2 == 0 {
		expectedNodesCount--
		w.log.Warnf("🛑 Start :: Expected node count is even, removing one node. new expected node count: %d", expectedNodesCount)
	}

	nodesWithManager, nodesWithoutManager, err := SelectBy(w.NodeManagers, CmpIsExist)
	w.log.Infof("Start :: Nodes with Registry Manager: %s, nodes without Registry Manager: %s", GetNodeNames(nodesWithManager), GetNodeNames(nodesWithoutManager))
	if err != nil {
		return err
	}

	if len(nodesWithManager) == expectedNodesCount {
		w.log.Infof("Start :: nodesWithManager equal expectedNodesCount, nodesWithManager: %d, expectedNodesCount: %d", len(nodesWithManager), expectedNodesCount)
		return w.syncCluster(nodesWithManager)
	}

	return nil
}

func (w *SeaweedfsSyncWorkflow) syncCluster(clusterNodes []RegistryNodeManager) error {
	w.log.Infof("ScaleWorkflow :: syncCluster :: Starting syncCluster with nodes: %s", GetNodeNames(clusterNodes))

	// Prepare leader and ips
	leader, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, clusterNodes, []RegistryNodeManager{})
	if err != nil {
		return err
	}

	w.log.Infof("syncCluster :: newClusterIPs: %v, unUsedIPs: %v, leader: %s", newClusterIPs, unUsedIPs, leader.GetNodeName())

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

	// RollingUpgrade all nodes if needed
	var needUpgrade []RegistryNodeManager
	for _, node := range clusterNodes {
		request, err := node.CheckNodeManifests(&checkRequest)
		if err != nil {
			return err
		}
		if request.NeedSomethingCreateOrUpdate() {
			w.log.Infof("syncCluster :: Node %s need upgrade", node.GetNodeName())
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
