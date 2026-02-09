package usecase

import (
	"context"

	"fencing-agent/internal/domain"
)

type NodesGetter interface {
	GetNodes(ctx context.Context) (domain.Nodes, error)
}

type GetNodes struct {
	nodesGetter NodesGetter
}

func NewGetNodes(ng NodesGetter) *GetNodes {
	return &GetNodes{nodesGetter: ng}
}

func (gn *GetNodes) GetNodes(ctx context.Context) (domain.Nodes, error) {
	nodes, err := gn.nodesGetter.GetNodes(ctx)
	return nodes, err
}
