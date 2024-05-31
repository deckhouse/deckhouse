package seaweedfs

import (
	"context"
	"github.com/seaweedfs/seaweedfs/weed/cluster"
	"github.com/seaweedfs/seaweedfs/weed/pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"
)

func getMastersAddress(commandEnv *commandEnv) ([]pb.ServerAddress, error) {
	var masters []pb.ServerAddress
	masters = append(masters, commandEnv.MasterClient.GetMasters(context.Background())...)
	return masters, nil
}

func getVolumeAddressAndTopologyInfo(commandEnv *commandEnv) ([]pb.ServerAddress, *master_pb.VolumeListResponse, error) {
	var volumeServers []pb.ServerAddress
	var resp *master_pb.VolumeListResponse

	err := commandEnv.MasterClient.WithClient(context.Background(), false, func(client master_pb.SeaweedClient) error {
		var err error
		resp, err = client.VolumeList(context.Background(), &master_pb.VolumeListRequest{})
		return err
	})

	if err != nil {
		return nil, nil, err
	}

	for _, dc := range resp.TopologyInfo.DataCenterInfos {
		for _, r := range dc.RackInfos {
			for _, dn := range r.DataNodeInfos {
				volumeServers = append(volumeServers, pb.NewServerAddressFromDataNode(dn))
			}
		}
	}
	return volumeServers, resp, nil
}

func getFilerAddress(commandEnv *commandEnv) ([]pb.ServerAddress, error) {
	var filers []pb.ServerAddress
	err := commandEnv.MasterClient.WithClient(context.Background(), false, func(client master_pb.SeaweedClient) error {
		resp, err := client.ListClusterNodes(context.Background(), &master_pb.ListClusterNodesRequest{
			ClientType: cluster.FilerType,
			FilerGroup: *commandEnv.option.FilerGroup,
		})

		if err != nil {
			return err
		}

		for _, node := range resp.ClusterNodes {
			filers = append(filers, pb.ServerAddress(node.Address))
		}
		return nil
	})
	return filers, err
}
