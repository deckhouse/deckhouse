package service

import (
	"context"
	"errors"
	"fencing-agent/internal/core/domain"
	"testing"
	"time"

	"github.com/gojuno/minimock/v3"

	mock "fencing-agent/internal/core/ports/minimock"
)

func TestNewStatusProvider(t *testing.T) {
	mc := minimock.NewController(t)

	clusterMock := mock.NewClusterProviderMock(mc)
	membershipMock := mock.NewMembershipProviderMock(mc)

	sp := NewStatusProvider(clusterMock, membershipMock)

	if sp == nil {
		t.Fatal("Expected NewStatusProvider to return non-nil instance")
	}
	if sp.cluster == nil {
		t.Error("Expected cluster to be set")
	}
	if sp.members == nil {
		t.Error("Expected members to be set")
	}
}

func TestStatusProvider_GetAllNodes_FromClusterAPI(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Create test nodes from cluster API
	node1 := domain.NewNode("node1")
	node1.Addresses["eth0"] = "192.168.1.10"
	node1.LastSeen = time.Now()

	node2 := domain.NewNode("node2")
	node2.Addresses["eth0"] = "192.168.1.11"
	node2.LastSeen = time.Now()

	expectedNodes := []domain.Node{node1, node2}

	clusterMock := mock.NewClusterProviderMock(mc).
		GetNodesMock.Return(expectedNodes, nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	sp := NewStatusProvider(clusterMock, membershipMock)

	nodes, err := sp.GetAllNodes(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}

	if nodes[0].Name != "node1" {
		t.Errorf("Expected first node to be 'node1', got '%s'", nodes[0].Name)
	}

	if nodes[1].Name != "node2" {
		t.Errorf("Expected second node to be 'node2', got '%s'", nodes[1].Name)
	}
}

func TestStatusProvider_GetAllNodes_FromClusterAPI_EmptyList(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	emptyNodes := []domain.Node{}

	clusterMock := mock.NewClusterProviderMock(mc).
		GetNodesMock.Return(emptyNodes, nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	sp := NewStatusProvider(clusterMock, membershipMock)

	nodes, err := sp.GetAllNodes(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(nodes))
	}
}

func TestStatusProvider_GetAllNodes_FallbackToMembership(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Cluster API returns error
	clusterErr := errors.New("cluster API unavailable")

	// Create test nodes from membership
	memberNode1 := domain.NewNode("member-node1")
	memberNode1.Addresses["eth0"] = "192.168.1.20"
	memberNode1.LastSeen = time.Now()

	memberNode2 := domain.NewNode("member-node2")
	memberNode2.Addresses["eth0"] = "192.168.1.21"
	memberNode2.LastSeen = time.Now()

	memberNodes := []domain.Node{memberNode1, memberNode2}

	clusterMock := mock.NewClusterProviderMock(mc).
		GetNodesMock.Return(nil, clusterErr)

	membershipMock := mock.NewMembershipProviderMock(mc).
		GetMembersMock.Return(memberNodes)

	sp := NewStatusProvider(clusterMock, membershipMock)

	nodes, err := sp.GetAllNodes(ctx)

	// Should not return error even when cluster API fails
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes from membership, got %d", len(nodes))
	}

	if nodes[0].Name != "member-node1" {
		t.Errorf("Expected first node to be 'member-node1', got '%s'", nodes[0].Name)
	}

	if nodes[1].Name != "member-node2" {
		t.Errorf("Expected second node to be 'member-node2', got '%s'", nodes[1].Name)
	}
}

func TestStatusProvider_GetAllNodes_FallbackToMembership_EmptyList(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	clusterErr := errors.New("cluster API unavailable")
	emptyMemberNodes := []domain.Node{}

	clusterMock := mock.NewClusterProviderMock(mc).
		GetNodesMock.Return(nil, clusterErr)

	membershipMock := mock.NewMembershipProviderMock(mc).
		GetMembersMock.Return(emptyMemberNodes)

	sp := NewStatusProvider(clusterMock, membershipMock)

	nodes, err := sp.GetAllNodes(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(nodes))
	}
}

