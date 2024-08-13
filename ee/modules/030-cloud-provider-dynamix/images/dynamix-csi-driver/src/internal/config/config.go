/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config

import (
	"dynamix-common/config"
	"flag"
)

type CSIConfig struct {
	DriverName    string
	Endpoint      string
	NodeName      string
	VendorVersion string
	Credentials   config.Credentials
}

func NewCSIConfig(version string) (CSIConfig, error) {
	var err error
	cfg := CSIConfig{
		VendorVersion: version,
	}
	flag.StringVar(&cfg.Endpoint, "endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	flag.StringVar(&cfg.DriverName, "drivername", "dynamix.deckhouse.io", "name of the driver")
	flag.StringVar(&cfg.NodeName, "node-name", "", "node name")

	credentialsConfig, err := config.NewCredentials()
	if err != nil {
		return cfg, err
	}

	cfg.Credentials = *credentialsConfig

	return cfg, nil
}
