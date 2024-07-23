/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/
package dynamix

import (
	"errors"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

type csiDriver struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedControllerServer
	csi.UnimplementedNodeServer
	csi.UnimplementedGroupControllerServer

	config Config
	mutex  sync.Mutex
}

type Config struct {
	DriverName    string
	Endpoint      string
	NodeID        string
	VendorVersion string
}

func NewCSIDriver(cfg Config) (*csiDriver, error) {
	if cfg.DriverName == "" {
		return nil, errors.New("no driver name provided")
	}

	if cfg.NodeID == "" {
		return nil, errors.New("no node id provided")
	}

	if cfg.Endpoint == "" {
		return nil, errors.New("no driver endpoint provided")
	}
	return &csiDriver{
		config: cfg,
	}, nil
}

func (d *csiDriver) Run() error {
	s := NewNonBlockingGRPCServer()
	s.Start(d.config.Endpoint, d, d, d, d)
	s.Wait()

	return nil
}
