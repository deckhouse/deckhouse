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
		return w.create(clusterNodes)
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

	createRequest := SeaweedfsCreateNodeRequest{
		MasterPeers: newIPs,
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
			MasterPeers:    newIPs,
			UpdateOrCreate: true,
		},
	}

	w.log.Infof("Get current cluster leaders count")
	leaders, err := GetLeaders(currentNodes)
	if err != nil {
		return err
	}
	if len(leaders) != 1 {
		w.log.Infof("Have more than one cluster leaders")
		return fmt.Errorf("len(*leaders) != 1")
	}
	w.log.Infof("Have one cluster leader")
	leader := leaders[0]

	w.log.Infof("Get cluster status from node %s", leader.GetNodeName())
	if masterInfo, err := leader.GetNodeClusterStatus(); err != nil {
		return err
	} else {
		oldIPs = append(oldIPs, masterInfo.ClusterNodesIPs...)
	}

	// Add node to cluster, create manifests and wait
	for _, newNode := range newNodes {
		w.log.Infof("Creating manifests for node %s", newNode.GetNodeName())
		if err := newNode.CreateNodeManifests(&createRequest); err != nil {
			return err
		}
	}

	for _, newNode := range newNodes {
		w.log.Infof("Waiting nodes %s", newNode.GetNodeName())
		if !WaitNode(w.log, newNode, cmpFuncIsRunning) {
			return fmt.Errorf("error waitig node %s", newNode.GetNodeName())
		}
	}

	for _, newNode := range newNodes {
		nodeIp, err := newNode.GetNodeIP()
		if err != nil {
			return err
		}
		w.log.Infof("Adding node %s to cluster", newNode.GetNodeName())
		if err := leader.AddNodeToCluster(nodeIp); err != nil {
			return err
		}
	}

	// Update old nodes
	for _, currentNode := range currentNodes {
		w.log.Infof("Updating manifests for node %s", currentNode.GetNodeName())
		if err := currentNode.UpdateNodeManifests(&updateRequest); err != nil {
			return err
		}
	}

	for _, oldIP := range oldIPs {
		if !pkg_utils.IsStringInSlice(oldIP, &newIPs) {
			w.log.Infof("Removing old node %s from cluster", oldIP)
			if err := leader.RemoveNodeFromCluster(oldIP); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) create(clusterNodes []NodeManager) error {
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
