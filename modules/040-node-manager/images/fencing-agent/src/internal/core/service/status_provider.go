package service

import (
	"context"
	"fencing-agent/internal/core/domain"
)

type ClusterCheck interface {
	GetNodes(ctx context.Context) ([]domain.Node, error)
}

type MembershipCheck interface {
	GetMembers() []domain.Node
}
type StatusProvider struct {
	cluster ClusterCheck
	members MembershipCheck
}

func NewStatusProvider(cluster ClusterCheck, members MembershipCheck) *StatusProvider {
	return &StatusProvider{cluster: cluster, members: members}
}

func (s *StatusProvider) GetAllNodes(ctx context.Context) ([]domain.Node, error) {
	apiNodes, err := s.cluster.GetNodes(ctx)
	if err != nil {
		return s.members.GetMembers(), nil
	}
	return apiNodes, nil
}
