/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dvpcsidriver

import (
	"errors"
	"sync"

	"dvp-csi-driver/internal/config"
	"dvp-csi-driver/pkg/dvp-csi-driver/service"

	dvpapi "dvp-common/api"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

type csiDriver struct {
	csi.UnimplementedGroupControllerServer

	config *config.CSIConfig
	mutex  sync.Mutex
	client *dvpapi.DVPCloudAPI
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

	client, err := dvpapi.NewDVPCloudAPI(&cfg.CloudConfig)
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
