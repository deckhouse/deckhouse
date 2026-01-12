package service

import (
	"context"
	"fencing-agent/internal/core/domain"
	"fencing-agent/internal/core/ports"
)

type StatusProvider struct {
	cluster ports.ClusterProvider
	members ports.MembershipProvider
}

func NewStatusProvider(cluster ports.ClusterProvider, members ports.MembershipProvider) *StatusProvider {
	return &StatusProvider{cluster: cluster, members: members}
}

func (s *StatusProvider) GetAllNodes(ctx context.Context) ([]domain.Node, error) {
	apiNodes, err := s.cluster.GetNodes(ctx)
	if err != nil {
		return s.members.GetMembers(), nil
	}
	return apiNodes, nil
}
