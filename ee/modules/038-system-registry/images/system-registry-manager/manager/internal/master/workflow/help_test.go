/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"errors"
	"reflect"
	"testing"
)

type MockNodeForHelper struct {
	IP                 string
	ClusterStatus      *SeaweedfsNodeClusterStatus
	ClusterStatusError error
}

func (m *MockNodeForHelper) GetNodeName() string {
	return "MockNodeForHelper"
}

func (m *MockNodeForHelper) GetNodeClusterStatus() (*SeaweedfsNodeClusterStatus, error) {
	return m.ClusterStatus, m.ClusterStatusError
}

func (m *MockNodeForHelper) GetNodeRunningStatus() (*SeaweedfsNodeRunningStatus, error) {
	return nil, nil
}

func (m *MockNodeForHelper) GetNodeIP() (string, error) {
	return m.IP, nil
}

func (m *MockNodeForHelper) AddNodeToCluster(newNodeIP string) error {
	return nil
}

func (m *MockNodeForHelper) RemoveNodeFromCluster(removeNodeIP string) error {
	return nil
}

func (m *MockNodeForHelper) CreateNodeManifests(request *SeaweedfsCreateNodeRequest) error {
	return nil
}

func (m *MockNodeForHelper) UpdateNodeManifests(request *SeaweedfsUpdateNodeRequest) error {
	return nil
}

func (m *MockNodeForHelper) DeleteNodeManifests() error {
	return nil
}

func TestGetMasters(t *testing.T) {
	tests := []struct {
		name            string
		nodes           []NodeManager
		expectedMasters []NodeManager
		expectedError   error
	}{
		{
			name: "Successful execution",
			nodes: []NodeManager{
				&MockNodeForHelper{
					IP: "192.168.1.1",
					ClusterStatus: &SeaweedfsNodeClusterStatus{
						IsMaster:        true,
						ClusterNodesIPs: []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
					},
				},
				&MockNodeForHelper{
					IP: "192.168.1.2",
					ClusterStatus: &SeaweedfsNodeClusterStatus{
						IsMaster:        false,
						ClusterNodesIPs: []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
					},
				},
				&MockNodeForHelper{
					IP: "192.168.1.3",
					ClusterStatus: &SeaweedfsNodeClusterStatus{
						IsMaster:        false,
						ClusterNodesIPs: []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
					},
				},
			},
			expectedMasters: []NodeManager{&MockNodeForHelper{IP: "192.168.1.1"}},
			expectedError:   nil,
		},
		{
			name: "Node returns an error",
			nodes: []NodeManager{
				&MockNodeForHelper{
					IP:                 "192.168.1.4",
					ClusterStatusError: errors.New("Cluster status error"),
				},
			},
			expectedMasters: nil,
			expectedError:   errors.New("Cluster status error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masters, err := GetMasters(tt.nodes)
			if !reflect.DeepEqual(err, tt.expectedError) {
				t.Errorf("Expected error: %v, got: %v", tt.expectedError, err)
			}

			mastersIPs := make([]string, 0, len(masters))
			expectedMastersIPs := make([]string, 0, len(tt.expectedMasters))

			for _, master := range masters {
				masterIP, err := master.GetNodeIP()
				if err == nil {
					continue
				}
				mastersIPs = append(mastersIPs, masterIP)
			}
			for _, master := range tt.expectedMasters {
				masterIP, err := master.GetNodeIP()
				if err == nil {
					continue
				}
				expectedMastersIPs = append(expectedMastersIPs, masterIP)
			}
			if !reflect.DeepEqual(mastersIPs, expectedMastersIPs) {
				t.Errorf("Expected masters: %v, got: %v", mastersIPs, expectedMastersIPs)
			}
		})
	}
}
