package grpc

import (
	"fencing-agent/internal/domain"
	pb "fencing-agent/pkg/api/v1"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func mapNodeGroup(nodeGroup domain.NodeGroup) *pb.NodeGroup {
	pbNodeGroup := &pb.NodeGroup{
		Networks: make(map[string]*pb.NodesInNetwork),
	}

	for networkInterface, nodesInNetwork := range nodeGroup.NodesInNetworks {
		pbNodesInNetwork := &pb.NodesInNetwork{
			Nodes: make(map[string]*pb.Node),
		}

		for nodeName, node := range nodesInNetwork.Members {
			pbNodesInNetwork.Nodes[string(nodeName)] = &pb.Node{
				Name:    node.Name,
				Address: node.Addr,
			}
		}

		pbNodeGroup.Networks[string(networkInterface)] = pbNodesInNetwork
	}

	return pbNodeGroup
}

func mapEvent(event domain.Event) *pb.Event {
	pbEvent := &pb.Event{
		Node: &pb.Node{
			Name:    event.Node.Name,
			Address: event.Node.Addr,
		},
		NetworkInterface: string(event.NetworkInterface),
		Type:             domainEventTypeToPB(event.EventType),
		Time:             timestamppb.New(time.Now()),
	}
	return pbEvent
}
