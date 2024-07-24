/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/
package dynamixcsidriver

import (
	"dynamix-csi-driver/pkg/dynamix-csi-driver/service"
	"dynamixcommon/config"
	"errors"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

type csiDriver struct {
	csi.UnimplementedControllerServer
	csi.UnimplementedNodeServer
	csi.UnimplementedGroupControllerServer

	config config.CSIConfig
	mutex  sync.Mutex
}

func NewDriver(cfg config.CSIConfig) (*csiDriver, error) {
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
	s.Start(
		d.config.Endpoint,
		service.NewIdentity(
			d.config.DriverName,
			d.config.VendorVersion,
		),
		d,
		d,
		d,
	)
	s.Wait()

	return nil
}
