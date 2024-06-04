/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"fmt"
	pkg_utils "system-registry-manager/pkg/utils"
)

type SeaweedfsScaleWorkflow struct {
	ExpectedNodeCount int
	NodeManagers      []*SeaweedfsNodeManager
}

func NewSeaweedfsScaleWorkflow(nodeManagers []*SeaweedfsNodeManager, expectedNodeCount int) *SeaweedfsScaleWorkflow {
	return &SeaweedfsScaleWorkflow{
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

func (w *SeaweedfsScaleWorkflow) needCluster(clusterNodes []*SeaweedfsNodeManager) error {
	currentNodes, other, err := SelectByRunningStatus(clusterNodes, CmpSelectIsRunning)
	if err != nil {
		return err
	}

	if len(currentNodes) == 0 {
		return w.create(currentNodes)
	}

	return w.scale(currentNodes, other)
}

func (w *SeaweedfsScaleWorkflow) scale(currentNodes []*SeaweedfsNodeManager, newNodes []*SeaweedfsNodeManager) error {
	oldIPs := []string{}
	newIPs := make([]string, 0, len(currentNodes)+len(newNodes))

	for _, node := range currentNodes {
		newIPs = append(newIPs, (*node).GetNodeIP())
	}
	for _, node := range newNodes {
		newIPs = append(newIPs, (*node).GetNodeIP())
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
	if masterInfo, err := (*master).GetNodeClusterStatus(); err != nil {
		return err
	} else {
		oldIPs = append(oldIPs, masterInfo.ClusterNodesIPs...)
	}

	// Add new nodes to cluster and create
	for _, newNode := range newNodes {
		(*master).AddNodeToCluster((*newNode).GetNodeIP())
		if err := (*master).CreateNodeManifests(&createRequest); err != nil {
			return err
		}
	}

	// Update old nodes
	for _, currentNode := range currentNodes {
		if err := (*currentNode).UpdateNodeManifests(&updateRequest); err != nil {
			return err
		}
	}

	for _, oldIP := range oldIPs {
		if !pkg_utils.IsStringInSlice(oldIP, &newIPs) {
			if err := (*master).RemoveNodeFromCluster(oldIP); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) create(clusterNodes []*SeaweedfsNodeManager) error {
	createRequest := SeaweedfsCreateNodeRequest{
		CreateManifestsData: struct{ MasterPeers []string }{make([]string, 0, len(clusterNodes))},
	}

	for _, node := range clusterNodes {
		createRequest.CreateManifestsData.MasterPeers = append(createRequest.CreateManifestsData.MasterPeers, (*node).GetNodeIP())
	}

	for _, node := range clusterNodes {
		err := (*node).CreateNodeManifests(&createRequest)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) delete(nodes []*SeaweedfsNodeManager) error {
	for _, node := range nodes {
		return (*node).DeleteNodeManifests()
	}
	return nil
}
