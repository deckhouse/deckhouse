/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

type SeaweedfsNodeManager interface {
	// Info
	GetNodeClusterStatus() (*SeaweedfsNodeClusterStatus, error)
	GetNodeRunningStatus() (*SeaweedfsNodeRunningStatus, error)
	GetNodeIP() string

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
	IsExist            bool
	IsRunning          bool
	NeedUpdateManifest bool
	NeedUpdateCerts    bool
	NeedUpdateCaCerts  bool
}

type SeaweedfsCreateNodeRequest struct {
	CreateManifestsData struct {
		MasterPeers []string
	}
}

type SeaweedfsUpdateNodeRequest struct {
	UpdateCert          bool
	UpdateCaCerts       bool
	UpdateManifests     bool
	UpdateManifestsData struct {
		MasterPeers []string
	}
}
