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

	if expectedNodeCount > len(w.NodeManagers) {
		return fmt.Errorf("expectedNodeCount > len(w.NodeManagers)")
	}

	if expectedNodeCount == 0 {
		return w.delete(w.NodeManagers)
	}

	sortedNodes, err := SortByStatus(w.NodeManagers)
	if err != nil {
		return err
	}

	clusterNodes := sortedNodes[:expectedNodeCount]
	deleteNodes := sortedNodes[expectedNodeCount:]
	if err := w.needCluster(clusterNodes); err != nil {
		return err
	}

	return w.delete(deleteNodes)
}

func (w *SeaweedfsScaleWorkflow) needCluster(clusterNodes []NodeManager) error {
	currentNodes, other, err := SelectByRunningStatus(clusterNodes, CmpSelectIsRunning)
	if err != nil {
		return err
	}

	if len(currentNodes) == 0 {
		return w.create(currentNodes)
	}

	return w.scale(currentNodes, other)
}

func (w *SeaweedfsScaleWorkflow) scale(currentNodes []NodeManager, newNodes []NodeManager) error {
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
		CreateManifestsData: struct{ MasterPeers []string }{newIPs},
	}

	updateRequest := SeaweedfsUpdateNodeRequest{
		UpdateCert:          true,
		UpdateCaCerts:       false,
		UpdateManifests:     true,
		UpdateManifestsData: struct{ MasterPeers []string }{newIPs},
	}

	// Check is one cluster
	masters, err := GetMasters(currentNodes)
	if err != nil {
		return err
	}
	if len(masters) != 1 {
		return fmt.Errorf("len(*clusters) != 1")
	}

	master := masters[0]

	// Get old cluster IPs
	if masterInfo, err := master.GetNodeClusterStatus(); err != nil {
		return err
	} else {
		oldIPs = append(oldIPs, masterInfo.ClusterNodesIPs...)
	}

	// Add new nodes to cluster and create
	for _, newNode := range newNodes {
		nodeIp, err := newNode.GetNodeIP()
		if err != nil {
			return err
		}
		master.AddNodeToCluster(nodeIp)
		if err := master.CreateNodeManifests(&createRequest); err != nil {
			return err
		}
	}

	// Update old nodes
	for _, currentNode := range currentNodes {
		if err := currentNode.UpdateNodeManifests(&updateRequest); err != nil {
			return err
		}
	}

	for _, oldIP := range oldIPs {
		if !pkg_utils.IsStringInSlice(oldIP, &newIPs) {
			if err := master.RemoveNodeFromCluster(oldIP); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) create(clusterNodes []NodeManager) error {
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
		err := node.CreateNodeManifests(&createRequest)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) delete(nodes []NodeManager) error {
	for _, node := range nodes {
		return node.DeleteNodeManifests()
	}
	return nil
}
