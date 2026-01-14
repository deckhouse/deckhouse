package grpc

import (
	pb "fencing-agent/pkg/api/v1"
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func Run(socketPath string, grpcSrv *Server) error {
	if err := os.RemoveAll(socketPath); err != nil {
		return fmt.Errorf("failed to remove socket: %w", err)
	}
	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()

	reflection.Register(grpcServer)

	pb.RegisterFencingServer(grpcServer, grpcSrv)

	if err = grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}
