/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	worker_client "system-registry-manager/pkg/worker/client"
)

type NodeManager interface {
	GetNodeName() string
	GetNodeIP() (string, error)

	// Info
	GetNodeClusterStatus() (*SeaweedfsNodeClusterStatus, error)
	GetNodeRunningStatus() (*SeaweedfsNodeRunningStatus, error)

	// Seaweedfs actions
	AddNodeToCluster(newNodeIP string) error
	RemoveNodeFromCluster(removeNodeIP string) error

	// Manager actions
	CreateNodeManifests(request *SeaweedfsCreateNodeRequest) error
	UpdateNodeManifests(request *SeaweedfsUpdateNodeRequest) error
	CheckNodeManifests(request *SeaweedfsCheckNodeRequest) (*SeaweedfsCheckNodeResponce, error)
	DeleteNodeManifests() error
}

type SeaweedfsNodeClusterStatus struct {
	IsLeader        bool
	ClusterNodesIPs []string
}

type SeaweedfsNodeRunningStatus struct {
	IsExist            bool
	IsRunning          bool
	NeedUpdateManifest bool
	NeedUpdateCerts    bool
}

type SeaweedfsCreateNodeRequest = worker_client.CreateRegistryRequest
type SeaweedfsUpdateNodeRequest = worker_client.UpdateRegistryRequest
type SeaweedfsCheckNodeRequest = worker_client.CheckRegistryRequest
type SeaweedfsCheckNodeResponce = worker_client.CheckRegistryResponse
