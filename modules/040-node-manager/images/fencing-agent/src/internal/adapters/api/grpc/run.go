package grpc

import (
	"fencing-controller/internal/core/ports"
	pb "fencing-controller/pkg/api/v1"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func Run(logger *zap.Logger, socketPath string, bus ports.EventsBus) error {
	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return err // TODO logging
	}

	grpcServer := grpc.NewServer()

	fencingServer := NewServer(bus)
	pb.RegisterFencingServer(grpcServer, fencingServer)

	// TODO logging
	if err = grpcServer.Serve(lis); err != nil {
		return err // TODO logging
	}
	return nil
}
