package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/domain"
	pb "fencing-agent/pkg/api/v1"
)

type NodesGetter interface {
	GetNodes(ctx context.Context) (domain.Nodes, error)
}

type Publisher interface {
	Subscribe(ctx context.Context) (<-chan domain.Event, error)
}

type Server struct {
	pb.UnimplementedFencingServer
	publisher   Publisher
	nodesGetter NodesGetter
	logger      *log.Logger
}

func NewServer(logger *log.Logger, publisher Publisher, nodesGetter NodesGetter) *Server {
	return &Server{
		publisher:   publisher,
		nodesGetter: nodesGetter,
		logger:      logger,
	}
}

func (s *Server) GetAll(ctx context.Context, _ *emptypb.Empty) (*pb.Nodes, error) {
	nodes, err := s.nodesGetter.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	pbNodes := make([]*pb.Node, 0, len(nodes.Nodes))
	for _, node := range nodes.Nodes {
		pbNodes = append(pbNodes, &pb.Node{
			Name:    node.Name,
			Address: node.Addr,
		})
	}
	s.logger.Info("grpc call: GetAll")
	return &pb.Nodes{Nodes: pbNodes}, nil
}

func (s *Server) StreamEvents(_ *emptypb.Empty, stream pb.Fencing_StreamEventsServer) error {
	s.logger.Info("grpc call: StreamEvents")
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

			pbEvent := &pb.Event{
				Node: &pb.Node{
					Name:    event.Node.Name,
					Address: event.Node.Addr,
				},
				Type: domainEventTypeToPB(event.EventType),
				Time: timestamppb.Now(),
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
