package seaweedfs

import (
	"context"
	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"
)

func clusterRaftPs(cm *commandEnv) (*master_pb.RaftListClusterServersResponse, error) {
	var result *master_pb.RaftListClusterServersResponse

	err := cm.MasterClient.WithClientCustomGetMaster(cm.getMasterAddress, false, func(client master_pb.SeaweedClient) error {
		var err error
		result, err = client.RaftListClusterServers(context.Background(), &master_pb.RaftListClusterServersRequest{})
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, err
}
