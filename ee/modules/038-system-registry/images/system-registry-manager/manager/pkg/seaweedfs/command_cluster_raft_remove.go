package seaweedfs

import (
	"context"
	"fmt"
	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"
)

type clusterRaftRemoveArgs struct {
	serverId string
}

func NewClusterRaftRemoveArgs(serverId string) *clusterRaftRemoveArgs {
	clusterRaftAddArgs := clusterRaftRemoveArgs{
		serverId: serverId,
	}
	return &clusterRaftAddArgs
}

func clusterRaftRemove(args *clusterRaftRemoveArgs, commandEnv *commandEnv) (*master_pb.RaftRemoveServerResponse, error) {
	var result *master_pb.RaftRemoveServerResponse

	if args.serverId == "" {
		return nil, fmt.Errorf("empty server id")
	}

	err := commandEnv.MasterClient.WithClient(context.Background(), false, func(client master_pb.SeaweedClient) error {
		var err error
		result, err = client.RaftRemoveServer(context.Background(), &master_pb.RaftRemoveServerRequest{
			Id:    args.serverId,
			Force: true,
		})
		if err != nil {
			return fmt.Errorf("raft remove server: %v", err)
		}
		return nil
	})
	return result, err
}
