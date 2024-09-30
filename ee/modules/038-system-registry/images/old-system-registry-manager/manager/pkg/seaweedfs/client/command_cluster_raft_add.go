package client

import (
	"context"
	"fmt"
	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"
)

type clusterRaftAddArgs struct {
	serverId      string
	serverAddress string
	serverVoter   bool
}

func NewClusterRaftAddArgs(serverId, serverAddress *string, serverVoter *bool) *clusterRaftAddArgs {
	clusterRaftAddArgs := clusterRaftAddArgs{
		serverId:      "",
		serverAddress: "",
		serverVoter:   true,
	}

	if serverId != nil {
		clusterRaftAddArgs.serverId = *serverId
	}
	if serverAddress != nil {
		clusterRaftAddArgs.serverAddress = *serverAddress
	}
	if serverVoter != nil {
		clusterRaftAddArgs.serverVoter = *serverVoter
	}
	return &clusterRaftAddArgs
}

func clusterRaftAdd(args *clusterRaftAddArgs, cm *commandEnv) (*master_pb.RaftAddServerResponse, error) {
	var result *master_pb.RaftAddServerResponse

	if args.serverId == "" || args.serverAddress == "" {
		return nil, fmt.Errorf("empty server id or address")
	}

	err := cm.MasterClient.WithClientCustomGetMaster(cm.getMasterAddress, false, func(client master_pb.SeaweedClient) error {
		var err error
		result, err = client.RaftAddServer(context.Background(), &master_pb.RaftAddServerRequest{
			Id:      args.serverId,
			Address: args.serverAddress,
			Voter:   args.serverVoter,
		})
		if err != nil {
			return fmt.Errorf("raft add server: %v", err)
		}
		return nil
	})
	return result, err
}
