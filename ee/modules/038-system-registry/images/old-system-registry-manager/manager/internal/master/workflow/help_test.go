/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package workflow

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type MockNodeForHelper struct {
	NodeName           string
	NodeIP             string
	ClusterStatus      *SeaweedfsNodeClusterStatus
	RunningStatus      *SeaweedfsNodeRunningStatus
	ClusterStatusError error
	RunningStatusError error
}

func CreateMockNode(ip string, clusterStatus *SeaweedfsNodeClusterStatus, runningStatus *SeaweedfsNodeRunningStatus, clusterStatusError error, runningStatusError error) *MockNodeForHelper {
	return &MockNodeForHelper{
		NodeName:           fmt.Sprintf("Node-%s", ip),
		NodeIP:             ip,
		ClusterStatus:      clusterStatus,
		RunningStatus:      runningStatus,
		ClusterStatusError: clusterStatusError,
		RunningStatusError: runningStatusError,
	}
}

func (m *MockNodeForHelper) GetNodeName() string {
	return m.NodeName
}

func (m *MockNodeForHelper) GetNodeClusterStatus() (*SeaweedfsNodeClusterStatus, error) {
	return m.ClusterStatus, m.ClusterStatusError
}

func (m *MockNodeForHelper) GetNodeRunningStatus() (*SeaweedfsNodeRunningStatus, error) {
	return m.RunningStatus, m.RunningStatusError
}

func (m *MockNodeForHelper) GetNodeIP() (string, error) {
	return m.NodeIP, nil
}

func (m *MockNodeForHelper) AddNodeToCluster(newNodeIP string) error {
	return fmt.Errorf("error add node to cluster")
}

func (m *MockNodeForHelper) RemoveNodeFromCluster(removeNodeIP string) error {
	return fmt.Errorf("error remove node manifests")
}

func (m *MockNodeForHelper) CreateNodeManifests(request *SeaweedfsCreateNodeRequest) error {
	return fmt.Errorf("error create node manifests")
}

func (m *MockNodeForHelper) UpdateNodeManifests(request *SeaweedfsUpdateNodeRequest) error {
	return fmt.Errorf("error update node manifests")
}

func (m *MockNodeForHelper) CheckNodeManifests(request *SeaweedfsCheckNodeRequest) (*SeaweedfsCheckNodeResponce, error) {
	return nil, fmt.Errorf("error check node manifests")
}

func (m *MockNodeForHelper) DeleteNodeManifests() error {
	return fmt.Errorf("error delete node manifests")
}

func TestGetClustersMembers(t *testing.T) {
	mockNodes := map[string]*MockNodeForHelper{
		"node1": CreateMockNode(
			"192.168.1.1",
			&SeaweedfsNodeClusterStatus{
				IsLeader:        true,
				ClusterNodesIPs: []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
			},
			nil,
			nil,
			nil,
		),
		"node2": CreateMockNode(
			"192.168.1.2",
			&SeaweedfsNodeClusterStatus{
				IsLeader:        false,
				ClusterNodesIPs: []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
			},
			nil,
			nil,
			nil,
		),
		"node3": CreateMockNode(
			"192.168.1.3",
			&SeaweedfsNodeClusterStatus{
				IsLeader:        false,
				ClusterNodesIPs: []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
			},
			nil,
			nil,
			nil,
		),
		"node4": CreateMockNode(
			"192.168.1.4",
			&SeaweedfsNodeClusterStatus{
				IsLeader:        false,
				ClusterNodesIPs: []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"},
			},
			nil,
			nil,
			nil,
		),
	}

	tests := []struct {
		name               string
		nodes              []RegistryNodeManager
		expClustersMembers []ClusterMembers
		expError           error
	}{
		{
			name: "Successful execution",
			nodes: []RegistryNodeManager{
				mockNodes["node1"],
				mockNodes["node2"],
				mockNodes["node3"],
			},
			expClustersMembers: []ClusterMembers{
				{
					Leader:  mockNodes["node1"],
					Members: []RegistryNodeManager{mockNodes["node1"], mockNodes["node2"], mockNodes["node3"]},
				},
			},
			expError: nil,
		},
		{
			name: "Node returns an error",
			nodes: []RegistryNodeManager{
				mockNodes["node4"],
			},
			expClustersMembers: []ClusterMembers{},
			expError:           nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterMembers, err := GetClustersMembers(tt.nodes)

			assert.Equal(t, tt.expError, err)

			assert.Len(t, clusterMembers, len(tt.expClustersMembers))

			for _, expCluster := range tt.expClustersMembers {
				foundCluster := false
				for _, cluster := range clusterMembers {
					if expCluster.Leader.GetNodeName() == cluster.Leader.GetNodeName() {
						assert.Len(t, cluster.Members, len(expCluster.Members))

						foundCluster = true
						expMembers := make([]string, 0, len(expCluster.Members))
						members := make([]string, 0, len(cluster.Members))

						for _, expMember := range expCluster.Members {
							expMembers = append(expMembers, expMember.GetNodeName())
						}

						for _, member := range cluster.Members {
							members = append(members, member.GetNodeName())
						}

						sort.Strings(expMembers)
						sort.Strings(members)

						assert.ElementsMatch(t, expMembers, members)
					}
				}
				assert.True(t, foundCluster)
			}
		})
	}
}

