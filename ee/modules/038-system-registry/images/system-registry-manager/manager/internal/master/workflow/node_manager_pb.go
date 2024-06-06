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

	// Info
	GetNodeClusterStatus() (*SeaweedfsNodeClusterStatus, error)
	GetNodeRunningStatus() (*SeaweedfsNodeRunningStatus, error)
	GetNodeIP() (string, error)

	// Cluster actions
	AddNodeToCluster(newNodeIP string) error
	RemoveNodeFromCluster(removeNodeIP string) error

	// Runtime actions
	CreateNodeManifests(request *SeaweedfsCreateNodeRequest) error
	UpdateNodeManifests(request *SeaweedfsUpdateNodeRequest) error
	DeleteNodeManifests() error
}

type SeaweedfsNodeClusterStatus struct {
	IsMaster        bool
	ClusterNodesIPs []string
}

type SeaweedfsNodeRunningStatus struct {
	IsExist             bool
	IsRunning           bool
	NeedUpdateStaticPod bool
	NeedUpdateManifest  bool
	NeedUpdateCerts     bool
}

type SeaweedfsCreateNodeRequest = worker_client.CreateRegistryRequest
type SeaweedfsUpdateNodeRequest = worker_client.UpdateRegistryRequest