func TestStatusProvider_GetAllNodes_PrioritizesClusterAPI(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Both cluster API and membership have nodes, but cluster API should be prioritized
	clusterNode := domain.NewNode("cluster-node")
	clusterNode.Addresses["eth0"] = "192.168.1.30"
	clusterNodes := []domain.Node{clusterNode}

	memberNode := domain.NewNode("member-node")
	memberNode.Addresses["eth0"] = "192.168.1.40"
	memberNodes := []domain.Node{memberNode}

	clusterMock := mock.NewClusterProviderMock(mc).
		GetNodesMock.Return(clusterNodes, nil)

	// GetMembers should not be called when cluster API is available
	membershipMock := mock.NewMembershipProviderMock(mc).
		GetMembersMock.Optional().Return(memberNodes)

	sp := NewStatusProvider(clusterMock, membershipMock)

	nodes, err := sp.GetAllNodes(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(nodes) != 1 {
		t.Errorf("Expected 1 node from cluster API, got %d", len(nodes))
	}

	if nodes[0].Name != "cluster-node" {
		t.Errorf("Expected node from cluster API 'cluster-node', got '%s'", nodes[0].Name)
	}
}

func TestStatusProvider_GetAllNodes_ContextCancellation(t *testing.T) {
	mc := minimock.NewController(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Cluster API should handle context cancellation
	clusterMock := mock.NewClusterProviderMock(mc).
		GetNodesMock.Set(func(ctx context.Context) ([]domain.Node, error) {
			return nil, ctx.Err()
		})

	memberNode := domain.NewNode("fallback-node")
	memberNodes := []domain.Node{memberNode}

	membershipMock := mock.NewMembershipProviderMock(mc).
		GetMembersMock.Return(memberNodes)

	sp := NewStatusProvider(clusterMock, membershipMock)

	nodes, err := sp.GetAllNodes(ctx)

	// Should fallback to membership even with context error
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(nodes) != 1 {
		t.Errorf("Expected 1 node from membership fallback, got %d", len(nodes))
	}
}

func TestStatusProvider_GetAllNodes_MultipleNodes(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Test with multiple nodes to verify slice handling
	nodes := make([]domain.Node, 0, 10)
	for i := 1; i <= 10; i++ {
		node := domain.NewNode("node-" + string(rune('0'+i)))
		node.Addresses["eth0"] = "192.168.1." + string(rune('0'+i))
		nodes = append(nodes, node)
	}

	clusterMock := mock.NewClusterProviderMock(mc).
		GetNodesMock.Return(nodes, nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	sp := NewStatusProvider(clusterMock, membershipMock)

	result, err := sp.GetAllNodes(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) != 10 {
		t.Errorf("Expected 10 nodes, got %d", len(result))
	}
}

func TestStatusProvider_GetAllNodes_NodeAttributes(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Test that node attributes are preserved
	testTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	node := domain.NewNode("test-node")
	node.Addresses["eth0"] = "192.168.1.100"
	node.Addresses["eth1"] = "10.0.0.100"
	node.LastSeen = testTime

	clusterMock := mock.NewClusterProviderMock(mc).
		GetNodesMock.Return([]domain.Node{node}, nil)

	membershipMock := mock.NewMembershipProviderMock(mc)

	sp := NewStatusProvider(clusterMock, membershipMock)

	result, err := sp.GetAllNodes(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(result))
	}

	resultNode := result[0]
	if resultNode.Name != "test-node" {
		t.Errorf("Expected name 'test-node', got '%s'", resultNode.Name)
	}

	if resultNode.Addresses["eth0"] != "192.168.1.100" {
		t.Errorf("Expected eth0 '192.168.1.100', got '%s'", resultNode.Addresses["eth0"])
	}

	if resultNode.Addresses["eth1"] != "10.0.0.100" {
		t.Errorf("Expected eth1 '10.0.0.100', got '%s'", resultNode.Addresses["eth1"])
	}

	if !resultNode.LastSeen.Equal(testTime) {
		t.Errorf("Expected LastSeen %v, got %v", testTime, resultNode.LastSeen)
	}
}

func TestStatusProvider_GetAllNodes_NilNodes(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Test handling of nil nodes slice - GetMembers returns nil
	clusterMock := mock.NewClusterProviderMock(mc).
		GetNodesMock.Return(nil, errors.New("some error"))

	membershipMock := mock.NewMembershipProviderMock(mc).
		GetMembersMock.Return(nil)

	sp := NewStatusProvider(clusterMock, membershipMock)

	result, err := sp.GetAllNodes(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// When GetMembers returns nil, the result should be nil
	// This is the actual behavior of the implementation
	if result != nil {
		t.Errorf("Expected nil result when GetMembers returns nil, got %v", result)
	}
}

func TestStatusProvider_Integration_ClusterToMembershipFallback(t *testing.T) {
	mc := minimock.NewController(t)

	ctx := context.Background()

	// Simulate cluster API becoming unavailable mid-operation
	callCount := 0
	clusterNodes := []domain.Node{
		{Name: "cluster-node1", Addresses: map[string]string{"eth0": "192.168.1.1"}},
		{Name: "cluster-node2", Addresses: map[string]string{"eth0": "192.168.1.2"}},
	}

	clusterMock := mock.NewClusterProviderMock(mc).
		GetNodesMock.Set(func(ctx context.Context) ([]domain.Node, error) {
			callCount++
			if callCount > 1 {
				// Simulate cluster API failure after first call
				return nil, errors.New("cluster API down")
			}
			return clusterNodes, nil
		})

	memberNodes := []domain.Node{
		{Name: "member-node1", Addresses: map[string]string{"eth0": "192.168.1.3"}},
	}

	membershipMock := mock.NewMembershipProviderMock(mc).
		GetMembersMock.Return(memberNodes)

	sp := NewStatusProvider(clusterMock, membershipMock)

	// First call - should return cluster nodes
	nodes1, err := sp.GetAllNodes(ctx)
	if err != nil {
		t.Errorf("First call: expected no error, got %v", err)
	}
	if len(nodes1) != 2 {
		t.Errorf("First call: expected 2 nodes, got %d", len(nodes1))
	}

	// Second call - should fallback to membership
	nodes2, err := sp.GetAllNodes(ctx)
	if err != nil {
		t.Errorf("Second call: expected no error, got %v", err)
	}
	if len(nodes2) != 1 {
		t.Errorf("Second call: expected 1 node from membership, got %d", len(nodes2))
	}
	if nodes2[0].Name != "member-node1" {
		t.Errorf("Second call: expected 'member-node1', got '%s'", nodes2[0].Name)
	}
}
