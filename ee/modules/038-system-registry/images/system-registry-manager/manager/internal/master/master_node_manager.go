/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package master

import (
	"context"
	"fmt"
	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"
	k8s_info "system-registry-manager/internal/master/k8s_info"
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
	workerInfo k8s_info.WorkerInfo
}

func NewNodeManager(ctx context.Context, workerInfo k8s_info.WorkerInfo) *NodeManager {
	log := pkg_logs.GetLoggerFromContext(ctx)

	nodeManager := &NodeManager{
		ctx:        ctx,
		log:        log,
		workerInfo: workerInfo,
	}
	return nodeManager
}

func (m *NodeManager) GetNodeName() string {
	return m.workerInfo.MasterNode.Name
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

	clusterServers := resp.GetClusterServers()
	if clusterServers == nil {
		return nil, fmt.Errorf("resp.GetClusterServers() == nil")
	}

	isLeader := false
	address := make([]string, 0, len(clusterServers))
	for _, server := range clusterServers {
		ip := seaweedfs_client.GetIpFromAddress(server.GetAddress())
		if len(ip) == 0 {
			continue
		}

		address = append(address, ip)
		if ip == nodeInternalIP {
			isLeader = server.GetIsLeader()
		}
	}

	return &master_workflow.SeaweedfsNodeClusterStatus{
		IsLeader:        isLeader,
		ClusterNodesIPs: address,
	}, nil
}

func (m *NodeManager) GetNodeRunningStatus() (*master_workflow.SeaweedfsNodeRunningStatus, error) {
	var resp *worker_client.CheckRegistryResponse

	f := func(client *worker_client.Client) error {
		var err error
		resp, err = client.RequestCheckRegistry(
			&worker_client.CheckRegistryRequest{
				CheckWithMasterPeers: false,
				MasterPeers:          []string{},
			},
		)
		return err
	}
	err := m.makeRequestToWorker(f)
	if err != nil {
		return nil, err
	}

	isRunning := false
	isExist := false
	seaweedfsPodInfo, err := m.seaweedfsPodInfo()
	if err != nil {
		return nil, err
	}
	if seaweedfsPodInfo != nil {
		isExist = true
		for _, containerStatuses := range seaweedfsPodInfo.Status.ContainerStatuses {
			if containerStatuses.Name == "seaweedfs" {
				isRunning = containerStatuses.Ready
			}
		}
	}

	return &master_workflow.SeaweedfsNodeRunningStatus{
		IsExist:            isExist,
		IsRunning:          isRunning,
		NeedUpdateManifest: resp.Data.RegistryFilesState.ManifestsWaitToUpdate || !resp.Data.RegistryFilesState.ManifestsIsExist,
		NeedUpdateCerts:    resp.Data.RegistryFilesState.CertificatesWaitToUpdate || !resp.Data.RegistryFilesState.CertificateIsExist,
	}, nil
}

func (m *NodeManager) GetNodeIP() (string, error) {
	return m.getNodeInternalIP()
}

// Cluster actions
func (m *NodeManager) AddNodeToCluster(newNodeIP string) error {
	newID := seaweedfs_client.CreateIDFromIP(newNodeIP)
	newMasterGrpcAddress := seaweedfs_client.FromIpToMasterGrpcHost(newNodeIP)
	serverVoter := true

	f := func(client *seaweedfs_client.Client) error {
		_, err := client.ClusterRaftAdd(
			seaweedfs_client.NewClusterRaftAddArgs(
				&newID,
				&newMasterGrpcAddress,
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
				seaweedfs_client.CreateIDFromIP(seaweedfs_client.CreateIDFromIP(removeNodeIP)),
			),
		)
		return err
	}
	return m.makeRequestToSeaweedfs(f)
}

// Runtime actions
func (m *NodeManager) CheckNodeManifests(request *master_workflow.SeaweedfsCheckNodeRequest) (*master_workflow.SeaweedfsCheckNodeResponce, error) {
	var err error
	var resp *master_workflow.SeaweedfsCheckNodeResponce
	f := func(client *worker_client.Client) error {
		resp, err = client.RequestCheckRegistry(request)
		return nil
	}
	m.makeRequestToWorker(f)
	return resp, err
}

func (m *NodeManager) CreateNodeManifests(request *master_workflow.SeaweedfsCreateNodeRequest) error {
	f := func(client *worker_client.Client) error {
		return client.RequestCreateRegistry(request)
	}

	return m.makeRequestToWorker(f)
}

func (m *NodeManager) UpdateNodeManifests(request *master_workflow.SeaweedfsUpdateNodeRequest) error {
	f := func(client *worker_client.Client) error {
		return client.RequestUpdateRegistry(request)
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

	masterHost := seaweedfs_client.FromIpToMasterHttpHost(nodeInternalIP)
	filerHost := seaweedfs_client.FromIpToFilerHttpHost(nodeInternalIP)

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

func (m *NodeManager) getNodeInternalIP() (string, error) {
	for _, address := range m.workerInfo.MasterNode.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address, nil
		}
	}
	return "", fmt.Errorf("address.Type != corev1.NodeInternalIP")
}

func (m *NodeManager) getWorkerIP() (string, error) {
	return m.workerInfo.Worker.IP, nil
}

func (m *NodeManager) seaweedfsPodInfo() (*corev1.Pod, error) {
	return k8s_info.GetSeaweedfsPodByNodeName(m.workerInfo.MasterNode.Name)
}