func TestWaitBy(t *testing.T) {
	mockNodes := map[string]*MockNodeForHelper{
		"node1": CreateMockNode(
			"192.168.1.1",
			&SeaweedfsNodeClusterStatus{
				IsLeader: true,
			},
			nil,
			nil,
			nil,
		),
		"node2": CreateMockNode(
			"192.168.1.2",
			nil,
			&SeaweedfsNodeRunningStatus{
				IsRunning: true,
			},
			nil,
			nil,
		),
	}
	tests := []struct {
		name         string
		nodeManagers []RegistryNodeManager
		cmpFuncs     []interface{}
		expResult    bool
		expError     error
	}{
		{
			name: "Nodes meet condition",
			nodeManagers: []RegistryNodeManager{
				mockNodes["node1"],
			},
			cmpFuncs: []interface{}{
				CmpIsLeader,
			},
			expResult: true,
			expError:  nil,
		},
		{
			name: "Nodes meet condition",
			nodeManagers: []RegistryNodeManager{
				mockNodes["node2"],
			},
			cmpFuncs: []interface{}{
				CmpIsRunning,
			},
			expResult: true,
			expError:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := WaitByAllNodes(context.Background(), logrus.NewEntry(logrus.New()), tt.nodeManagers, tt.cmpFuncs...)

			assert.Equal(t, tt.expError, err)
			assert.Equal(t, tt.expResult, result)
		})
	}
}

func TestSelectBy(t *testing.T) {
	mockNodes := map[string]*MockNodeForHelper{
		"node1": CreateMockNode(
			"192.168.1.1",
			&SeaweedfsNodeClusterStatus{
				IsLeader: true,
			},
			&SeaweedfsNodeRunningStatus{
				IsRunning: false,
			},
			nil,
			nil,
		),
		"node2": CreateMockNode(
			"192.168.1.2",
			&SeaweedfsNodeClusterStatus{
				IsLeader: false,
			},
			&SeaweedfsNodeRunningStatus{
				IsRunning: false,
			},
			nil,
			nil,
		),
		"node3": CreateMockNode(
			"192.168.1.3",
			&SeaweedfsNodeClusterStatus{
				IsLeader: true,
			},
			&SeaweedfsNodeRunningStatus{
				IsRunning: true,
			},
			nil,
			nil,
		),
	}

	tests := []struct {
		name           string
		nodeManagers   []RegistryNodeManager
		cmpFuncs       []interface{}
		expSelected    []RegistryNodeManager
		expNotSelected []RegistryNodeManager
		expError       error
	}{
		{
			name: "Select nodes that meet condition",
			nodeManagers: []RegistryNodeManager{
				mockNodes["node1"],
				mockNodes["node2"],
			},
			cmpFuncs: []interface{}{
				CmpIsLeader,
			},
			expSelected: []RegistryNodeManager{
				mockNodes["node1"],
			},
			expNotSelected: []RegistryNodeManager{
				mockNodes["node2"],
			},
			expError: nil,
		},
		{
			name: "Select nodes that meet condition",
			nodeManagers: []RegistryNodeManager{
				mockNodes["node1"],
				mockNodes["node2"],
				mockNodes["node3"],
			},
			cmpFuncs: []interface{}{
				CmpIsLeader,
				CmpIsRunning,
			},
			expSelected: []RegistryNodeManager{
				mockNodes["node3"],
			},
			expNotSelected: []RegistryNodeManager{
				mockNodes["node1"],
				mockNodes["node2"],
			},
			expError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selected, notSelected, err := SelectBy(tt.nodeManagers, tt.cmpFuncs...)

			assert.Equal(t, tt.expError, err)

			assert.Len(t, selected, len(tt.expSelected))
			assert.Len(t, notSelected, len(tt.expNotSelected))

			assert.ElementsMatch(t, selected, tt.expSelected)
			assert.ElementsMatch(t, notSelected, tt.expNotSelected)
		})
	}
}

