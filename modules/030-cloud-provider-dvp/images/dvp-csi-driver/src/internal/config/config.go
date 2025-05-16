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

package config

import (
	"dvp-common/config"
	"flag"
	"fmt"
)

type CSIConfig struct {
	DriverName    string
	Endpoint      string
	NodeName      string
	VendorVersion string
	config.CloudConfig
}

func NewCSIConfig(version string) (*CSIConfig, error) {
	var err error
	cfg := &CSIConfig{
		VendorVersion: version,
	}
	flag.StringVar(&cfg.Endpoint, "endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	flag.StringVar(&cfg.DriverName, "drivername", "csi.dvp.deckhouse.io", "name of the driver")
	flag.StringVar(&cfg.NodeName, "node-name", "", "node name")

	cloudConfig, err := config.NewCloudConfig()
	if err != nil {
		return nil, err
	}

	if cloudConfig == nil {
		return nil, fmt.Errorf("cloud config is required")
	}
	cfg.CloudConfig = *cloudConfig

	return cfg, nil
}
