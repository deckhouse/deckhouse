// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"sigs.k8s.io/yaml"
	"strings"
)

const (
	RegistryDataDeviceEnableTerraformVar = "registryDataDeviceEnable"
	RegistryDataDeviceModuleName         = "system-registry"
)

var (
	providersWithRegistryDataDeviceSupport = map[string]struct{}{
		"aws":         {},
		"gcp":         {},
		"yandex":      {},
		"azure":       {},
		"openstack":   {},
		"huaweicloud": {},
		// "vsphere":     {},
		// "vcd":         {},
		// "zvirt":       {},
		// "dynamix":     {},
	}
	registryModesWithoutRegistryDataDeviceSupport = []string{RegistryModeDirect}
)

type ProviderSecondaryDevicesConfig struct {
	RegistryDataDeviceEnable bool `yaml:"RegistryDataDeviceEnable"`
}

func NewProviderSecondaryDevicesConfigFromData(data []byte) (ProviderSecondaryDevicesConfig, error) {
	var ret ProviderSecondaryDevicesConfig
	if len(data) == 0 {
		return ret, nil
	}

	err := yaml.UnmarshalStrict(data, &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

func (cfg *ProviderSecondaryDevicesConfig) ToYAML() ([]byte, error) {
	return yaml.Marshal(cfg)
}

func (d *ProviderSecondaryDevicesConfig) Validate(cloudProvider string) error {
	if err := d.validateRegistryDataDevice(cloudProvider); err != nil {
		return err
	}
	return nil
}

func (d *ProviderSecondaryDevicesConfig) validateRegistryDataDevice(cloudProvider string) error {
	// Skip if disable
	if !d.RegistryDataDeviceEnable {
		return nil
	}

	// Check cloud provider`s white list
	if _, supported := providersWithRegistryDataDeviceSupport[strings.ToLower(cloudProvider)]; supported {
		return nil
	}

	// Return an error if data device is unsupported
	return fmt.Errorf(
		"The registry data device for the '%s' module is not supported with the cloud provider '%s'. "+
			"Please select a registry mode that does not require the registry data device. Available modes: %+v",
		RegistryDataDeviceModuleName,
		cloudProvider,
		registryModesWithoutRegistryDataDeviceSupport,
	)
}
