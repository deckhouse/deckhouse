package seaweedfs

import (
	"context"
	"github.com/seaweedfs/seaweedfs/weed/pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"
	"math/rand"
)

func NewClient(mastersHosts *string, filer *string, retryOptions *retryOptions) (*Client, error) {
	options := newShellOptions(mastersHosts, filer)
	commandEnv := newCommandEnv(options, retryOptions)
	client := &Client{
		commandEnv:            commandEnv,
		keepConnectedToMaster: NewKeepConnectedToMaster(commandEnv),
	}
	return client, client.clientStart()
}

type Client struct {
	commandEnv            *commandEnv
	keepConnectedToMaster *KeepConnectedToMaster
}

func (client *Client) ClusterCheck() (*ClusterCheckResult, error) {
	if err := client.waitUntilConnected(); err != nil {
		return nil, err
	}
	return clusterCheck(client.commandEnv)
}

func (client *Client) ClusterRaftAdd(args *clusterRaftAddArgs) (*master_pb.RaftAddServerResponse, error) {
	if err := client.waitUntilConnected(); err != nil {
		return nil, err
	}
	return clusterRaftAdd(args, client.commandEnv)
}

func (client *Client) ClusterRaftPs() (*master_pb.RaftListClusterServersResponse, error) {
	if err := client.waitUntilConnected(); err != nil {
		return nil, err
	}
	return clusterRaftPs(client.commandEnv)
}

func (client *Client) ClusterRaftRemove(args *clusterRaftRemoveArgs) (*master_pb.RaftRemoveServerResponse, error) {
	if err := client.waitUntilConnected(); err != nil {
		return nil, err
	}
	return clusterRaftRemove(args, client.commandEnv)
}

func (client *Client) ClientClose() {
	client.keepConnectedToMaster.Stop()
}

func (client *Client) clientStart() error {
	client.keepConnectedToMaster.Start()

	if err := client.waitUntilConnected(); err != nil {
		return err
	}

	var err error
	var filers []pb.ServerAddress

	if client.commandEnv.option.FilerAddress == "" {
		filers, err = getFilerAddress(client.commandEnv)
	}

	if err != nil {
		return err
	}

	if len(filers) > 0 {
		client.commandEnv.option.FilerAddress = filers[rand.Intn(len(filers))]
	}
	return nil
}

func (client *Client) waitUntilConnected() error {
	ctx, cancel := context.WithTimeout(context.Background(), client.commandEnv.retryOption.timeout)
	defer cancel()

	done := make(chan struct{}) // Channel for notifying function completion
	go func() {
		client.commandEnv.MasterClient.WaitUntilConnected(context.Background()) // Call the original function
		close(done)                                                             // Notify about function completion
	}()

	select {
	case <-ctx.Done():
		// "Timeout reached"
		return ctx.Err()
	case <-done:
		// "WaitUntilConnected completed"
		return nil
	}
}
