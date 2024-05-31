package seaweedfs

import (
	"time"

	"context"
	"github.com/seaweedfs/seaweedfs/weed/cluster"
	"github.com/seaweedfs/seaweedfs/weed/pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"
	"github.com/seaweedfs/seaweedfs/weed/wdclient"
	"google.golang.org/grpc"
)

var (
	IsSecure                              = false
	DefaultTimeout                        = 30 * time.Second
	DefaultRetryCount                     = 3
	certFileName, keyFileName, caFileName = "", "", ""
	defaultfilerGroup                     = ""
	defaultFiler                          = ""
	defaultMasters                        = "localhost:9333"
	lockerAndClientName                   = "system-registry-manager"
)

func NewRetryOptions(timeout *time.Duration) *retryOptions {
	retryOptions := retryOptions{
		timeout: DefaultTimeout,
	}
	if timeout != nil {
		retryOptions.timeout = *timeout
	}
	return &retryOptions
}

func newShellOptions(mastersHosts *string, filer *string) *shellOptions {

	options := shellOptions{
		Masters:    &defaultMasters,
		FilerGroup: &defaultfilerGroup,
		Directory:  "/",
	}

	if mastersHosts != nil {
		options.Masters = mastersHosts
	}

	if filer != nil {
		options.FilerAddress = pb.ServerAddress(defaultFiler)
	}

	if IsSecure {
		options.GrpcDialOption = DialOptionWithTLS(certFileName, keyFileName, caFileName)
	} else {
		options.GrpcDialOption = DialOptionWithoutTLS()
	}
	return &options
}

func newCommandEnv(options *shellOptions, reretryOptions *retryOptions) *commandEnv {
	ce := &commandEnv{
		env:          make(map[string]string),
		MasterClient: wdclient.NewMasterClient(options.GrpcDialOption, *options.FilerGroup, lockerAndClientName, "", "", "", *pb.ServerAddresses(*options.Masters).ToServiceDiscovery()),
		option:       options,
		retryOption:  reretryOptions,
	}
	ce.locker = NewExclusiveLocker(ce, lockerAndClientName)
	return ce
}

type retryOptions struct {
	timeout time.Duration
}

type shellOptions struct {
	Masters        *string
	GrpcDialOption grpc.DialOption

	FilerGroup   *string
	FilerAddress pb.ServerAddress
	Directory    string
}

type commandEnv struct {
	env          map[string]string
	MasterClient *wdclient.MasterClient
	locker       *ExclusiveLocker
	option       *shellOptions
	retryOption  *retryOptions
}

func (cm *commandEnv) getVolumeAddressAndTopologyInfo() ([]pb.ServerAddress, *master_pb.VolumeListResponse, error) {
	var volumeServers []pb.ServerAddress
	var resp *master_pb.VolumeListResponse

	err := cm.MasterClient.WithClientCustomGetMaster(cm.getMasterAddress, false, func(client master_pb.SeaweedClient) error {
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

func (cm *commandEnv) getFilerAddress() ([]pb.ServerAddress, error) {
	var filers []pb.ServerAddress
	err := cm.MasterClient.WithClientCustomGetMaster(cm.getMasterAddress, false, func(client master_pb.SeaweedClient) error {
		resp, err := client.ListClusterNodes(context.Background(), &master_pb.ListClusterNodesRequest{
			ClientType: cluster.FilerType,
			FilerGroup: *cm.option.FilerGroup,
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

func (cm *commandEnv) getMastersAddress() ([]pb.ServerAddress, error) {
	var masters []pb.ServerAddress
	ctx, cancel := context.WithTimeout(context.Background(), cm.retryOption.timeout)
	defer cancel()
	masters = append(masters, cm.MasterClient.GetMasters(ctx)...)
	return masters, nil
}

func (cm *commandEnv) getMasterAddress() pb.ServerAddress {
	ctx, cancel := context.WithTimeout(context.Background(), cm.retryOption.timeout)
	defer cancel()
	return cm.MasterClient.GetMaster(ctx)
}
