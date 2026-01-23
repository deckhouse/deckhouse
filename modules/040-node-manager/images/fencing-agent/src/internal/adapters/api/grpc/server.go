package grpc

import (
	"context"
	"fencing-agent/internal/core/domain"
	pb "fencing-agent/pkg/api/v1"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type StatusQuery interface {
	GetAllNodes(ctx context.Context) ([]domain.Node, error)
}

type EventsBus interface {
	Subscribe(ctx context.Context) <-chan domain.Event
}

type Server struct {
	pb.UnimplementedFencingServer
	eventBus       EventsBus
	statusProvider StatusQuery
}

func NewServer(eventBus EventsBus, statusProvider StatusQuery) *Server {
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

	pbNodes := make([]*pb.Node, 0, len(nodes))
	for _, node := range nodes {
		pbNodes = append(pbNodes, &pb.Node{
			Name:      node.Name,
			Addresses: node.Addresses,
		})
	}

	return &pb.AllNodes{Nodes: pbNodes}, nil
}

func (s *Server) StreamEvents(_ *emptypb.Empty, stream pb.Fencing_StreamEventsServer) error {
	ctx := stream.Context()

	events := s.eventBus.Subscribe(ctx)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Event channel closed
				return nil
			}

			pbEvent := &pb.Event{
				Node: &pb.Node{
					Name:      event.Node.Name,
					Addresses: event.Node.Addresses,
				},
				Time: timestamppb.Now(),
				Type: domainEventTypeToPB(event.EventType),
			}

			if err := stream.Send(pbEvent); err != nil {
				return err
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func domainEventTypeToPB(et domain.EventType) pb.EventType {
	switch et {
	case domain.EventTypeJoin:
		return pb.EventType_JOIN
	case domain.EventTypeLeave:
		return pb.EventType_LEFT
	default:
		return pb.EventType_UNDEFINED
	}
}
