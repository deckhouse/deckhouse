package seaweedfs

import (
	"context"
	"github.com/seaweedfs/seaweedfs/weed/pb"
	"github.com/seaweedfs/seaweedfs/weed/pb/master_pb"
	"math/rand"
)

func NewClient(mastersHosts *string, filer *string, retryOptions *retryOptions) (*Client, error) {
	options := newShellOptions(mastersHosts, filer)
	cm := newCommandEnv(options, retryOptions)
	client := &Client{
		cm:                    cm,
		keepConnectedToMaster: NewKeepConnectedToMaster(cm),
	}
	return client, client.clientStart()
}

type Client struct {
	cm                    *commandEnv
	keepConnectedToMaster *KeepConnectedToMaster
}

func (client *Client) ClusterCheck() (*ClusterCheckResult, error) {
	if err := client.waitUntilConnected(); err != nil {
		return nil, err
	}
	return clusterCheck(client.cm)
}

func (client *Client) ClusterRaftAdd(args *clusterRaftAddArgs) (*master_pb.RaftAddServerResponse, error) {
	if err := client.waitUntilConnected(); err != nil {
		return nil, err
	}
	return clusterRaftAdd(args, client.cm)
}

func (client *Client) ClusterRaftPs() (*master_pb.RaftListClusterServersResponse, error) {
	if err := client.waitUntilConnected(); err != nil {
		return nil, err
	}
	return clusterRaftPs(client.cm)
}

func (client *Client) ClusterRaftRemove(args *clusterRaftRemoveArgs) (*master_pb.RaftRemoveServerResponse, error) {
	if err := client.waitUntilConnected(); err != nil {
		return nil, err
	}
	return clusterRaftRemove(args, client.cm)
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

	if client.cm.option.FilerAddress == "" {
		filers, err = client.cm.getFilerAddress()
	}

	if err != nil {
		return err
	}

	if len(filers) > 0 {
		client.cm.option.FilerAddress = filers[rand.Intn(len(filers))]
	}
	return nil
}

func (client *Client) waitUntilConnected() error {
	ctx, cancel := context.WithTimeout(context.Background(), client.cm.retryOption.timeout)
	defer cancel()

	done := make(chan struct{}) // Channel for notifying function completion
	go func() {
		client.cm.MasterClient.WaitUntilConnected(context.Background()) // Call the original function
		close(done)                                                     // Notify about function completion
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
