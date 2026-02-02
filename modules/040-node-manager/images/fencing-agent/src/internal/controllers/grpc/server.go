package grpc

import (
	"context"
	"fencing-agent/internal/domain"
	pb "fencing-agent/pkg/api/v1"

	"google.golang.org/protobuf/types/known/emptypb"
)

type NodesGetter interface {
	GetNodes(ctx context.Context) (domain.NodeGroup, error)
}

type Publisher interface {
	Subscribe(ctx context.Context) (<-chan domain.Event, error)
}

type Server struct {
	pb.UnimplementedFencingServer
	publisher   Publisher
	nodesGetter NodesGetter
}

func NewServer(publisher Publisher, nodesGetter NodesGetter) *Server {
	return &Server{
		publisher:   publisher,
		nodesGetter: nodesGetter,
	}
}

func (s *Server) GetAll(ctx context.Context, _ *emptypb.Empty) (*pb.NodeGroup, error) {
	// TODO scan method
	nodeGroup, err := s.nodesGetter.GetNodes(ctx)
	if err != nil {
		return nil, err
	}
	return mapNodeGroup(nodeGroup), nil
}

func (s *Server) StreamEvents(_ *emptypb.Empty, stream pb.Fencing_StreamEventsServer) error {
	// TODO scan method
	ctx := stream.Context()

	events, err := s.publisher.Subscribe(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Event channel closed
				return nil
			}

			pbEvent := mapEvent(event)

			if err = stream.Send(pbEvent); err != nil {
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
