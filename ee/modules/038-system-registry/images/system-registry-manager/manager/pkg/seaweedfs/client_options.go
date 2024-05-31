package seaweedfs

import (
	"time"

	"github.com/seaweedfs/seaweedfs/weed/pb"
	"github.com/seaweedfs/seaweedfs/weed/wdclient"
	"github.com/seaweedfs/seaweedfs/weed/wdclient/exclusive_locks"
	"google.golang.org/grpc"
)

var (
	IsSecure                              = false
	DefaultTimeout                        = 30 * time.Second
	DefaultRetryCount                     = 3
	certFileName, keyFileName, caFileName = "", "", ""

	defaultfilerGroup = ""
	defaultFiler      = ""
	defaultMasters    = "localhost:9333"
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

func newCommandEnv(options *shellOptions, reretryOptions *retryOptions) *commandEnv {
	ce := &commandEnv{
		env:          make(map[string]string),
		MasterClient: wdclient.NewMasterClient(options.GrpcDialOption, *options.FilerGroup, pb.AdminShellClient, "", "", "", *pb.ServerAddresses(*options.Masters).ToServiceDiscovery()),
		option:       options,
		retryOption:  reretryOptions,
	}
	ce.locker = exclusive_locks.NewExclusiveLocker(ce.MasterClient, "shell")
	return ce
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

type retryOptions struct {
	timeout time.Duration
}

type commandEnv struct {
	env          map[string]string
	MasterClient *wdclient.MasterClient
	option       *shellOptions
	retryOption  *retryOptions
	locker       *exclusive_locks.ExclusiveLocker
}

type shellOptions struct {
	Masters        *string
	GrpcDialOption grpc.DialOption

	FilerGroup   *string
	FilerAddress pb.ServerAddress
	Directory    string
}
