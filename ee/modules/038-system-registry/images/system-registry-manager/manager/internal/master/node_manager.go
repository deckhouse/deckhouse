/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package master

import (
	"fmt"
	"system-registry-manager/internal/master/handler"
	master_workflow "system-registry-manager/internal/master/workflow"
	"system-registry-manager/pkg/api"
	pkg_api "system-registry-manager/pkg/api"
	pkg_cfg "system-registry-manager/pkg/cfg"
	seaweedfs_client "system-registry-manager/pkg/seaweedfs/client"

	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type NodeManager struct {
	logger        *logrus.Entry
	nodeName      string
	nodeInfo      *handler.MergeInfo
	commonHandler *handler.CommonHandler
}

func NewNodeManager(logger *logrus.Entry, nodeName string, commonHandler *handler.CommonHandler) *NodeManager {
	nodeManager := &NodeManager{
		logger:        logger,
		nodeName:      nodeName,
		commonHandler: commonHandler,
	}
	nodeManager.updateData()
	return nodeManager
}

// Info
func (m *NodeManager) GetNodeClusterStatus() (*master_workflow.SeaweedfsNodeClusterStatus, error) {
	nodeInternalIP, err := m.getNodeInternalIP()
	if err != nil {
		return nil, err
	}

	var resp *master_pb.RaftListClusterServersResponse
	f := func(client *seaweedfs_client.Client) error {
		var err error
		resp, err = client.ClusterRaftPs()
		return err
	}

	err = m.makeRequestToSeaweedfs(f)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("resp == nil")
	}

	isMaster := false
	address := make([]string, 0, len(resp.ClusterServers))
	for _, server := range resp.ClusterServers {
		address = append(address, server.Address)
		if server.Address == nodeInternalIP {
			isMaster = server.IsLeader
		}
	}

	return &master_workflow.SeaweedfsNodeClusterStatus{
		IsMaster:        isMaster,
		ClusterNodesIPs: address,
	}, nil
}

func (m *NodeManager) GetNodeRunningStatus() (*master_workflow.SeaweedfsNodeRunningStatus, error) {
	var resp *api.CheckRegistryResponse

	f := func(client *api.Client) error {
		var err error
		resp, err = client.RequestCheckRegistry(&pkg_api.CheckRegistryRequest{})
		return err
	}
	err := m.makeRequestToWorker(f)
	if err != nil {
		return nil, err
	}

	isRunning := false
	for _, containerStatuses := range m.nodeInfo.SeaweedfsPod.Status.ContainerStatuses {
		if containerStatuses.Name == "seaweedfs" {
			isRunning = containerStatuses.Ready
		}
	}

	return &master_workflow.SeaweedfsNodeRunningStatus{
		IsExist:            !resp.Data.RegistryFilesState.ManifestsWaitToCreate,
		IsRunning:          isRunning,
		NeedUpdateManifest: resp.Data.RegistryFilesState.ManifestsWaitToUpdate,
		NeedUpdateCerts:    resp.Data.RegistryFilesState.CertificatesWaitToUpdate,
		NeedUpdateCaCerts:  resp.Data.RegistryFilesState.CertificatesWaitToUpdate,
	}, nil
}

func (m *NodeManager) GetNodeIP() (string, error) {
	return m.getNodeInternalIP()
}

// Cluster actions
func (m *NodeManager) AddNodeToCluster(newNodeIP string) error {
	newID := seaweedfs_client.FromIpToId(newNodeIP)
	newMasterAddress := seaweedfs_client.FromIpToMasterHost(newNodeIP)
	serverVoter := false

	f := func(client *seaweedfs_client.Client) error {
		_, err := client.ClusterRaftAdd(
			seaweedfs_client.NewClusterRaftAddArgs(
				&newID,
				&newMasterAddress,
				&serverVoter,
			),
		)
		return err
	}
	return m.makeRequestToSeaweedfs(f)
}

func (m *NodeManager) RemoveNodeFromCluster(removeNodeIP string) error {
	f := func(client *seaweedfs_client.Client) error {
		_, err := client.ClusterRaftRemove(
			seaweedfs_client.NewClusterRaftRemoveArgs(
				seaweedfs_client.FromIpToId(removeNodeIP),
			),
		)
		return err
	}
	return m.makeRequestToSeaweedfs(f)
}

// Runtime actions
func (m *NodeManager) CreateNodeManifests(request *master_workflow.SeaweedfsCreateNodeRequest) error {
	// TODO
	createRequest := pkg_api.UpdateRegistryRequest{
		Seaweedfs: struct {
			MasterPeers []string "json:\"masterPeers\""
		}{MasterPeers: request.CreateManifestsData.MasterPeers},
	}

	f := func(client *pkg_api.Client) error {
		return client.RequestUpdateRegistry(&createRequest)
	}

	return m.makeRequestToWorker(f)
}

func (m *NodeManager) UpdateNodeManifests(request *master_workflow.SeaweedfsUpdateNodeRequest) error {
	// TODO
	createRequest := pkg_api.UpdateRegistryRequest{
		Seaweedfs: struct {
			MasterPeers []string "json:\"masterPeers\""
		}{MasterPeers: request.UpdateManifestsData.MasterPeers},
	}

	f := func(client *pkg_api.Client) error {
		return client.RequestUpdateRegistry(&createRequest)
	}
	return m.makeRequestToWorker(f)
}

func (m *NodeManager) DeleteNodeManifests() error {
	f := func(client *pkg_api.Client) error {
		return client.RequestDeleteRegistry()
	}

	return m.makeRequestToWorker(f)
}

func (m *NodeManager) makeRequestToSeaweedfs(request func(client *seaweedfs_client.Client) error) error {
	// update data and get api
	nodeInternalIP, err := m.getNodeInternalIP()
	if err != nil {
		return err
	}

	masterHost := seaweedfs_client.FromIpToMasterHost(nodeInternalIP)
	filerHost := seaweedfs_client.FromIpToFillerHost(nodeInternalIP)

	client, err := seaweedfs_client.NewClient(&masterHost, &filerHost, nil)
	if err != nil {
		return err
	}

	defer client.ClientClose()
	return request(client)
}

func (m *NodeManager) makeRequestToWorker(request func(client *pkg_api.Client) error) error {
	// update data and get api
	workerIp, err := m.getWorkerIP()
	if err != nil {
		return err
	}

	client := pkg_api.NewClient(m.logger, workerIp, pkg_cfg.GetConfig().Manager.WorkerPort)
	return request(client)
}

func (m *NodeManager) updateData() {
	m.nodeInfo = m.commonHandler.GetAllDataByNodeName(m.nodeName)
}

func (m *NodeManager) getNodeInternalIP() (string, error) {
	m.updateData()

	if m.nodeInfo == nil {
		return "", fmt.Errorf("m.nodeInfo == nil")
	}
	for _, address := range m.nodeInfo.MasterNode.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address, nil
		}
	}
	return "", fmt.Errorf("address.Type != corev1.NodeInternalIP")
}

func (m *NodeManager) getWorkerIP() (string, error) {
	m.updateData()

	if m.nodeInfo == nil {
		return "", fmt.Errorf("m.nodeInfo == nil")
	}
	return m.nodeInfo.Worker.IP, nil
}
