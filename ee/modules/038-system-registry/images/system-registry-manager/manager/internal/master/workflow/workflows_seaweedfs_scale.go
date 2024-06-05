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
	pkg_utils "system-registry-manager/pkg/utils"
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
		return w.delete(w.NodeManagers)
	}

	w.log.Infof("Sorting nodes by status")
	sortedNodes, err := SortByStatus(w.NodeManagers)
	if err != nil {
		return err
	}

	clusterNodes := sortedNodes[:expectedNodeCount]
	deleteNodes := sortedNodes[expectedNodeCount:]
	if err := w.needCluster(clusterNodes); err != nil {
		return err
	}

	w.log.Infof("Deleting nodes: %s", GetNodeNames(deleteNodes))
	return w.delete(deleteNodes)
}

func (w *SeaweedfsScaleWorkflow) needCluster(clusterNodes []NodeManager) error {
	w.log.Infof("Ensuring cluster for nodes: %s", GetNodeNames(clusterNodes))
	currentNodes, other, err := SelectByRunningStatus(clusterNodes, CmpSelectIsRunning)
	if err != nil {
		return err
	}

	if len(currentNodes) == 0 {
		w.log.Info("No current running nodes, creating new cluster")
		return w.create(currentNodes)
	}

	w.log.Infof("Scaling existing cluster")
	return w.scale(currentNodes, other)
}

func (w *SeaweedfsScaleWorkflow) scale(currentNodes []NodeManager, newNodes []NodeManager) error {
	w.log.Infof("Scaling cluster with current nodes: %s and new nodes: %s", GetNodeNames(currentNodes), GetNodeNames(newNodes))
	oldIPs := []string{}
	newIPs := make([]string, 0, len(currentNodes)+len(newNodes))

	for _, node := range currentNodes {
		nodeIp, err := node.GetNodeIP()
		if err != nil {
			return err
		}
		newIPs = append(newIPs, nodeIp)
	}
	for _, node := range newNodes {
		nodeIp, err := node.GetNodeIP()
		if err != nil {
			return err
		}
		newIPs = append(newIPs, nodeIp)
	}

	w.log.Infof("Creating request with new IPs: %v", newIPs)
	createRequest := SeaweedfsCreateNodeRequest{
		CreateManifestsData: struct{ MasterPeers []string }{newIPs},
	}

	updateRequest := SeaweedfsUpdateNodeRequest{
		UpdateCert:          true,
		UpdateCaCerts:       false,
		UpdateManifests:     true,
		UpdateManifestsData: struct{ MasterPeers []string }{newIPs},
	}

	masters, err := GetMasters(currentNodes)
	if err != nil {
		return err
	}
	if len(masters) != 1 {
		return fmt.Errorf("len(*clusters) != 1")
	}

	master := masters[0]

	if masterInfo, err := master.GetNodeClusterStatus(); err != nil {
		return err
	} else {
		oldIPs = append(oldIPs, masterInfo.ClusterNodesIPs...)
	}

	for _, newNode := range newNodes {
		nodeIp, err := newNode.GetNodeIP()
		if err != nil {
			return err
		}
		master.AddNodeToCluster(nodeIp)
		w.log.Infof("Adding node %s to cluster", newNode.GetNodeName())
		if err := master.CreateNodeManifests(&createRequest); err != nil {
			return err
		}
	}

	for _, currentNode := range currentNodes {
		w.log.Infof("Updating manifests for node %s", currentNode.GetNodeName())
		if err := currentNode.UpdateNodeManifests(&updateRequest); err != nil {
			return err
		}
	}

	for _, oldIP := range oldIPs {
		if !pkg_utils.IsStringInSlice(oldIP, &newIPs) {
			w.log.Infof("Removing old node %s from cluster", oldIP)
			if err := master.RemoveNodeFromCluster(oldIP); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) create(clusterNodes []NodeManager) error {
	w.log.Infof("Creating new cluster with nodes: %s", GetNodeNames(clusterNodes))
	createRequest := SeaweedfsCreateNodeRequest{
		CreateManifestsData: struct{ MasterPeers []string }{make([]string, 0, len(clusterNodes))},
	}

	for _, node := range clusterNodes {
		nodeIp, err := node.GetNodeIP()
		if err != nil {
			return err
		}
		createRequest.CreateManifestsData.MasterPeers = append(createRequest.CreateManifestsData.MasterPeers, nodeIp)
	}

	for _, node := range clusterNodes {
		w.log.Infof("Creating manifests for node %s", node.GetNodeName())
		err := node.CreateNodeManifests(&createRequest)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) delete(nodes []NodeManager) error {
	w.log.Infof("Deleting nodes %s", GetNodeNames(nodes))
	for _, node := range nodes {
		w.log.Infof("Deleting manifests for node %s", node.GetNodeName())
		if err := node.DeleteNodeManifests(); err != nil {
			return err
		}
	}
	return nil
}
