/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/
package dynamixcsidriver

import (
	"errors"
	"sync"

	dynamixapi "dynamix-common/api"
	"dynamix-csi-driver/internal/config"
	"dynamix-csi-driver/pkg/dynamix-csi-driver/service"
	"github.com/container-storage-interface/spec/lib/go/csi"
)

type csiDriver struct {
	csi.UnimplementedGroupControllerServer

	config *config.CSIConfig
	mutex  sync.Mutex
	client *dynamixapi.DynamixCloudAPI
}

func NewDriver(cfg *config.CSIConfig) (*csiDriver, error) {
	if cfg == nil {
		return nil, errors.New("no configuration provided")
	}

	if cfg.DriverName == "" {
		return nil, errors.New("no driver name provided")
	}

	if cfg.NodeName == "" {
		return nil, errors.New("no node name provided")
	}

	if cfg.Endpoint == "" {
		return nil, errors.New("no driver endpoint provided")
	}

	client, err := dynamixapi.NewDynamixCloudAPI(cfg.Credentials)
	if err != nil {
		return nil, err
	}

	return &csiDriver{
		config: cfg,
		client: client,
	}, nil
}

func (d *csiDriver) Run() error {
	s := NewNonBlockingGRPCServer()
	s.Start(
		d.config.Endpoint,
		service.NewIdentity(
			d.config.DriverName,
			d.config.VendorVersion,
			d.client,
		),
		service.NewController(d.client),
		service.NewNode(d.config.NodeName, d.client),
		d,
	)
	s.Wait()

	return nil
}
