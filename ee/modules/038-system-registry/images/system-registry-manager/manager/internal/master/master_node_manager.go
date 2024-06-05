/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package master

import (
	"context"
	"fmt"
	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"
	k8s_handler "system-registry-manager/internal/master/k8s_handler"
	master_workflow "system-registry-manager/internal/master/workflow"
	pkg_cfg "system-registry-manager/pkg/cfg"
	pkg_logs "system-registry-manager/pkg/logs"
	seaweedfs_client "system-registry-manager/pkg/seaweedfs/client"
	worker_client "system-registry-manager/pkg/worker/client"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

type NodeManager struct {
	ctx        context.Context
	log        *logrus.Entry
	nodeName   string
	nodeInfo   *k8s_handler.MergeInfo
	k8sHandler *k8s_handler.CommonHandler
}

func NewNodeManager(ctx context.Context, nodeName string, k8sHandler *k8s_handler.CommonHandler) *NodeManager {
	log := pkg_logs.GetLoggerFromContext(ctx)

	nodeManager := &NodeManager{
		ctx:        ctx,
		log:        log,
		nodeName:   nodeName,
		k8sHandler: k8sHandler,
	}
	nodeManager.updateData()
	return nodeManager
}

func (m *NodeManager) GetNodeName() string {
	return m.nodeName
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
	var resp *worker_client.CheckRegistryResponse

	f := func(client *worker_client.Client) error {
		var err error
		resp, err = client.RequestCheckRegistry(&worker_client.CheckRegistryRequest{})
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
	createRequest := worker_client.UpdateRegistryRequest{
		Seaweedfs: struct {
			MasterPeers []string "json:\"masterPeers\""
		}{MasterPeers: request.CreateManifestsData.MasterPeers},
	}

	f := func(client *worker_client.Client) error {
		return client.RequestUpdateRegistry(&createRequest)
	}

	return m.makeRequestToWorker(f)
}

func (m *NodeManager) UpdateNodeManifests(request *master_workflow.SeaweedfsUpdateNodeRequest) error {
	// TODO
	createRequest := worker_client.UpdateRegistryRequest{
		Seaweedfs: struct {
			MasterPeers []string "json:\"masterPeers\""
		}{MasterPeers: request.UpdateManifestsData.MasterPeers},
	}

	f := func(client *worker_client.Client) error {
		return client.RequestUpdateRegistry(&createRequest)
	}
	return m.makeRequestToWorker(f)
}

func (m *NodeManager) DeleteNodeManifests() error {
	f := func(client *worker_client.Client) error {
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

func (m *NodeManager) makeRequestToWorker(request func(client *worker_client.Client) error) error {
	// update data and get api
	workerIp, err := m.getWorkerIP()
	if err != nil {
		return err
	}

	client := worker_client.NewClient(m.log, workerIp, pkg_cfg.GetConfig().Manager.WorkerPort)
	return request(client)
}

func (m *NodeManager) updateData() {
	m.nodeInfo = m.k8sHandler.GetAllDataByNodeName(m.nodeName)
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