func TestSortBy(t *testing.T) {
	mockNodes := map[string]*MockNodeForHelper{
		// Test 1
		"node1": CreateMockNode(
			"192.168.1.2",
			&SeaweedfsNodeClusterStatus{
				IsLeader: false,
			},
			&SeaweedfsNodeRunningStatus{
				IsRunning: false,
			},
			nil,
			nil,
		),
		"node2": CreateMockNode(
			"192.168.1.1",
			&SeaweedfsNodeClusterStatus{
				IsLeader: true,
			},
			&SeaweedfsNodeRunningStatus{
				IsRunning: false,
			},
			nil,
			nil,
		),
		"node3": CreateMockNode(
			"192.168.1.3",
			&SeaweedfsNodeClusterStatus{
				IsLeader: false,
			},
			&SeaweedfsNodeRunningStatus{
				IsRunning: false,
			},
			nil,
			nil,
		),

		// Test 2
		"node4": CreateMockNode(
			"192.168.1.2",
			&SeaweedfsNodeClusterStatus{
				IsLeader: true,
			},
			&SeaweedfsNodeRunningStatus{
				IsRunning: true,
			},
			nil,
			nil,
		),
		"node5": CreateMockNode(
			"192.168.1.1",
			&SeaweedfsNodeClusterStatus{
				IsLeader: true,
			},
			&SeaweedfsNodeRunningStatus{
				IsRunning: false,
			},
			nil,
			nil,
		),

		// Test 3
		"node6": CreateMockNode(
			"192.168.1.2",
			nil,
			nil,
			fmt.Errorf("Cluster status error"),
			nil,
		),
	}

	tests := []struct {
		name         string
		nodeManagers []RegistryNodeManager
		cmpFuncs     []interface{}
		expSorted    []RegistryNodeManager
		expError     error
	}{
		{
			name: "Sort nodes by leader status",
			nodeManagers: []RegistryNodeManager{
				mockNodes["node1"],
				mockNodes["node2"],
				mockNodes["node3"],
			},
			cmpFuncs: []interface{}{
				CmpIsLeader,
			},
			expSorted: []RegistryNodeManager{
				mockNodes["node2"],
				mockNodes["node1"],
				mockNodes["node3"],
			},
			expError: nil,
		},
		{
			name: "Sort nodes by running status",
			nodeManagers: []RegistryNodeManager{
				mockNodes["node4"],
				mockNodes["node5"],
			},
			cmpFuncs: []interface{}{
				CmpIsRunning,
			},
			expSorted: []RegistryNodeManager{
				mockNodes["node4"],
				mockNodes["node5"],
			},
			expError: nil,
		},
		{
			name: "Error in getting node status",
			nodeManagers: []RegistryNodeManager{
				mockNodes["node6"],
			},
			cmpFuncs: []interface{}{
				CmpIsLeader,
			},
			expSorted: nil,
			expError:  fmt.Errorf("Cluster status error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortedNodes, err := SortBy(tt.nodeManagers, tt.cmpFuncs...)

			assert.Equal(t, tt.expError, err)
			assert.Len(t, sortedNodes, len(tt.expSorted))
			assert.ElementsMatch(t, sortedNodes, tt.expSorted)
		})
	}
}

// func TestGetExpectedNodeCount(t *testing.T) {
// 	tests := []struct {
// 		expectedNodeCount int
// 		expResult         int
// 	}{
// 		{0, 0},
// 		{1, 1},
// 		{2, 1},
// 		{3, 3},
// 		{4, 3},
// 		{-1, 0},
// 	}

// 	for _, tt := range tests {
// 		t.Run(fmt.Sprintf("expectedNodeCount=%d", tt.expectedNodeCount), func(t *testing.T) {
// 			result := GetExpectedNodeCount(tt.expectedNodeCount)
// 			assert.Equal(t, result, tt.expResult)
// 		})
// 	}
// }

func TestGetNodeNames(t *testing.T) {
	mockNodes := map[string]*MockNodeForHelper{
		"node1": CreateMockNode("192.168.1.1", nil, nil, nil, nil),
		"node2": CreateMockNode("192.168.1.2", nil, nil, nil, nil),
		"node3": CreateMockNode("192.168.1.3", nil, nil, nil, nil),
		"node4": CreateMockNode("192.168.1.4", nil, nil, nil, nil),
		"node5": CreateMockNode("192.168.1.5", nil, nil, nil, nil),
		"node6": CreateMockNode("192.168.1.6", nil, nil, nil, nil),
	}
	tests := []struct {
		nodes     []RegistryNodeManager
		expResult string
	}{
		{
			nodes: []RegistryNodeManager{
				mockNodes["node1"],
				mockNodes["node3"],
				mockNodes["node5"],
				mockNodes["node6"],
				mockNodes["node4"],
				mockNodes["node2"],
			},
			expResult: "[Node-192.168.1.1,Node-192.168.1.3,Node-192.168.1.5,Node-192.168.1.6,Node-192.168.1.4,Node-192.168.1.2]",
		},
		{
			nodes:     []RegistryNodeManager{},
			expResult: "[]",
		},
	}

	for _, tt := range tests {
		t.Run("GetNodeNames", func(t *testing.T) {
			result := GetNodeNames(tt.nodes)
			assert.Equal(t, result, tt.expResult)
		})
	}
}
