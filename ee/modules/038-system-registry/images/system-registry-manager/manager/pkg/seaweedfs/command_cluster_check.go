package seaweedfs

import (
	"context"
	"github.com/seaweedfs/seaweedfs/weed/cluster"
	"github.com/seaweedfs/seaweedfs/weed/pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/volume_server_pb"
)

type connectionsInfo struct {
	From pb.ServerAddress
	To   pb.ServerAddress
	IsOk bool
}

func newConnectionsInfo(from, to pb.ServerAddress, isOK bool) connectionsInfo {
	return connectionsInfo{
		From: from,
		To:   to,
		IsOk: isOK,
	}
}

type ClusterCheckResult struct {
	TopologyInfo master_pb.VolumeListResponse

	MastersAddress       []pb.ServerAddress
	VolumeServersAddress []pb.ServerAddress
	FilersAddress        []pb.ServerAddress

	Connections struct {
		MasterToMaster       []connectionsInfo
		MasterToVolumeServer []connectionsInfo

		VolumeServerToVolumeServer []connectionsInfo
		VolumeServerToMaster       []connectionsInfo

		FilerToFiler        []connectionsInfo
		FilerToMaster       []connectionsInfo
		FilerToVolumeServer []connectionsInfo
	}
}

func clusterCheck(commandEnv *commandEnv) (*ClusterCheckResult, error) {
	result := &ClusterCheckResult{}
	var err error

	// collect filers
	filers, err := getFilerAddress(commandEnv)
	if err != nil {
		return result, err
	}
	result.FilersAddress = filers

	// collect all masters
	masters, err := getMastersAddress(commandEnv)
	if err != nil {
		return result, err
	}
	result.MastersAddress = masters

	// collect volume servers
	volumeServers, topologyInfo, err := getVolumeAddressAndTopologyInfo(commandEnv)
	if err != nil {
		return result, err
	}
	result.VolumeServersAddress = volumeServers
	result.TopologyInfo = *topologyInfo

	// check between masters
	for _, sourceMaster := range masters {
		for _, targetMaster := range masters {
			if sourceMaster == targetMaster {
				continue
			}
			_ = pb.WithMasterClient(false, sourceMaster, commandEnv.option.GrpcDialOption, false, func(client master_pb.SeaweedClient) error {
				_, err := client.Ping(context.Background(), &master_pb.PingRequest{
					Target:     string(targetMaster),
					TargetType: cluster.MasterType,
				})
				if err == nil {
					result.Connections.MasterToMaster = append(
						result.Connections.MasterToMaster,
						newConnectionsInfo(sourceMaster, targetMaster, true),
					)
				} else {
					result.Connections.MasterToMaster = append(
						result.Connections.MasterToMaster,
						newConnectionsInfo(sourceMaster, targetMaster, false),
					)
				}
				return err
			})
		}
	}

	// check between volume servers
	for _, sourceVolumeServer := range volumeServers {
		for _, targetVolumeServer := range volumeServers {
			if sourceVolumeServer == targetVolumeServer {
				continue
			}
			_ = pb.WithVolumeServerClient(false, sourceVolumeServer, commandEnv.option.GrpcDialOption, func(client volume_server_pb.VolumeServerClient) error {
				_, err := client.Ping(context.Background(), &volume_server_pb.PingRequest{
					Target:     string(targetVolumeServer),
					TargetType: cluster.VolumeServerType,
				})
				if err == nil {
					result.Connections.VolumeServerToVolumeServer = append(
						result.Connections.VolumeServerToVolumeServer,
						newConnectionsInfo(sourceVolumeServer, targetVolumeServer, true),
					)
				} else {
					result.Connections.VolumeServerToVolumeServer = append(
						result.Connections.VolumeServerToVolumeServer,
						newConnectionsInfo(sourceVolumeServer, targetVolumeServer, false),
					)
				}
				return err
			})
		}
	}

	// check between filers, and need to connect to itself
	for _, sourceFiler := range filers {
		for _, targetFiler := range filers {
			_ = pb.WithFilerClient(false, 0, sourceFiler, commandEnv.option.GrpcDialOption, func(client filer_pb.SeaweedFilerClient) error {
				_, err := client.Ping(context.Background(), &filer_pb.PingRequest{
					Target:     string(targetFiler),
					TargetType: cluster.FilerType,
				})
				if err == nil {
					result.Connections.FilerToFiler = append(
						result.Connections.FilerToFiler,
						newConnectionsInfo(sourceFiler, targetFiler, true),
					)
				} else {
					result.Connections.FilerToFiler = append(
						result.Connections.FilerToFiler,
						newConnectionsInfo(sourceFiler, targetFiler, false),
					)
				}
				return err
			})
		}
	}

	// check from master to volume servers
	for _, master := range masters {
		for _, volumeServer := range volumeServers {
			_ = pb.WithMasterClient(false, master, commandEnv.option.GrpcDialOption, false, func(client master_pb.SeaweedClient) error {
				_, err := client.Ping(context.Background(), &master_pb.PingRequest{
					Target:     string(volumeServer),
					TargetType: cluster.VolumeServerType,
				})
				if err == nil {
					result.Connections.MasterToVolumeServer = append(
						result.Connections.MasterToVolumeServer,
						newConnectionsInfo(master, volumeServer, true),
					)
				} else {
					result.Connections.MasterToVolumeServer = append(
						result.Connections.MasterToVolumeServer,
						newConnectionsInfo(master, volumeServer, false),
					)
				}
				return err
			})
		}
	}

	// check from volume servers to masters
	for _, volumeServer := range volumeServers {
		for _, master := range masters {
			_ = pb.WithVolumeServerClient(false, volumeServer, commandEnv.option.GrpcDialOption, func(client volume_server_pb.VolumeServerClient) error {
				_, err := client.Ping(context.Background(), &volume_server_pb.PingRequest{
					Target:     string(master),
					TargetType: cluster.MasterType,
				})
				if err == nil {
					result.Connections.VolumeServerToMaster = append(
						result.Connections.VolumeServerToMaster,
						newConnectionsInfo(volumeServer, master, true),
					)
				} else {
					result.Connections.VolumeServerToMaster = append(
						result.Connections.VolumeServerToMaster,
						newConnectionsInfo(volumeServer, master, false),
					)
				}
				return err
			})
		}
	}

	// check from filers to masters
	for _, filer := range filers {
		for _, master := range masters {
			_ = pb.WithFilerClient(false, 0, filer, commandEnv.option.GrpcDialOption, func(client filer_pb.SeaweedFilerClient) error {
				_, err := client.Ping(context.Background(), &filer_pb.PingRequest{
					Target:     string(master),
					TargetType: cluster.MasterType,
				})
				if err == nil {
					result.Connections.FilerToMaster = append(
						result.Connections.FilerToMaster,
						newConnectionsInfo(filer, master, true),
					)
				} else {
					result.Connections.FilerToMaster = append(
						result.Connections.FilerToMaster,
						newConnectionsInfo(filer, master, false),
					)
				}
				return err
			})
		}
	}

	// check from filers to volume servers
	for _, filer := range filers {
		for _, volumeServer := range volumeServers {
			_ = pb.WithFilerClient(false, 0, filer, commandEnv.option.GrpcDialOption, func(client filer_pb.SeaweedFilerClient) error {
				_, err := client.Ping(context.Background(), &filer_pb.PingRequest{
					Target:     string(volumeServer),
					TargetType: cluster.VolumeServerType,
				})
				if err == nil {
					result.Connections.FilerToVolumeServer = append(
						result.Connections.FilerToVolumeServer,
						newConnectionsInfo(filer, volumeServer, true),
					)
				} else {
					result.Connections.FilerToVolumeServer = append(
						result.Connections.FilerToVolumeServer,
						newConnectionsInfo(filer, volumeServer, false),
					)
				}
				return err
			})
		}
	}
	return result, nil
}
