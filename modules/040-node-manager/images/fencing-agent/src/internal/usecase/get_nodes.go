package usecase

import (
	"context"
	"fencing-agent/internal/domain"
)

type NodesGetter interface {
	GetNodes(ctx context.Context) (domain.NetworkInterface, domain.NodesInNetwork, error)
}

type GetNodes struct {
	nodesGetters []NodesGetter
}

func NewGetNodes(ng []NodesGetter) *GetNodes {
	return &GetNodes{nodesGetters: ng}
}

func GetAll(gn *GetNodes) (domain.NodeGroup, error) {
	var ng domain.NodeGroup
	for _, nodeGetter := range gn.nodesGetters {
		interfaceName, nodesInNetwork, err := nodeGetter.GetNodes(context.TODO())
		if err != nil {
			// TODO log?
			continue
		}
		ng.NodesInNetworks[interfaceName] = nodesInNetwork
	}
	return ng, nil
}
