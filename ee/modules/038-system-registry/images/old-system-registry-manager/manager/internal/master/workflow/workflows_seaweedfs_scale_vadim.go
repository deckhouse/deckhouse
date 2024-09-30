/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

/*
import (
	"fmt"
	"github.com/sirupsen/logrus"
	pkg_logs "system-registry-manager/pkg/logs"
	pkg_utils "system-registry-manager/pkg/utils"
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

	w.log.Infof("Starting scale workflow with expected node count: %d", expectedNodesCount)

	if expectedNodesCount == 0 {
		w.log.Info("Expected node count is 0, deleting all nodes.")
		return DeleteNodes(w.ctx, w.log, w.NodeManagers)
	}
	if expectedNodesCount%2 == 0 {
		expectedNodesCount--
		w.log.Warnf("!!!!Expected node count is even, removing one node. new expected node count: %d", expectedNodesCount)
	}

	existNodes, notExistNodes, err := SelectBy(w.NodeManagers, CmpIsExist)
	w.log.Infof("Exist nodes: %s, Not exist nodes: %s", GetNodeNames(existNodes), GetNodeNames(notExistNodes))
	if err != nil {
		return err
	}
	if len(existNodes) == 0 {
		clusterNodes, _ := SplitNodesByCount(notExistNodes, expectedNodesCount)
		return w.createCluster(clusterNodes)
	}

	if len(existNodes) == expectedNodesCount {
		w.log.Infof("existsNodesCount qual expectedNodesCount, existNodesCount: %d, expectedNodesCount: %d", len(existNodes), expectedNodesCount)
		return w.syncCluster(existNodes)
	}

	if len(existNodes) < expectedNodesCount {
		w.log.Infof("existsNodesCount less than expectedNodesCount, existNodesCount: %d, expectedNodesCount: %d", len(existNodes), expectedNodesCount)
		newClusterNodes, _ := SplitNodesByCount(notExistNodes, expectedNodesCount-len(existNodes))
		return w.scaleUpCluster(existNodes, newClusterNodes)
	}

	if len(existNodes) > expectedNodesCount {
		w.log.Infof("existsNodesCount more than expectedNodesCount, existNodesCount: %d, expectedNodesCount: %d", len(existNodes), expectedNodesCount)
		sortedExistNodes, err := SortBy(existNodes, CmpIsRunning)
		if err != nil {
			return err
		}

		clusterNodes, clusterNodesToDelete := SplitNodesByCount(sortedExistNodes, len(existNodes)-expectedNodesCount)
		w.log.Infof("Cluster nodes: %s, Cluster nodes to delete: %s", GetNodeNames(clusterNodes), GetNodeNames(clusterNodesToDelete))
		return w.scaleDownCluster(clusterNodes, clusterNodesToDelete)
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) syncCluster(clusterNodes []RegistryNodeManager) error {
	w.log.Infof("Sync cluster with nodes: %s", GetNodeNames(clusterNodes))

	// Prepare leader and ips
	w.log.Infof("Get IPs for current cluster")
	leaders, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, clusterNodes, []RegistryNodeManager{})
	if err != nil {
		return err
	}
	if len(leaders) != 1 {
		w.log.Infof("The number of leaders is not equal to 1")
		return fmt.Errorf("len(leaders) != 1")
	}
	leader := leaders[0]

	w.log.Infof("DEBUG!!! leader: %s, newClusterIPs: %s, unUsedIPs: %s", leader.GetNodeName(), newClusterIPs, unUsedIPs)

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

func (w *SeaweedfsScaleWorkflow) createCluster(clusterNodes []RegistryNodeManager) error {
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

func (w *SeaweedfsScaleWorkflow) scaleUpCluster(oldClusterNodes, newClusterNodes []RegistryNodeManager) error {
	w.log.Infof("Scaling up cluster %s with new nodes: %s", GetNodeNames(oldClusterNodes), GetNodeNames(newClusterNodes))

	// Prepare leader and ips
	w.log.Infof("Prepare IPs for new cluster")
	leaders, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, append(oldClusterNodes, newClusterNodes...), []RegistryNodeManager{})
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
		w.log.Infof("Remove unused IP %s from cluster", ip)
		if err := leader.RemoveNodeFromCluster(ip); err != nil {
			return err
		}
	}
	return nil
}

func (w *SeaweedfsScaleWorkflow) scaleDownCluster(clusterNodes, clusterNodesToDelete []RegistryNodeManager) error {
	w.log.Infof("Scaling down cluster %s, with nodes: %s", GetNodeNames(append(clusterNodes, clusterNodesToDelete...)), GetNodeNames(clusterNodesToDelete))
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

func (w *SeaweedfsScaleWorkflow) scaleDownClusterPerNode(clusterNodes []RegistryNodeManager, clusterNodeToDelete RegistryNodeManager) error {
	w.log.Infof("Deleting node: %s", GetNodeNames([]RegistryNodeManager{clusterNodeToDelete}))

	// Change leader
	w.log.Infof("!!!! POTENTIAL PROBLEM. RemoveLeaderStatusForNode\n)")
	if err := RemoveLeaderStatusForNode(w.ctx, w.log, clusterNodes, clusterNodeToDelete); err != nil {
		return err
	}

	// Prepare leader and ips
	w.log.Infof("!!!! POTENTIAL PROBLEM. GetNewAndUnusedClusterIP\n")

	leaders, newClusterIPs, unUsedIPs, err := GetNewAndUnusedClusterIP(w.ctx, w.log, clusterNodes, []RegistryNodeManager{clusterNodeToDelete})
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

	// Remove unused nodes from cluster
	for _, ip := range unUsedIPs {
		w.log.Infof("Remove unused IP %s from cluster", ip)
		if err := leader.RemoveNodeFromCluster(ip); err != nil {
			return err
		}
		if err := WaitLeaderElectionForNodes(w.ctx, w.log, clusterNodes); err != nil {
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
	w.log.Infof("RollingUpgradeNodes: %v, %+v", needUpgrade, updateRequest)
	err = RollingUpgradeNodes(w.ctx, w.log, needUpgrade, &updateRequest)
	if err != nil {
		return err
	}

	if err := DeleteNodes(w.ctx, w.log, []RegistryNodeManager{clusterNodeToDelete}); err != nil {
		return err
	}
	return nil
}
*/
