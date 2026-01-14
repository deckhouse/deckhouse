package grpc

import (
	"context"
	"fencing-agent/internal/core/domain"
	"fencing-agent/internal/core/ports"
	pb "fencing-agent/pkg/api/v1"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	pb.UnimplementedFencingServer
	eventBus       ports.EventsBus
	statusProvider ports.StatusQuery
}

func NewServer(eventBus ports.EventsBus, statusProvider ports.StatusQuery) *Server {
	return &Server{
		eventBus:       eventBus,
		statusProvider: statusProvider,
	}
}

func (s *Server) GetAll(ctx context.Context, _ *emptypb.Empty) (*pb.AllNodes, error) {
	nodes, err := s.statusProvider.GetAllNodes(ctx)
	if err != nil {
		return nil, err
	}
	sNodes := make([]*pb.Node, 0, len(nodes))
	for _, node := range nodes {
		sNodes = append(sNodes, &pb.Node{
			Name:      node.Name,
			Addresses: node.Addresses,
		})
	}
	return &pb.AllNodes{Nodes: sNodes}, nil
}

func (s *Server) StreamEvents(_ *emptypb.Empty, stream pb.Fencing_StreamEventsServer) error {
	ctx := stream.Context()

	events := s.eventBus.Subscribe(ctx)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return nil
			}

			pbEvent := &pb.Event{
				Node: &pb.Node{
					Name:      event.Node.Name,
					Addresses: event.Node.Addresses,
				},
				Time: timestamppb.Now(), // TODO think about time in domain.Event
				Type: convertEventType(event.EventType),
			}
			if err := stream.Send(pbEvent); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func convertEventType(et domain.EventType) pb.EventType {
	switch et {
	case domain.EventTypeJoin:
		return pb.EventType_JOIN
	case domain.EventTypeLeave:
		return pb.EventType_LEFT
	default:
		return pb.EventType_UNDEFINED
	}
}
